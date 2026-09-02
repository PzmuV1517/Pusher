package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/andreibanu/pusher/internal/adb"
)

// RecordingDir is where the profiler writes on the hub. It has to agree with
// DIR in the generated source, which is why both are here and why a test
// compares them.
const RecordingDir = "/sdcard/FIRST/pusher-profile"

// Recording is one profiled run sitting on the robot.
type Recording struct {
	Path string
	Name string

	// OpMode and When come out of the file's name, which the profiler builds as
	// <opmode>-<epoch millis>.txt. Two runs of the same OpMode are otherwise
	// indistinguishable in a list, which makes a list of them useless.
	OpMode string
	When   time.Time

	// Problem marks a note the profiler left about why it recorded nothing,
	// which is worth more than the empty directory it would otherwise leave.
	Problem bool
}

// Label is how one run is named in a list: what ran, and when.
func (r Recording) Label() string {
	if r.Problem {
		return "problem"
	}
	if r.When.IsZero() {
		return r.OpMode
	}

	when := r.When.Format("15:04:05")
	if time.Since(r.When) > 12*time.Hour {
		when = r.When.Format("2 Jan 15:04")
	}

	return r.OpMode + "  " + when
}

// describe pulls the OpMode and the time back out of a recording's name.
func describe(name string) (string, time.Time) {
	base := strings.TrimSuffix(name, ".txt")

	i := strings.LastIndex(base, "-")
	if i <= 0 {
		return base, time.Time{}
	}

	millis, err := strconv.ParseInt(base[i+1:], 10, 64)
	if err != nil {
		return base, time.Time{}
	}

	return base[:i], time.UnixMilli(millis)
}

// List returns the recordings on the robot, newest first.
//
// A directory that is not there is not an error: it is what a robot looks like
// before the profiler has written anything, which is the most common reason to
// run this, and reporting it as a failure to list buries the message that would
// have said what to do instead. `|| true` is what makes that distinction
// possible, the device's ls exiting non-zero for a missing directory and adb
// passing that back as its own.
func List(serial string) ([]Recording, error) {
	out, err := adb.Shell(serial, "ls", "-t", RecordingDir, "2>/dev/null", "||", "true")
	if err != nil {
		return nil, fmt.Errorf("cannot reach the robot to look in %s: %w", RecordingDir, err)
	}

	var found []Recording
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(line)
		if !strings.HasSuffix(name, ".txt") {
			continue
		}
		opMode, when := describe(name)
		found = append(found, Recording{
			Path:    RecordingDir + "/" + name,
			Name:    name,
			OpMode:  opMode,
			When:    when,
			Problem: strings.HasPrefix(name, "problem-"),
		})
	}

	return found, nil
}

// Read pulls one recording off the robot and parses it.
func Read(serial string, rec Recording) (Report, error) {
	dir, err := os.MkdirTemp("", "pusher-profile-*")
	if err != nil {
		return Report{}, err
	}
	defer os.RemoveAll(dir)

	local := filepath.Join(dir, "recording.txt")
	if err := adb.Pull(serial, rec.Path, local); err != nil {
		return Report{}, fmt.Errorf("cannot pull %s: %w", rec.Path, err)
	}

	content, err := os.ReadFile(local)
	if err != nil {
		return Report{}, err
	}

	return Parse(string(content))
}

// Attached reports whether the profiler announced itself on the robot.
//
// It logs one line when the robot controller calls its startup hook, so this
// separates a profiler that is running and waiting for a run from one that
// never made it into the APK. Without it, "no recordings" covers both and
// points at neither.
//
// Best effort: the log is a ring buffer, so not finding the line proves less
// than finding it does.
func Attached(serial string) bool {
	out, err := adb.Shell(serial, "logcat", "-d", "-t", "4000", "-s", "PusherProfile", "||", "true")
	if err != nil {
		return false
	}
	return strings.Contains(out, "loop profiler attached")
}

// Clear removes the recordings from the robot.
func Clear(serial string) error {
	_, err := adb.Shell(serial, "rm", "-f", RecordingDir+"/*.txt")
	return err
}
