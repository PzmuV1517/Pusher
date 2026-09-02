package robot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/spf13/viper"
)

func withConfig(t *testing.T) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())
	viper.Reset()

	if err := config.Initialize(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(viper.Reset)
}

// A prompt that says "connect?" is asking somebody to agree to something it has
// not told them: connecting drops whatever network the laptop is on. Naming the
// network is the difference between a question and a surprise.
func TestNetworkNamesWhatWouldBeJoined(t *testing.T) {
	withConfig(t)

	if got := Network(); got != "" {
		t.Errorf("nothing configured, so there is nothing to name, got %q", got)
	}

	if err := config.AddProfile("default", "14270-RC", "password"); err != nil {
		t.Fatal(err)
	}

	if got := Network(); got != "14270-RC" {
		t.Errorf("Network = %q, want the profile's SSID", got)
	}
}

// Having no profile is not a reason to refuse: somebody already sitting on the
// robot's network needs no profile for adb to reach it, and refusing there
// leaves them being told to go and configure Wi-Fi they are already on.
func TestOnlyMissingToolsRuleConnectingOut(t *testing.T) {
	withConfig(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "adb"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir)
	if !Possible() {
		t.Error("adb is there and only the profile is missing, which connecting does not need")
	}

	t.Setenv("PATH", t.TempDir())
	if Possible() {
		t.Error("offered to connect with no adb to connect with")
	}
}
