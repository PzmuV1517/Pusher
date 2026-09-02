package adb

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// fakeADB puts an adb on the path that answers `devices` with whatever is given.
func fakeADB(t *testing.T, deviceList string) {
	t.Helper()

	dir := t.TempDir()
	// A full PATH inside the script, because the test replaces the outer one
	// with a directory that has adb in it and nothing else, cat included.
	script := "#!/bin/sh\nPATH=/bin:/usr/bin\ncat <<'EOF'\n" + deviceList + "\nEOF\n"

	if err := os.WriteFile(filepath.Join(dir, "adb"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

// The two ways of having no robot need telling apart, because one of them is
// worth offering to fix and the other is somebody's afternoon with a package
// manager. Callers decide that with errors.Is, so the wrapping is the feature.
func TestTargetSaysWhichKindOfNothing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := Target()
	if !errors.Is(err, ErrNoADB) {
		t.Errorf("no adb on the path gave %v, which no caller can act on", err)
	}
	if errors.Is(err, ErrNoRobot) {
		t.Error("missing platform tools must not read as a robot worth going to get")
	}

	fakeADB(t, "List of devices attached\n")

	_, err = Target()
	if !errors.Is(err, ErrNoRobot) {
		t.Errorf("adb present and nothing attached gave %v, so nothing offers to connect", err)
	}
}

func TestTargetTakesWhatIsAttached(t *testing.T) {
	fakeADB(t, "List of devices attached\n84B7N16919000123\tdevice model:Control_Hub_v1_0")

	serial, err := Target()
	if err != nil {
		t.Fatalf("a hub over USB is a robot: %v", err)
	}
	if serial != "84B7N16919000123" {
		t.Errorf("got %q", serial)
	}
}
