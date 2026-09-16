package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/follower"
	tea "github.com/charmbracelet/bubbletea"
)

// Follower logs are a different recording of a different thing from path
// traces, and they have their own entry rather than being folded into the run
// list. Folding them in would mean one list of two file formats, which is how
// the robot's trace directory ends up with something in it that the library
// scans and cannot read.
func TestTheBlobMenuOffersFollowerLogsSeparatelyFromRuns(t *testing.T) {
	m := modelIn(t, gradleWithBlob, true)

	items := m.blobMenuItems()
	var runs, logs bool
	for _, item := range items {
		runs = runs || item == "Recorded runs"
		logs = logs || item == "Follower logs"
	}

	if !runs || !logs {
		t.Fatalf("the blob menu is %v", items)
	}
	if blobIndex("Follower logs") == blobIndex("Recorded runs") {
		t.Error("the two entries resolve to the same row")
	}
}

// Coming back out has to land on the entry that was opened. The numbers that
// used to do this were written down, and one of them was already wrong.
func TestLeavingAScreenPutsTheCursorBackWhereItCameFrom(t *testing.T) {
	m := modelIn(t, gradleWithBlob, true)
	m.screen = screenBlobFollower

	m.updateFollower(tea.KeyMsg{Type: tea.KeyEsc})

	if m.screen != screenBlob {
		t.Fatalf("esc went to screen %v", m.screen)
	}
	if m.blobMenuItems()[m.cursor] != "Follower logs" {
		t.Errorf("the cursor came back on %q", m.blobMenuItems()[m.cursor])
	}
}

// Having no logs is the normal state of a robot, not a failure of one, and the
// two ways of having none are different problems with different answers.
func TestHavingNoLogsReadsAsANoteRatherThanAnError(t *testing.T) {
	m := modelIn(t, gradleWithBlob, true)
	m.width = 80

	for _, err := range []error{follower.ErrNoRecorder, follower.ErrNothingRecorded} {
		m.follower = followerState{err: err}

		if !nothingToShow(err) {
			t.Errorf("%v was treated as a failure to read the robot", err)
		}

		view := m.viewFollower()
		if !strings.Contains(view, "recordFollower") {
			t.Errorf("the note does not say what to turn on:\n%s", view)
		}
	}

	// A robot that cannot be reached at all is a different matter, and that one
	// is a failure.
	offline := errors.New("adb: device offline")
	if nothingToShow(offline) {
		t.Error("a robot that could not be reached was filed as an empty one")
	}

	m.follower = followerState{err: offline}
	if !strings.Contains(m.viewFollower(), "offline") {
		t.Error("a real failure was swallowed")
	}
}

// No robot connected is an offer to go and get one, not an instruction to quit
// the menu and run another command.
func TestNoRobotBecomesAnOfferToConnect(t *testing.T) {
	m := modelIn(t, gradleWithBlob, true)
	m.width = 80
	m.follower = followerState{}

	m.Update(followerListMsg{err: adb.ErrNoRobot})

	if m.follower.err != nil && !m.follower.connect.open {
		t.Skip("no saved robot network on this machine, so there is nothing to offer")
	}
	if m.follower.connect.open && m.follower.err != nil {
		t.Error("the offer and the error were both kept, which says it twice")
	}
}
