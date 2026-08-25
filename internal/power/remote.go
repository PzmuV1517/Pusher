package power

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/andreibanu/pusher/internal/adb"
)

// RecordingDir is where the monitor writes on the hub. It has to agree with DIR
// in the generated source, which is why both are here.
const RecordingDir = "/sdcard/FIRST/pusher-power"

// Recording is one recorded run sitting on the robot.
type Recording struct {
	Path string
	Name string

	// OpMode and When come out of the file's name, which the monitor builds as
	// <opmode>-<epoch millis>.txt. Two runs of the same OpMode are otherwise
	// indistinguishable in a list, which makes a list of them useless.
	OpMode string
	When   time.Time

	// Problem marks a note the monitor left about why it recorded nothing,
	// rather than a recording. Worth more than the empty directory it would
	// otherwise have left behind.
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
// A directory that is not there is not an error. It is what a robot looks like
// before the monitor has ever written anything, which is the most common reason
// somebody runs this, and reporting it as a failure to list buries the one
// message that would have told them what to do instead.
//
// `|| true` is what makes that distinction possible: the device's ls exits
// non-zero for a missing directory, and adb passes that back as its own exit
// code, so without it there is no telling that apart from adb failing to reach
// the robot at all. With it, the only thing left that can fail is adb.
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
	dir, err := os.MkdirTemp("", "pusher-power-*")
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

// Attached reports whether the monitor announced itself on the robot.
//
// It logs one line when the robot controller calls its startup hook, so this is
// the difference between a monitor that is running and waiting for a run, and
// one that never made it into the APK. Without it, "no recordings" covers both
// and points at neither.
//
// Best effort. The log is a ring buffer, so a robot that has been on for a long
// time may have rotated the line away, and not finding it proves less than
// finding it does.
func Attached(serial string) bool {
	out, err := adb.Shell(serial, "logcat", "-d", "-t", "4000", "-s", "PusherPower", "||", "true")
	if err != nil {
		return false
	}
	return strings.Contains(out, "power monitor attached")
}

// Clear removes the recordings from the robot.
func Clear(serial string) error {
	_, err := adb.Shell(serial, "rm", "-f", RecordingDir+"/*.txt")
	return err
}
