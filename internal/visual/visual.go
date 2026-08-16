package visual

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/pathtrace"
)

// Device picks the robot to talk to.
func Device() (string, error) {
	return adb.Target()
}

// List returns the traces on the robot, newest first.
func List() (string, []adb.RemoteTrace, error) {
	serial, err := Device()
	if err != nil {
		return "", nil, err
	}

	traces, err := adb.ListTraces(serial)
	if err != nil {
		return serial, nil, err
	}
	if len(traces) == 0 {
		return serial, nil, fmt.Errorf("no traces on the robot in %s\n"+
			"Record one: switch to the blob-dev build, set BlobParams.recordTrace = true, "+
			"and call blob.saveTrace() from stop()", adb.TraceDir)
	}
	return serial, traces, nil
}

// Render pulls a trace and writes the HTML, returning the output path.
func Render(serial string, t adb.RemoteTrace, projectRoot, out string, lim pathtrace.Limits) (string, error) {
	local := filepath.Join(os.TempDir(), t.Name)
	if err := adb.Pull(serial, t.Path, local); err != nil {
		return "", err
	}
	return RenderLocal(local, projectRoot, out, lim)
}

// RenderLocal renders a trace file already on disk.
func RenderLocal(local, projectRoot, out string, lim pathtrace.Limits) (string, error) {
	trace, err := pathtrace.Load(local)
	if err != nil {
		return "", err
	}
	return RenderTrace(trace, projectRoot, out, lim)
}

// RenderTrace renders a trace already in memory.
func RenderTrace(trace *pathtrace.Trace, projectRoot, out string, lim pathtrace.Limits) (string, error) {
	if projectRoot == "" {
		projectRoot, _ = os.Getwd()
	}
	trace.Annotate(projectRoot)
	trace.Profile(lim)

	if out == "" {
		out = filepath.Join(os.TempDir(), fmt.Sprintf("pusher-%s.html", safe(trace.OpMode)))
	}
	if err := trace.Render(out, lim); err != nil {
		return "", err
	}
	return out, nil
}

// RenderDemo draws a made up run, so the visualiser can be looked at without a
// robot and without a recorded trace.
func RenderDemo(out string, lim pathtrace.Limits) (string, error) {
	return RenderTrace(pathtrace.Demo(), "", out, lim)
}

// Summary is the one-line description of a rendered run.
func Summary(local string) string {
	trace, err := pathtrace.Load(local)
	if err != nil {
		return ""
	}
	trace.Profile(pathtrace.DefaultLimits())
	_, actual := trace.Totals()
	return fmt.Sprintf("%d legs, %.1fs", len(trace.Segments), actual)
}

func safe(s string) string {
	if s == "" {
		return "trace"
	}
	return strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ' ' || r == ':' {
			return '-'
		}
		return r
	}, s)
}

// Open shows a rendered page in the browser.
func Open(path string) {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", path)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		c = exec.Command("xdg-open", path)
	}
	c.Start()
}
