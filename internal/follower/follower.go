// Package follower reads blob's per-loop follower log: the closed loop's own
// account of every stage of every control loop.
//
// It exists to answer one question that nothing else can. A robot that wobbles
// is either fighting its own mechanics or running code that does not do what
// anybody believes it does, and those have completely different fixes. A path
// trace shows where the robot went, which is the symptom. This shows what the
// controller asked for, what it was given, and what the mix could actually
// deliver, which is where the two explanations separate.
//
// Distinct from the other two things blob writes, and deliberately not merged
// with them: pusher-traces is 20 Hz JSON of where the robot went, blob-sysid is
// an open-loop identification run, and blob scans the trace directory expecting
// traces.
package follower

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/andreibanu/pusher/internal/adb"
)

// Dir is where blob writes follower logs on the hub.
const Dir = "/sdcard/FIRST/blob-follower"

// FormatVersion is the format this understands. Minor changes only ever append
// columns, so a file is read by column name and a different version here means
// the columns themselves moved.
const FormatVersion = 1

// MaxRows is where the recorder stops, silently. Nothing in the file says it
// happened, so a log of exactly this many rows is a run that was cut short
// rather than one that ended.
const MaxRows = 20000

// Two ways to have no logs, and they are not the same thing. One is a robot
// running a build that physically cannot record; the other is a robot that can
// and was not asked to. Showing an empty list for both leaves somebody looking
// for a file that was never going to be there.
var (
	// ErrNoRecorder is the directory not existing at all.
	ErrNoRecorder = errors.New("this robot has no " + Dir + " directory, so nothing has ever\n" +
		"recorded a follower log on it. The competition build cannot: its recorder is a\n" +
		"stub with no file IO in it. Switch to the blob-dev build under `pusher settings`\n" +
		"-> blob library, set BlobParams.recordFollower = true, and run an OpMode that\n" +
		"follows a path")

	// ErrNothingRecorded is the directory being there and empty.
	ErrNothingRecorded = errors.New("the robot has a " + Dir + " directory but no logs in it.\n" +
		"That is a dev build with BlobParams.recordFollower left off, or an OpMode that\n" +
		"never entered the RST follower")
)

// Run is one log sitting on the robot.
type Run struct {
	Path string
	Name string

	// When is the OpMode's start time, which is what the recorder names the
	// file after. It is the only thing in the listing that identifies a run:
	// the file carries no OpMode name at all.
	When time.Time

	// Rows and Bytes come from the robot, in one call, so a list of twenty logs
	// does not mean pulling twenty files to say how big they are. Rows is the
	// one worth reading: a log of 30 rows is half a second of somebody pressing
	// stop, and it looks identical to a real run in a list of timestamps.
	Rows  int
	Bytes int64
}

// Label is how one run is named in a list.
func (r Run) Label() string {
	if r.When.IsZero() {
		return r.Name
	}

	when := r.When.Format("15:04:05")
	if time.Since(r.When) > 12*time.Hour {
		when = r.When.Format("2 Jan 15:04")
	}
	return when
}

// Detail is the size of a run, in the terms somebody chooses by.
func (r Run) Detail() string {
	if r.Rows <= 0 {
		return size(r.Bytes)
	}
	return fmt.Sprintf("%d loops · %s", r.Rows, size(r.Bytes))
}

func size(bytes int64) string {
	switch {
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(bytes)/(1<<10))
	}
	return fmt.Sprintf("%d B", bytes)
}

// List returns the logs on the robot, newest first.
func List(serial string) ([]Run, error) {
	// A missing directory has to be told apart from an unreadable robot, and
	// the device's own ls exit code arrives as adb's. `|| true` keeps the two
	// separate: after it, an error is adb failing rather than a file not being
	// there.
	out, err := adb.Shell(serial, "ls", "-d", Dir, "2>/dev/null", "||", "true")
	if err != nil {
		return nil, fmt.Errorf("cannot reach the robot to look in %s: %w", Dir, err)
	}
	if !strings.Contains(out, Dir) {
		return nil, ErrNoRecorder
	}

	// One call for every file's line count and size. wc prints lines then
	// bytes, and rows are lines less the two header lines.
	out, err = adb.Shell(serial, "wc", "-lc", Dir+"/*.csv", "2>/dev/null", "||", "true")
	if err != nil {
		return nil, fmt.Errorf("cannot reach the robot to measure the logs: %w", err)
	}

	runs := parseListing(out)
	if len(runs) == 0 {
		return nil, ErrNothingRecorded
	}

	sort.SliceStable(runs, func(i, j int) bool { return runs[i].When.After(runs[j].When) })
	return runs, nil
}

// parseListing reads wc's table, which is lines, bytes and then the path.
func parseListing(out string) []Run {
	var runs []Run

	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 3 {
			continue
		}

		path := fields[len(fields)-1]
		if !strings.HasSuffix(path, ".csv") {
			// wc's own total line, and anything else it had to say.
			continue
		}

		lines, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		bytes, _ := strconv.ParseInt(fields[1], 10, 64)

		// The header comment and the column header are not loops.
		rows := lines - 2
		if rows < 0 {
			rows = 0
		}

		name := filepath.Base(path)
		runs = append(runs, Run{
			Path:  path,
			Name:  name,
			When:  startedAt(name),
			Rows:  rows,
			Bytes: bytes,
		})
	}

	return runs
}

// startedAt reads the OpMode's start time back out of the file's name.
func startedAt(name string) time.Time {
	base := strings.TrimSuffix(name, ".csv")

	i := strings.LastIndex(base, "-")
	if i < 0 {
		return time.Time{}
	}

	millis, err := strconv.ParseInt(base[i+1:], 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.UnixMilli(millis)
}

// Pull copies a log off the robot and reads it.
//
// The file on the robot is rewritten whole every couple of seconds while the
// run is going, so what comes back is whatever was complete at the moment it
// was asked for: a valid, shorter log of a run still happening. Nothing in the
// file distinguishes that from a finished one, which is why the page says so
// rather than pretending the number of loops is final.
func Pull(serial string, run Run, local string) (*Log, error) {
	if err := adb.Pull(serial, run.Path, local); err != nil {
		return nil, fmt.Errorf("cannot pull %s off the robot: %w", run.Name, err)
	}

	log, err := Load(local)
	if err != nil {
		return nil, err
	}
	if log.When.IsZero() {
		log.When = run.When
	}
	return log, nil
}
