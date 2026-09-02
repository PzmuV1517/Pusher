package profile

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The generated file is compiled by the team's Gradle build, on a machine
// pusher does not control, against whichever SDK they are on. Nothing here
// otherwise checks that it compiles at all, and a typo in it is not a pusher
// bug somebody hits: it is a build that fails on their robot, at their
// competition, in a file they did not write.
//
// So it is compiled here when there is an SDK on the machine to compile it
// against, and skipped when there is not. Skipping is the right answer on a
// machine with no Android SDK rather than a failure, but on the machine pusher
// is developed on this runs.
func TestTheGeneratedProfilerCompiles(t *testing.T) {
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("no javac on this machine")
	}

	classpath := sdkClasspath(t)
	if classpath == "" {
		t.Skip("no FTC SDK jars and android.jar to compile against")
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "src", "org", "firstinspires", "ftc", "teamcode", Package)

	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, Class+".java"), []byte(sourceFor(10)), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command(javac, "-nowarn", "-cp", classpath, "-d",
		filepath.Join(dir, "out"), filepath.Join(src, Class+".java")).CombinedOutput()

	if err != nil {
		t.Fatalf("the generated profiler does not compile:\n%s", out)
	}
}

// sdkClasspath is the FTC SDK as unpacked into the Gradle cache, plus the
// android.jar every one of those classes is written against.
func sdkClasspath(t *testing.T) string {
	t.Helper()

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	android := newestDir(filepath.Join(home, "Library", "Android", "sdk", "platforms"))
	if android == "" {
		android = newestDir(filepath.Join(home, "Android", "Sdk", "platforms"))
	}
	if android == "" {
		return ""
	}

	jar := filepath.Join(android, "android.jar")
	if _, err := os.Stat(jar); err != nil {
		return ""
	}

	aars, err := filepath.Glob(filepath.Join(home, ".gradle", "caches", "modules-2", "files-2.1",
		"org.firstinspires.ftc", "*", "*", "*", "*.aar"))
	if err != nil || len(aars) == 0 {
		return ""
	}

	// One version, the newest, since two on the classpath at once is a pile of
	// duplicate classes rather than a compile.
	pick := map[string]string{}
	for _, aar := range aars {
		name := filepath.Base(aar)
		module := name[:strings.LastIndex(name, "-")]
		if at, seen := pick[module]; !seen || aar > at {
			pick[module] = aar
		}
	}

	parts := []string{jar}
	dir := t.TempDir()

	for module, aar := range pick {
		into := filepath.Join(dir, module)
		if err := os.MkdirAll(into, 0o755); err != nil {
			return ""
		}
		if out, err := exec.Command("unzip", "-oq", aar, "classes.jar", "-d", into).CombinedOutput(); err != nil {
			t.Logf("could not unpack %s: %s", aar, out)
			continue
		}
		parts = append(parts, filepath.Join(into, "classes.jar"))
	}

	if len(parts) < 2 {
		return ""
	}
	sort.Strings(parts)

	return strings.Join(parts, string(os.PathListSeparator))
}

// newestDir is the last entry in a directory by name, which for android-NN is
// the newest platform installed.
func newestDir(parent string) string {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return ""
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "android-") {
			names = append(names, entry.Name())
		}
	}
	if len(names) == 0 {
		return ""
	}

	sort.Slice(names, func(i, j int) bool {
		return len(names[i]) < len(names[j]) || (len(names[i]) == len(names[j]) && names[i] < names[j])
	})
	return filepath.Join(parent, names[len(names)-1])
}
