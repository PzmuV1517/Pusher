package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andreibanu/pusher/internal/extreme"
	"github.com/spf13/viper"
)

// recording builds a file shaped like one the robot writes: a loop that reads
// hardware, follows a path and pushes telemetry, with the hardware read
// dominating, which is what a real FTC profile looks like.
func recording() string {
	stacks := [][]string{
		{"dalvik.system.NativeStart.main", "com.qualcomm.robotcore.eventloop.EventLoopManager.run",
			"org.firstinspires.ftc.teamcode.FarBlue.loop", "com.blob.Blob.update",
			"com.qualcomm.hardware.lynx.LynxModule.getBulkData"},
		{"dalvik.system.NativeStart.main", "com.qualcomm.robotcore.eventloop.EventLoopManager.run",
			"org.firstinspires.ftc.teamcode.FarBlue.loop", "com.blob.Blob.update",
			"com.blob.follow.PathFollower.step"},
		{"dalvik.system.NativeStart.main", "com.qualcomm.robotcore.eventloop.EventLoopManager.run",
			"org.firstinspires.ftc.teamcode.FarBlue.loop",
			"org.firstinspires.ftc.teamcode.mechanisms.Spindexer.update"},
	}
	counts := []int{60, 25, 15}

	ids := map[string]int{}
	var nodes []string
	var runs []string
	next, total := 1, 0

	for i, stack := range stacks {
		at := 0
		key := ""

		for j, frame := range stack {
			key += "/" + frame
			if _, seen := ids[key]; !seen {
				self := 0
				if j == len(stack)-1 {
					self = counts[i]
				}
				ids[key] = next
				nodes = append(nodes, fmt.Sprintf("node %d %d %d %s", next, at, self, frame))
				next++
			}
			at = ids[key]
		}

		runs = append(runs, fmt.Sprintf("run %d %d", at, counts[i]))
		total += counts[i]
	}

	head := []string{
		"pusher-profile 1",
		"opmode FarBlue",
		"class org.firstinspires.ftc.teamcode.FarBlue",
		"started 1787700000000",
		"period-ms 10",
		"duration-ms 1000",
		fmt.Sprintf("samples %d", total),
		"missed 0",
		"truncated 0",
	}

	return strings.Join(append(append(head, nodes...), runs...), "\n") + "\n"
}

func TestASampleTreeBecomesTimeSpent(t *testing.T) {
	r, err := Parse(recording())
	if err != nil {
		t.Fatal(err)
	}

	if r.OpMode != "FarBlue" || r.Samples != 100 {
		t.Fatalf("read opmode %q with %d samples", r.OpMode, r.Samples)
	}
	if r.Period != 10*time.Millisecond {
		t.Errorf("period = %v", r.Period)
	}

	// A hundred samples at ten milliseconds is a second, and the run says it
	// took a second, so the profiler saw all of it.
	if got := r.Coverage(); got < 0.99 || got > 1.01 {
		t.Errorf("coverage = %.2f, want 1", got)
	}

	hot := r.Hottest(3)
	if len(hot) != 3 {
		t.Fatalf("ranked %d frames", len(hot))
	}
	if !strings.HasSuffix(hot[0].Name, "LynxModule.getBulkData") {
		t.Errorf("hottest is %q, want the hardware read that was in 60%% of samples", hot[0].Name)
	}
	if got := r.Seconds(hot[0].Self); got < 0.59 || got > 0.61 {
		t.Errorf("60 samples at 10ms is %.2fs, want 0.60", got)
	}
}

// The width of a bar is time spent in a method and everything under it, which
// is the number a flame chart draws. Getting it from self samples alone would
// draw every caller as costing nothing.
func TestTotalsGatherWhatIsUnderneath(t *testing.T) {
	r, err := Parse(recording())
	if err != nil {
		t.Fatal(err)
	}

	at := map[string]Frame{}
	for _, f := range r.Frames {
		at[f.Name] = f
	}

	root, ok := r.Root()
	if !ok {
		t.Fatal("no root")
	}
	if root.Total != 100 {
		t.Errorf("the root covers %d samples, want all 100", root.Total)
	}

	loop := at["org.firstinspires.ftc.teamcode.FarBlue.loop"]
	if loop.Total != 100 {
		t.Errorf("the loop covers %d samples, want all of them", loop.Total)
	}
	if loop.Self != 0 {
		t.Errorf("the loop itself ran for %d samples, want 0: it was always inside something", loop.Self)
	}

	update := at["com.blob.Blob.update"]
	if update.Total != 85 {
		t.Errorf("blob.update covers %d samples, want 85", update.Total)
	}
}

// The team's own code is the part somebody can do something about, so the page
// colours it differently and the table marks it.
func TestTeamCodeIsToldApartFromEverythingElse(t *testing.T) {
	r, err := Parse(recording())
	if err != nil {
		t.Fatal(err)
	}

	team, other := 0, 0
	for _, f := range r.Frames {
		if f.Team() {
			team++
			continue
		}
		other++
	}

	if team != 2 {
		t.Errorf("found %d team frames, want the OpMode and the mechanism", team)
	}
	if other == 0 {
		t.Error("nothing was recognised as library or SDK code")
	}
}

// Children are drawn widest first so a chart reads left to right by cost rather
// than by whatever order the robot happened to see the calls in.
func TestTheWidestBranchIsDrawnFirst(t *testing.T) {
	r, err := Parse(recording())
	if err != nil {
		t.Fatal(err)
	}

	at := map[int]Frame{}
	for _, f := range r.Frames {
		at[f.ID] = f
	}

	for _, f := range r.Frames {
		for i := 1; i < len(f.Kids); i++ {
			if at[f.Kids[i-1]].Total < at[f.Kids[i]].Total {
				t.Errorf("under %s, a narrower branch is drawn before a wider one", f.Name)
			}
		}
	}
}

// A robot may be running a profiler written by a newer pusher. A reader that
// stops at the first line it does not know turns a format that was designed to
// grow into one that breaks.
func TestAnUnknownFieldIsSkippedNotRefused(t *testing.T) {
	body := strings.Replace(recording(), "missed 0", "threads 4\nmissed 0", 1)

	r, err := Parse(body)
	if err != nil {
		t.Fatalf("a recording with an unknown field was refused: %v", err)
	}
	if r.Samples != 100 {
		t.Errorf("the rest of it did not survive: %d samples", r.Samples)
	}
}

func TestSomethingElseIsNotAProfile(t *testing.T) {
	if _, err := Parse("pusher-power 1\nopmode FarBlue\n"); err == nil {
		t.Error("read a power recording as a profile")
	}
	if _, err := Parse(""); err == nil {
		t.Error("read an empty file as a profile")
	}
}

// The profiler leaves a note rather than an empty directory when it records
// nothing, and that note has to come back as an explanation rather than as a
// parse failure.
func TestAProblemNoteIsReadNotRefused(t *testing.T) {
	r, err := Parse("pusher-profile-problem 1\nmessage The profiler could not attach: nope\n")
	if err != nil {
		t.Fatalf("a problem note was refused: %v", err)
	}
	if !strings.Contains(r.Problem, "could not attach") {
		t.Errorf("problem = %q", r.Problem)
	}
}

func TestInstallAndRemoveRoundTrip(t *testing.T) {
	root := projectRoot(t)

	if Installed(root) {
		t.Fatal("a fresh project already has a profiler in it")
	}
	if err := Install(root); err != nil {
		t.Fatal(err)
	}
	if !Installed(root) {
		t.Fatal("installed and then not found")
	}

	body, err := os.ReadFile(File(root))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "package org.firstinspires.ftc.teamcode."+Package+";") {
		t.Error("the generated file is not in the package it is filed under")
	}

	if err := Remove(root); err != nil {
		t.Fatal(err)
	}
	if Installed(root) {
		t.Error("removed and still there")
	}
}

// Somebody's own class at that path is not pusher's to overwrite or to delete.
func TestSomebodyElsesFileIsLeftAlone(t *testing.T) {
	root := projectRoot(t)

	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	mine := []byte("// mine\npackage org.firstinspires.ftc.teamcode." + Package + ";\n")
	if err := os.WriteFile(File(root), mine, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Install(root); err == nil {
		t.Error("overwrote a file pusher did not write")
	}
	if err := Remove(root); err == nil {
		t.Error("deleted a file pusher did not write")
	}

	body, _ := os.ReadFile(File(root))
	if string(body) != string(mine) {
		t.Error("the file was changed anyway")
	}
}

// A profiler that is reloaded rather than packaged never runs: its hook is
// called when the web server comes up, long before anything is reloaded.
func TestExtremeKeepsTheProfilerInTheAPK(t *testing.T) {
	root := projectRoot(t)

	if err := extreme.Exclude(root); err != nil {
		t.Fatal(err)
	}
	if err := Install(root); err != nil {
		t.Fatal(err)
	}

	block, err := os.ReadFile(extreme.GradleFile(root))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(block), extreme.ProfilePackage) {
		t.Errorf("the exclusion does not keep the profiler in the APK:\n%s", block)
	}

	if err := Remove(root); err != nil {
		t.Fatal(err)
	}

	block, _ = os.ReadFile(extreme.GradleFile(root))
	if strings.Contains(string(block), extreme.ProfilePackage) {
		t.Error("the gradle block still names a profiler that is gone")
	}
}

// The path in the generated source and the path pusher reads from are two
// copies of one fact, and a run written where nothing looks for it is a feature
// that silently does nothing.
func TestTheRecordingPathAgreesWithTheGeneratedSource(t *testing.T) {
	if !strings.Contains(source, `"`+RecordingDir+`"`) {
		t.Errorf("the profiler writes somewhere other than %s", RecordingDir)
	}
}

// The period lives in the generated file, because the robot cannot read this
// laptop's settings.
func TestThePeriodIsBakedIn(t *testing.T) {
	if strings.Contains(sourceFor(5), "%PERIOD%") {
		t.Error("the placeholder survived into the generated file")
	}
	if !strings.Contains(sourceFor(5), "PERIOD_MS = 5;") {
		t.Error("the chosen period is not in the generated file")
	}
	if !strings.Contains(sourceFor(0), fmt.Sprintf("PERIOD_MS = %d;", DefaultPeriod)) {
		t.Error("an unset period did not fall back to the default")
	}
}

func TestRecordingsAreToldApartByTime(t *testing.T) {
	name, when := describe("FarBlue-1756089600000.txt")
	if name != "FarBlue" {
		t.Errorf("opmode = %q", name)
	}
	if when.IsZero() {
		t.Error("the time did not come back out of the name")
	}

	// A name with no timestamp is still a recording, just one that cannot be
	// placed in time.
	name, when = describe("something.txt")
	if name != "something" || !when.IsZero() {
		t.Errorf("got %q %v", name, when)
	}
}

func TestThePageRendersWhateverItIsGiven(t *testing.T) {
	r, err := Parse(recording())
	if err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "page.html")
	if _, err := r.Render(out); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	page := string(body)

	for _, want := range []string{"FarBlue", "Flame chart", "getBulkData", "not for use in a match"} {
		if !strings.Contains(page, want) {
			t.Errorf("the page does not mention %q", want)
		}
	}

	// A flame graph grows from its root: the widest bars at the bottom, the
	// stack rising off them, and the code that was actually executing along the
	// top. Drawn the other way up it is an icicle chart, which is a different
	// convention and reads as upside down to anybody who has seen one before.
	if !strings.Contains(page, "column-reverse") {
		t.Error("the chart is drawn downwards from the root, which is upside down for a flame graph")
	}

	// Everything inline, because this opens on a laptop that may be on a
	// robot's Wi-Fi with no route anywhere.
	for _, forbidden := range []string{"<script src=", "<link rel=\"stylesheet\"", "https://"} {
		if strings.Contains(page, forbidden) {
			t.Errorf("the page fetches something: %q", forbidden)
		}
	}

	// A problem note has to render as a page too, or the one run that explains
	// itself is the one that cannot be opened.
	problem, err := Parse("pusher-profile-problem 1\nmessage nothing attached\n")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := problem.Render(filepath.Join(t.TempDir(), "problem.html")); err != nil {
		t.Errorf("a problem note would not render: %v", err)
	}
}

func projectRoot(t *testing.T) string {
	t.Helper()

	t.Setenv("HOME", t.TempDir())
	viper.Reset()
	t.Cleanup(viper.Reset)

	root := t.TempDir()

	// With team code in it, because a project without any is not one anybody
	// sets Pusher Extreme up on, and the keep list is resolved against what is
	// actually there.
	team := filepath.Join(root, extreme.SourceRoot, filepath.FromSlash(extreme.TeamPackage))
	if err := os.MkdirAll(team, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(team, "Robot.java"), []byte("class Robot {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, extreme.Module), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, extreme.Module, "build.gradle"), []byte("android { }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}
