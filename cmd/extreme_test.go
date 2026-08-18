package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/extreme"
	"github.com/spf13/viper"
)

// project makes the smallest thing extreme.Exclude will write to.
func project(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	source := filepath.Join(root, extreme.SourceRoot, filepath.FromSlash(extreme.TeamPackage))
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "Robot.java"), []byte("class Robot {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	gradle := filepath.Join(root, extreme.Module, "build.gradle")
	if err := os.MkdirAll(filepath.Dir(gradle), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gradle, []byte("android {\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	return root
}

// settings gives the test its own config, since the whole point is to set one.
func settings(t *testing.T, extremeOn bool) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())
	viper.Reset()

	if err := config.Initialize(); err != nil {
		t.Fatal(err)
	}
	if err := config.SetExtreme(extremeOn); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(viper.Reset)
}

// Turning Pusher Extreme off does not put team code back in the APK. Only
// undoing the setup does, and until it is undone every install produces an APK
// with no OpModes in it that has to be reloaded regardless of the setting.
//
// This was reported as one new OpMode missing while every older one stayed:
// the robot was still serving the dex from the last reload that ran, so
// everything written before it was there and everything written after it was
// not.
func TestAnEmptyAPKIsReloadedEvenWithTheSettingOff(t *testing.T) {
	root := project(t)
	settings(t, false)

	if !apkCarriesTeamCode(root) {
		t.Fatal("an untouched project already reads as excluded")
	}

	if err := extreme.Exclude(root); err != nil {
		t.Fatal(err)
	}

	if apkCarriesTeamCode(root) {
		t.Error("the APK is reported as carrying team code that the gradle file excludes, " +
			"so the install would be left without a reload")
	}
}

func TestTheSettingDoesNotDecideWhatIsInTheAPK(t *testing.T) {
	root := project(t)

	if err := extreme.Exclude(root); err != nil {
		t.Fatal(err)
	}

	for _, on := range []bool{true, false} {
		settings(t, on)

		if apkCarriesTeamCode(root) {
			t.Errorf("extreme=%v changed what the gradle file packages", on)
		}
	}
}

func TestUndoingTheSetupPutsTeamCodeBack(t *testing.T) {
	root := project(t)
	settings(t, true)

	if err := extreme.Exclude(root); err != nil {
		t.Fatal(err)
	}
	if err := extreme.Include(root); err != nil {
		t.Fatal(err)
	}

	if !apkCarriesTeamCode(root) {
		t.Error("team code is still excluded after the exclusion was removed")
	}
}
