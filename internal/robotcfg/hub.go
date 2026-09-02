package robotcfg

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/andreibanu/pusher/internal/adb"
)

// HubDir is where the robot controller keeps its configurations.
const HubDir = "/sdcard/FIRST"

// Ext is the extension the robot controller looks for.
const Ext = ".xml"

const illegalNameChars = `?:"*|/\<>`

// CheckName reports whether a name can be used as a configuration name.
func CheckName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("a configuration name cannot be empty")
	}
	if name != strings.TrimSpace(name) {
		return fmt.Errorf("%q has leading or trailing whitespace", name)
	}
	if i := strings.IndexAny(name, illegalNameChars); i >= 0 {
		return fmt.Errorf("%q contains %q, which the robot controller does not allow in a "+
			"configuration name (none of %s)", name, name[i], illegalNameChars)
	}
	return nil
}

// RemotePath is where a named configuration lives on the robot.
func RemotePath(name string) string {
	return HubDir + "/" + name + Ext
}

// List returns the configuration names on the robot, sorted.
func List(serial string) ([]string, error) {
	out, err := adb.Shell(serial, "ls", "-1", HubDir, "2>/dev/null")
	if err != nil {
		return nil, fmt.Errorf("cannot list %s on the robot: %w", HubDir, err)
	}

	return parseListing(out), nil
}

func parseListing(out string) []string {
	var names []string

	for _, line := range strings.Split(out, "\n") {

		file := strings.TrimSpace(strings.TrimRight(line, "\r"))

		if !strings.HasSuffix(file, Ext) {
			continue
		}
		names = append(names, strings.TrimSuffix(file, Ext))
	}

	sort.Strings(names)
	return names
}

// Hashes returns an MD5 for every configuration on the robot, keyed by name.
func Hashes(serial string) map[string]string {
	out, err := adb.Shell(serial, "md5sum", HubDir+"/*"+Ext, "2>/dev/null")
	if err != nil {
		return nil
	}

	return parseHashes(out)
}

func parseHashes(out string) map[string]string {
	hashes := map[string]string{}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))

		digest, path, found := strings.Cut(line, "  ")
		if !found || len(digest) != 32 {
			continue
		}

		path = strings.TrimSpace(path)
		if !strings.HasPrefix(path, HubDir+"/") || !strings.HasSuffix(path, Ext) {
			continue
		}

		name := strings.TrimSuffix(strings.TrimPrefix(path, HubDir+"/"), Ext)
		hashes[name] = strings.ToLower(digest)
	}

	return hashes
}

// Hash is the digest form used to compare against Hashes.
func Hash(data []byte) string {
	return fmt.Sprintf("%x", md5.Sum(data))
}

// Fetch reads one configuration off the robot.
func Fetch(serial, name string) ([]byte, error) {
	local, err := os.CreateTemp("", "pusher-config-*.xml")
	if err != nil {
		return nil, err
	}
	local.Close()
	defer os.Remove(local.Name())

	if err := adb.Pull(serial, RemotePath(name), local.Name()); err != nil {
		return nil, fmt.Errorf("cannot read %q off the robot: %w", name, err)
	}

	data, err := os.ReadFile(local.Name())
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("%q came back empty", name)
	}

	return data, nil
}

// Send writes one configuration to the robot.
func Send(serial, name string, data []byte) error {
	if err := CheckName(name); err != nil {
		return err
	}

	local, err := os.CreateTemp("", "pusher-config-*.xml")
	if err != nil {
		return err
	}
	defer os.Remove(local.Name())

	if _, err := local.Write(data); err != nil {
		local.Close()
		return err
	}
	if err := local.Close(); err != nil {
		return err
	}

	if err := adb.Push(serial, local.Name(), RemotePath(name)); err != nil {
		return fmt.Errorf("cannot write %q to the robot: %w", name, err)
	}

	return verify(serial, name, data)
}

// verify reads the configuration back and checks the robot has what it was
// sent.
//
// A push that reports success and leaves nothing usable behind is the failure
// worth catching here: adb exits zero having written a file the robot cannot
// read, or writes it somewhere the robot controller does not look, and the only
// symptom is a configuration that never appears in the list. Reading it back
// turns that into an error at the moment it happens.
func verify(serial, name string, sent []byte) error {
	got, readErr := Fetch(serial, name)
	names, listErr := List(serial)

	return checkPushed(name, sent, got, readErr, names, listErr)
}

// checkPushed decides whether the robot ended up with what it was sent.
//
// Separated from the fetching so the decision can be tested without a robot,
// which is the half that has been wrong.
func checkPushed(name string, sent, got []byte, readErr error, names []string, listErr error) error {
	if readErr != nil {
		return fmt.Errorf("%q was pushed but cannot be read back: %w\n"+
			"    The robot controller will not see it either", name, readErr)
	}

	// Trailing whitespace is not a difference worth failing on: the robot
	// controller rewrites these itself and is not fussy about the last byte.
	if !bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(sent)) {
		return fmt.Errorf("%q arrived on the robot with different contents (%d bytes sent, %d read back)",
			name, len(sent), len(got))
	}

	// A robot that will not answer a listing is not evidence of a failed push,
	// and the file has already been read back byte for byte.
	if listErr != nil {
		return nil
	}

	for _, n := range names {
		if n == name {
			return nil
		}
	}

	return fmt.Errorf("%q was written to %s but is not in the robot's list of configurations.\n"+
		"    The robot controller reads that directory directly, so this means the\n"+
		"    file is not where it looks", name, RemotePath(name))
}

// ControllerPackage finds the robot controller app on the device.
func ControllerPackage(serial string) string {
	out, err := adb.Shell(serial, "pm", "list", "packages", "2>/dev/null")
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimPrefix(strings.TrimSpace(strings.TrimRight(line, "\r")), "package:")
		if strings.Contains(name, "ftcrobotcontroller") {
			return name
		}
	}
	return ""
}

// Restart restarts the robot controller app.
//
// The robot controller lists the directory afresh whenever it is asked, so a
// pushed configuration is normally there the next time the Driver Station opens
// the list. What it does not do is notice one appearing underneath it, so a
// Driver Station already showing the list goes on showing the old one. This is
// the blunt instrument that settles it.
func Restart(serial, pkg string) error {
	if pkg == "" {
		return fmt.Errorf("no robot controller package to restart")
	}

	if _, err := adb.Shell(serial, "am", "force-stop", pkg); err != nil {
		return fmt.Errorf("cannot stop %s: %w", pkg, err)
	}

	if _, err := adb.Shell(serial, "monkey", "-p", pkg, "-c",
		"android.intent.category.LAUNCHER", "1", ">/dev/null", "2>&1"); err != nil {
		return fmt.Errorf("cannot start %s again: %w", pkg, err)
	}

	return nil
}

// Remove deletes a configuration from the robot.
func Remove(serial, name string) error {
	if _, err := adb.Shell(serial, "rm", "-f", shellQuote(RemotePath(name))); err != nil {
		return fmt.Errorf("cannot delete %q from the robot: %w", name, err)
	}
	return nil
}

// Exists reports whether the robot has a configuration by that name.
func Exists(serial, name string) bool {
	names, err := List(serial)
	if err != nil {
		return false
	}
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

func shellQuote(path string) string {
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

var rcPackages = []string{
	"com.qualcomm.ftcrobotcontroller",
	"com.revrobotics.ftcrobotcontroller",
}

const activeConfigPref = "pref_hardware_config_filename"

// ActiveConfig returns the configuration the robot has selected, empty if it cannot tell.
func ActiveConfig(serial string) string {
	for _, pkg := range rcPackages {
		path := fmt.Sprintf("/data/data/%s/shared_prefs/%s_preferences.xml", pkg, pkg)

		out, err := adb.Shell(serial, "cat", path, "2>/dev/null")
		if err != nil || strings.TrimSpace(out) == "" {
			continue
		}

		if name := activeFromPrefs(out); name != "" {
			return name
		}
	}

	return ""
}

func activeFromPrefs(prefs string) string {
	marker := `name="` + activeConfigPref + `"`

	start := strings.Index(prefs, marker)
	if start < 0 {
		return ""
	}

	rest := prefs[start:]
	open := strings.Index(rest, ">")
	closing := strings.Index(rest, "</string>")
	if open < 0 || closing < 0 || closing < open {
		return ""
	}

	value := unescapeXML(rest[open+1 : closing])

	var stored struct {
		Name string `json:"name"`
	}
	if json.Unmarshal([]byte(value), &stored) != nil {
		return ""
	}

	return stored.Name
}

func unescapeXML(s string) string {
	return strings.NewReplacer(
		"&quot;", `"`,
		"&apos;", "'",
		"&lt;", "<",
		"&gt;", ">",
		"&amp;", "&",
	).Replace(s)
}

// LocalDir is where configurations are kept in an FTC project.
func LocalDir(projectRoot string) string {
	return filepath.Join(projectRoot, "configs")
}
