package cmd

import (
	"strings"
	"testing"

	"github.com/andreibanu/pusher/internal/config"
)

// The question has to name the network, because saying yes takes the laptop off
// the one it is on. "Connect?" is not enough to agree to that.
func TestTheQuestionNamesTheNetwork(t *testing.T) {
	settings(t, false)

	if got := connectQuestion(); !strings.Contains(got, "Connect to the robot") {
		t.Errorf("with no profile the question should still be askable, got %q", got)
	}

	if err := config.AddProfile("default", "14270-RC", "password"); err != nil {
		t.Fatal(err)
	}

	got := connectQuestion()
	if !strings.Contains(got, "14270-RC") {
		t.Errorf("question = %q, which does not say what it is about to join", got)
	}
}

// A pipe cannot answer, and a question written to one is a script that hangs
// with nothing on screen saying why. Scripts keep the error they always got.
func TestAPipeIsNeverAsked(t *testing.T) {
	settings(t, false)
	t.Setenv("PATH", t.TempDir())

	called := false
	old := connectNow
	connectNow = func() error {
		called = true
		return nil
	}
	t.Cleanup(func() { connectNow = old })

	if _, err := requireRobot(); err == nil {
		t.Fatal("no adb and no robot should still be an error")
	}
	if called {
		t.Error("connected without anybody agreeing to it")
	}
}
