package hubwifi

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/andreibanu/pusher/internal/adb"
)

// Setup puts the hub onto a network and reports where it landed.
func Setup(out io.Writer, serial string, opt Options) (Result, error) {
	if opt.SSID == "" {
		return Result{}, fmt.Errorf("no network to join: set one in `pusher settings` -> ADB relay")
	}

	prefix, err := rootPrefix(serial)
	if err != nil {
		return Result{}, err
	}
	r := runner{serial: serial, root: prefix, out: out}

	r.quiet("mkdir", "-p", Dir)

	if err := r.loadDriver(opt.Module); err != nil {
		return Result{}, err
	}
	if err := r.associate(opt); err != nil {
		return Result{}, err
	}

	result, err := r.address()
	if err != nil {
		return Result{}, err
	}

	if err := r.route(result); err != nil {
		return result, err
	}

	if opt.Persist {
		if err := r.persist(opt); err != nil {
			// The robot is on the network either way, so this is a warning
			// rather than a failure: losing it at the next reboot is not the
			// same as not having it now.
			r.say("[!] On the network, but it will not come back after a reboot: %v", err)
		}
	}

	return result, nil
}

// loadDriver gets the adapter to the point of being an interface.
func (r runner) loadDriver(module string) error {
	if r.hasIface() {
		r.say("[OK] %s is already there", Iface)
		return nil
	}

	// Only now is a driver needed, and only now is one looked for. Resolving it
	// up front meant a hub whose interface was already up could still fail on
	// "no driver for Control Hub OS unknown", which is a complaint about a
	// question nobody had to ask.
	if module == "" {
		var err error
		if module, err = r.carriedDriver(); err != nil {
			return err
		}
	}

	r.say("[*] Loading the driver")

	if err := adb.Push(r.serial, module, ModulePath); err != nil {
		return fmt.Errorf("cannot put the driver on the robot: %w", err)
	}

	// insmod says why it refused, and it does not always use the word error.
	// Reporting its own words beats guessing at them: "File exists" and
	// "Exec format error" are different problems with different answers, and
	// both used to come out as "loaded but claimed nothing".
	out, err := r.run("insmod", ModulePath)
	text := strings.TrimSpace(out)

	// Already loaded is not a refusal. It is what a second run looks like, and
	// what every run after a manual one looks like. The driver is in the kernel
	// either way, so the question is whether it has claimed anything, which the
	// wait below answers. Sending somebody to reboot the hub over this was
	// advice to undo the thing that had already worked.
	if alreadyLoaded(text) {
		r.say("[*] Driver was already loaded")
	} else if err != nil || refused(text) {
		// Its own words, and when it had none, at least say what happened
		// rather than reporting an empty string as a reason.
		reason := text
		if reason == "" && err != nil {
			reason = "insmod exited with an error and said nothing: " + err.Error()
		}

		return fmt.Errorf("the robot would not load the driver: %s%s", or(reason, "no reason given"), insmodHint(text))
	}

	// The driver registering and the driver claiming something are different
	// events, and telling them apart is the whole diagnosis.
	registered := false
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)

		if r.hasIface() {
			r.say("[OK] %s came up", Iface)
			return nil
		}
		if !registered && r.driverRegistered() {
			registered = true
			r.say("[*] Driver loaded, waiting for it to claim the adapter")
		}
	}

	if !registered {
		return fmt.Errorf("the driver was accepted but never registered itself, which is not something\n" +
			"    a working module does. Check `pusher ip --adapters` for this hub's kernel")
	}

	return fmt.Errorf("the driver loaded but claimed nothing, so no %s appeared.\n%s", Iface, r.whyNothingClaimed())
}

// alreadyLoaded reports whether insmod refused because the module is in the
// kernel already, which is a state to carry on from rather than fail on.
func alreadyLoaded(out string) bool {
	lower := strings.ToLower(out)
	return strings.Contains(lower, "file exists") || strings.Contains(lower, "already")
}

// carriedDriver is the module pusher carries for this hub.
func (r runner) carriedDriver() (string, error) {
	osVersion := strings.TrimSpace(first(r.run("getprop", "ro.controlhub.os.version")))
	if osVersion == "" {
		osVersion = strings.TrimSpace(first(r.run("getprop", "ro.build.display.id")))
	}
	procVersion := strings.TrimSpace(first(r.run("cat", "/proc/version")))

	// A robot that will not say what it is running is a different problem from
	// one running something unsupported, and the answers are different too.
	if procVersion == "" && osVersion == "" {
		return "", fmt.Errorf("the robot would not say what it is running, so pusher cannot tell\n" +
			"    which driver it needs. Check the connection and try again")
	}

	driver, match := For(osVersion, procVersion)
	if match == NoMatch {
		return "", fmt.Errorf("%s", Unsupported(osVersion, procVersion))
	}

	if match == ByKernel {
		r.say("[*] Using the driver built for Control Hub OS %s: same kernel, which is what the hub checks", driver.OS)
	}

	return Extract(driver)
}

// first is the output of a command, ignoring whether it succeeded: getprop on
// an unset property exits non-zero with nothing to say, which is not an error
// worth stopping for.
func first(out string, _ error) string { return out }

// refused reports whether insmod turned the module down.
func refused(out string) bool {
	lower := strings.ToLower(out)

	for _, sign := range []string{"error", "failed", "no such", "exists", "invalid", "denied", "not permitted"} {
		if strings.Contains(lower, sign) {
			return true
		}
	}
	return false
}

// insmodHint turns insmod's words into what to do about them.
func insmodHint(out string) string {
	lower := strings.ToLower(out)

	switch {
	case strings.Contains(lower, "exec format"):
		return "\n    That is a kernel mismatch: the module was built for a different kernel than\n" +
			"    this hub runs. `pusher ip --adapters` prints the version it needs."
	case strings.Contains(lower, "exists"):
		return "\n    It is already in the kernel."
	case strings.Contains(lower, "denied"), strings.Contains(lower, "not permitted"):
		return "\n    Loading a module needs root, and this hub would not give it."
	}
	return ""
}

// driverRegistered reports whether the module registered itself on the bus,
// which happens whether or not anything is plugged in for it to drive.
func (r runner) driverRegistered() bool {
	out, err := r.run("ls", "-d", "/sys/bus/usb/drivers/rtl8822bu")
	return err == nil && !strings.Contains(strings.ToLower(out), "no such")
}

// whyNothingClaimed says what the hub can see, because a driver that loads and
// claims nothing means either there is no adapter plugged in or the one that is
// there is a chipset this driver does not cover.
func (r runner) whyNothingClaimed() string {
	out, err := r.run("ls", "/sys/bus/usb/devices")
	if err != nil {
		return "    Could not ask the hub what is on its USB bus."
	}

	var seen []string
	for _, entry := range strings.Fields(out) {
		if strings.Contains(entry, ":") || strings.HasPrefix(entry, "usb") {
			continue
		}

		base := "/sys/bus/usb/devices/" + entry
		vendor, _ := r.run("cat", base+"/idVendor")
		product, _ := r.run("cat", base+"/idProduct")
		name, _ := r.run("cat", base+"/product")

		vendor, product = strings.TrimSpace(vendor), strings.TrimSpace(product)
		if vendor == "" {
			continue
		}
		seen = append(seen, fmt.Sprintf("      %s:%s  %s", vendor, product, strings.TrimSpace(name)))
	}

	if len(seen) == 0 {
		return "    Nothing at all is on the hub's USB bus, so the adapter is not plugged in,\n" +
			"    not seated, or not getting power. Try the hub's other port."
	}

	return "    This is what is plugged in:\n" + strings.Join(seen, "\n") + "\n\n" +
		"    If your adapter is in that list, its chipset is not one this driver covers.\n" +
		"    It handles RTL8822BU and RTL8812BU. `pusher ip --adapters` says what else\n" +
		"    this hub can drive."
}

func or(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// hasIface reports whether the adapter has become an interface.
//
// -d matters. /sys/class/net/wlan1 is a symlink to a directory, so a bare ls
// prints what is inside it — address, mtu, operstate — and never the name being
// looked for. Without it this said no on a hub where the interface was plainly
// there, so pusher tried to load a driver that was already loaded and then
// waited ten seconds for something it could not see.
func (r runner) hasIface() bool {
	out, err := r.run("ls", "-d", "/sys/class/net/"+Iface)
	if err != nil {
		return false
	}
	return strings.Contains(out, Iface) && !strings.Contains(strings.ToLower(out), "no such")
}

// associate joins the network.
func (r runner) associate(opt Options) error {
	r.say("[*] Joining %s", opt.SSID)

	// Every supplicant killed by pid and counted. pkill does not do it on this
	// build, and the ones it leaves behind share the radio: they knock each
	// other off, the network gets blacklisted, and a run that worked in four
	// seconds fails forever afterwards for no visible reason.
	if err := r.killSupplicants(); err != nil {
		return err
	}

	// A killed supplicant leaves its socket, and the next one cannot bind it.
	r.quiet("rm", "-f", CtrlDir+"/"+Iface)

	// The interface does not come back from an association in a state a fresh
	// supplicant can scan from.
	r.quiet("ip", "link", "set", Iface, "down")
	time.Sleep(time.Second)
	r.quiet("ip", "link", "set", Iface, "up")

	if err := r.writeConf(opt); err != nil {
		return err
	}

	// setsid, or adbd kills it along with the shell that started it.
	start := []string{"wpa_supplicant", "-B", "-i", Iface, "-Dnl80211", "-c", ConfPath}
	if _, err := r.run("which", "setsid"); err == nil {
		start = append([]string{"setsid"}, start...)
	}
	r.quiet(start...)

	time.Sleep(2 * time.Second)
	if n := r.supplicants(); n != 1 {
		return fmt.Errorf("expected one wpa_supplicant, found %d: more than one shares the radio and none of them can associate", n)
	}

	// Told to connect. A config loaded at startup is not an enabled, selected
	// network, and one with nothing selected sits at DISCONNECTED without ever
	// scanning.
	r.quiet("wpa_cli", "-p", CtrlDir, "-i", Iface, "enable_network", "0")
	r.quiet("wpa_cli", "-p", CtrlDir, "-i", Iface, "select_network", "0")

	for i := 0; i < 30; i++ {
		time.Sleep(2 * time.Second)

		if r.state() == "COMPLETED" {
			r.say("[OK] Associated")
			return nil
		}
	}

	return fmt.Errorf("did not associate with %s in a minute.\n"+
		"    Last state: %s. A wrong passphrase shows up as WRONG_KEY in:\n"+
		"    adb shell \"%s logcat -d\" | grep wpa_supplicant", opt.SSID, r.state(), r.root)
}

func (r runner) state() string {
	out, err := r.run("wpa_cli", "-p", CtrlDir, "-i", Iface, "status")
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(out, "\n") {
		if value, found := strings.CutPrefix(strings.TrimSpace(line), "wpa_state="); found {
			return value
		}
	}
	return ""
}

func (r runner) supplicants() int {
	out, err := r.run("ps")
	if err != nil {
		return 0
	}

	n := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "wpa_supplicant") {
			n++
		}
	}
	return n
}

func (r runner) killSupplicants() error {
	for attempt := 0; attempt < 3; attempt++ {
		out, err := r.run("ps")
		if err != nil {
			return fmt.Errorf("cannot see what the robot is running: %w", err)
		}

		found := false
		for _, line := range strings.Split(out, "\n") {
			if !strings.Contains(line, "wpa_supplicant") {
				continue
			}

			fields := strings.Fields(line)
			if len(fields) > 1 {
				r.quiet("kill", "-9", fields[1])
				found = true
			}
		}

		if !found {
			return nil
		}
		time.Sleep(time.Second)
	}

	return fmt.Errorf("wpa_supplicant would not stay dead, so a new one cannot have the radio to itself")
}
