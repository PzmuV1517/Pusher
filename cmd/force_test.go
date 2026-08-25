package cmd

import (
	"testing"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/spf13/viper"
)

// reload re-reads the config the way a later run would.
func reload(t *testing.T) {
	t.Helper()

	viper.Reset()
	if err := config.Initialize(); err != nil {
		t.Fatal(err)
	}
}

// Swapping a library in the menu happens with no robot connected, so nothing
// on the robot can know about it. Until the flag existed, the next deploy asked
// the robot whether anything had changed, the robot said no, and the reload ran
// against the jar that was there before.
func TestALibraryChangeForcesAnInstall(t *testing.T) {
	settings(t, true)

	if config.GetForceInstall() {
		t.Fatal("a fresh config already demands an install")
	}

	if err := config.SetForceInstall(true); err != nil {
		t.Fatal(err)
	}
	if !config.GetForceInstall() {
		t.Error("the flag did not survive being set")
	}

	// And it has to survive being read by the next process, not just this one.
	reload(t)
	if !config.GetForceInstall() {
		t.Error("the flag was not written to the config file")
	}

	if err := config.SetForceInstall(false); err != nil {
		t.Fatal(err)
	}
	reload(t)
	if config.GetForceInstall() {
		t.Error("clearing the flag did not stick")
	}
}
