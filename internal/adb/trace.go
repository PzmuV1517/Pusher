package adb

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// TraceDir is where the blob library writes path traces on the hub.
const TraceDir = "/sdcard/FIRST/pusher-traces"

// RemoteTrace is one trace file sitting on the robot.
type RemoteTrace struct {
	Path   string
	Name   string
	OpMode string
}

// Shell runs an adb shell command and returns its combined output.
func Shell(serial string, args ...string) (string, error) {
	return run(serial, append([]string{"shell"}, args...)...)
}

// ShellOutput is Shell, keeping what the command printed even when it failed.
//
// Shell throws the output away on a non-zero exit and puts it inside the error
// text, which loses exactly the part worth reading: a tool that refuses says
// why on stderr and then exits non-zero, so "the robot would not load the
// driver" came back with no reason given when insmod had explained itself
// perfectly well.
func ShellOutput(serial string, args ...string) (string, error) {
	full := append([]string{"shell"}, args...)
	if serial != "" {
		full = append([]string{"-s", serial}, full...)
	}

	out, err := exec.Command("adb", full...).CombinedOutput()
	return string(out), err
}

// Pull copies a file off the device.
func Pull(serial, remote, local string) error {
	_, err := run(serial, "pull", remote, local)
	return err
}

// Push copies a file onto the device.
func Push(serial, local, remote string) error {
	_, err := run(serial, "push", local, remote)
	return err
}

// Target picks the robot to talk to, preferring USB the way deploying does.
//
// Both failures are wrapped sentinels rather than plain messages, because what
// a caller should do about them differs: one is worth offering to fix, and the
// other is somebody's afternoon with a package manager.
func Target() (string, error) {
	if !IsInstalled() {
		return "", fmt.Errorf("%w - install Android SDK Platform-Tools", ErrNoADB)
	}
	if dev, ok := FindUSBDevice(); ok {
		return dev.Serial, nil
	}
	if IsConnected() {
		return RobotAddr(), nil
	}
	return "", fmt.Errorf("%w - plug in USB or run `pusher connect`", ErrNoRobot)
}

// ListTraces returns the trace files on the device, newest first.
func ListTraces(serial string) ([]RemoteTrace, error) {
	// A missing directory is a robot that has never recorded a trace, not a
	// failure to look. Without `|| true` the device's ls exit code comes back
	// as adb's, and "you have no traces yet" reads as "adb is broken".
	out, err := Shell(serial, "ls", "-t", TraceDir, "2>/dev/null", "||", "true")
	if err != nil {
		return nil, fmt.Errorf("cannot reach the robot to look in %s: %w", TraceDir, err)
	}

	var traces []RemoteTrace
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(line)
		if !strings.HasSuffix(name, ".json") {
			continue
		}

		traces = append(traces, RemoteTrace{
			Path:   TraceDir + "/" + name,
			Name:   name,
			OpMode: opModeFromName(name),
		})
	}

	return traces, nil
}

func opModeFromName(name string) string {
	base := strings.TrimSuffix(name, ".json")
	if i := strings.LastIndex(base, "-"); i > 0 {
		return base[:i]
	}
	return base
}

// MatchTraces returns the traces whose OpMode matches name, case-insensitively.
func MatchTraces(traces []RemoteTrace, name string) []RemoteTrace {
	if name == "" {
		return traces
	}

	want := strings.ToLower(name)
	var hits []RemoteTrace
	for _, t := range traces {
		if strings.ToLower(t.OpMode) == want {
			hits = append(hits, t)
		}
	}
	if len(hits) > 0 {
		return hits
	}

	for _, t := range traces {
		if strings.Contains(strings.ToLower(t.OpMode), want) {
			hits = append(hits, t)
		}
	}
	return hits
}

// OpModeNames lists the distinct OpModes present, for error messages.
func OpModeNames(traces []RemoteTrace) []string {
	seen := map[string]bool{}
	var names []string
	for _, t := range traces {
		if !seen[t.OpMode] {
			seen[t.OpMode] = true
			names = append(names, t.OpMode)
		}
	}
	sort.Strings(names)
	return names
}
