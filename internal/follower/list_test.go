package follower

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeHub puts an adb on the path that answers the way a Control Hub does, for
// whichever of the three states this robot is in.
//
// The states are the point. A robot with no directory is running a build whose
// recorder is a stub with no file IO in it, a robot with an empty one was never
// asked to record, and those two send somebody to completely different places.
func fakeHub(t *testing.T, listing string, hasDir bool) {
	t.Helper()

	dir := t.TempDir()
	answer := ""
	if hasDir {
		answer = Dir
	}

	script := `#!/bin/sh
PATH=/bin:/usr/bin
while [ $# -gt 0 ]; do
  case "$1" in
    shell) shift; break ;;
    *) shift ;;
  esac
done

if [ "$1" = "ls" ]; then
  printf '%s' "` + answer + `"
  [ -n "` + answer + `" ] && echo
  exit 0
fi

if [ "$1" = "wc" ]; then
  cat <<'LISTING'
` + listing + `
LISTING
  exit 0
fi
exit 0
`
	if err := os.WriteFile(filepath.Join(dir, "adb"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

// The listing has to come out of one call. A menu that pulled twenty files to
// say how many loops were in each of them would take a minute to open, and the
// robot already knows.
func TestTheListingCountsLoopsWithoutPullingAnything(t *testing.T) {
	fakeHub(t, "     1202    412033 "+Dir+"/follower-1700000100000.csv\n"+
		"     3402   1204221 "+Dir+"/follower-1700000000000.csv\n"+
		"     4604   1616254 total", true)

	runs, err := List("robot")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("found %d logs, want 2 and not wc's own total line", len(runs))
	}

	// Newest first, the way every other list in pusher is ordered.
	if runs[0].When.UnixMilli() != 1700000100000 {
		t.Errorf("listed %v first", runs[0].When)
	}

	// Two of those lines are the format marker and the column header, so they
	// are not loops. Counting them would put every run two loops over.
	if runs[0].Rows != 1200 || runs[1].Rows != 3400 {
		t.Errorf("counted %d and %d loops", runs[0].Rows, runs[1].Rows)
	}
	if runs[0].Bytes != 412033 {
		t.Errorf("read the size as %d", runs[0].Bytes)
	}
	if !strings.Contains(runs[0].Detail(), "1200 loops") {
		t.Errorf("the listing says %q", runs[0].Detail())
	}
}

func TestARobotThatCannotRecordIsToldApartFromOneThatDidNot(t *testing.T) {
	fakeHub(t, "", false)
	if _, err := List("robot"); !errors.Is(err, ErrNoRecorder) {
		t.Errorf("a robot with no directory came back as %v", err)
	}

	fakeHub(t, "", true)
	if _, err := List("robot"); !errors.Is(err, ErrNothingRecorded) {
		t.Errorf("a robot with an empty directory came back as %v", err)
	}
}

// Both messages have to say what to do next, because "no logs" on its own is
// the one thing somebody already knows.
func TestBothWaysOfHavingNoLogsSayWhatToDo(t *testing.T) {
	if !strings.Contains(ErrNoRecorder.Error(), "recordFollower") ||
		!strings.Contains(ErrNoRecorder.Error(), "competition") {
		t.Errorf("the missing directory message does not explain itself:\n%v", ErrNoRecorder)
	}
	if !strings.Contains(ErrNothingRecorded.Error(), "recordFollower") {
		t.Errorf("the empty directory message does not explain itself:\n%v", ErrNothingRecorded)
	}
}
