package power

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/andreibanu/pusher/internal/extreme"
)

const recording = `pusher-power 1
opmode TeleOP
seconds 42.500
period 100
volts 11.20 13.40
device left_front motor 420 2.1000 8.4000 12.300 0
device shooter motor 420 6.5000 21.7000 30.100 0
device right_front motor 420 2.0000 7.9000 12.100 3
device Control_Hub hub 420 14.2000 33.5000 30.100 0
`

func TestParseRanksTheBiggestDrawFirst(t *testing.T) {
	report, err := Parse(recording)
	if err != nil {
		t.Fatal(err)
	}

	if report.OpMode != "TeleOP" {
		t.Errorf("opmode = %q", report.OpMode)
	}
	if report.Seconds != 42.5 || report.Period != 100 {
		t.Errorf("seconds = %v, period = %v", report.Seconds, report.Period)
	}

	// The question this exists to answer is which one is worst, so the worst
	// one has to be first.
	motors := report.Motors()
	if len(motors) != 3 {
		t.Fatalf("got %d motors, want 3", len(motors))
	}
	if motors[0].Name != "shooter" {
		t.Errorf("first motor is %q, want the shooter, which drew the most", motors[0].Name)
	}
	if motors[0].Peak != 21.7 || motors[0].PeakAt != 30.1 {
		t.Errorf("shooter peak = %v at %v", motors[0].Peak, motors[0].PeakAt)
	}

	// A hub reading is everything plugged into it, so ranking it beside the
	// motors would put it top of every list and say nothing.
	for _, m := range motors {
		if m.Kind != "motor" {
			t.Errorf("%s is a %s, which is not a motor", m.Name, m.Kind)
		}
	}
	if hubs := report.Hubs(); len(hubs) != 1 || hubs[0].Name != "Control_Hub" {
		t.Errorf("hubs = %+v", hubs)
	}

	if got := report.Sag(); got < 2.19 || got > 2.21 {
		t.Errorf("sag = %v, want about 2.2V", got)
	}
	if got := report.Total(); got < 10.5 || got > 10.7 {
		t.Errorf("motor total = %v, want the three motor means added up", got)
	}
}

// A device that would not answer is reported with the caveat rather than as a
// device that drew nothing.
func TestFailedReadsAreCarried(t *testing.T) {
	report, err := Parse(recording)
	if err != nil {
		t.Fatal(err)
	}

	for _, d := range report.Devices {
		if d.Name == "right_front" && d.Failures != 3 {
			t.Errorf("failures = %d, want 3", d.Failures)
		}
	}
}

// The robot may be running a newer monitor than this pusher, and a field nobody
// here understands is not a reason to throw away the ones it does.
func TestAnUnknownFieldIsSkippedNotRefused(t *testing.T) {
	report, err := Parse(recording + "servos 4 1.20\ndevice arm motor 10 1.0000 2.0000 1.000 0\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Motors()) != 4 {
		t.Errorf("got %d motors, want the arm to have been read too", len(report.Motors()))
	}
}

func TestSomethingElseIsNotARecording(t *testing.T) {
	if _, err := Parse("hello\n"); err == nil {
		t.Error("arbitrary text parsed as a recording")
	}
	if _, err := Parse("pusher-power 1\nopmode X\n"); err == nil {
		t.Error("a recording with no readings was accepted")
	}
}

func project(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, extreme.SourceRoot), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestInstallAndRemoveRoundTrip(t *testing.T) {
	root := project(t)

	if Installed(root) {
		t.Fatal("an untouched project already has the monitor")
	}

	if err := Install(root); err != nil {
		t.Fatal(err)
	}
	if !Installed(root) {
		t.Fatal("not installed after installing")
	}

	// Twice is not an error: the menu toggles, and somebody may deploy in
	// between.
	if err := Install(root); err != nil {
		t.Fatalf("installing over itself: %v", err)
	}

	if err := Remove(root); err != nil {
		t.Fatal(err)
	}
	if Installed(root) {
		t.Error("still installed after removal")
	}
	if _, err := os.Stat(Dir(root)); err == nil {
		t.Error("the package directory was left behind")
	}

	// Removing what is not there is what a toggle does when the file was
	// deleted by hand.
	if err := Remove(root); err != nil {
		t.Errorf("removing nothing: %v", err)
	}
}

// Somebody's own class at that path is theirs. Overwriting or deleting it
// because of a name collision would be pusher losing somebody's work.
func TestSomebodyElsesFileIsLeftAlone(t *testing.T) {
	root := project(t)

	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	const mine = "package org.firstinspires.ftc.teamcode.pusherpower;\npublic class PusherPowerMonitor {}\n"
	if err := os.WriteFile(File(root), []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}

	if Installed(root) {
		t.Error("a hand-written file was reported as pusher's")
	}
	if err := Install(root); err == nil {
		t.Error("Install overwrote a file pusher did not write")
	}
	if err := Remove(root); err == nil {
		t.Error("Remove deleted a file pusher did not write")
	}

	if got, _ := os.ReadFile(File(root)); string(got) != mine {
		t.Error("the file was changed")
	}
}

// The monitor's entry point is a startup hook, and reloaded classes are scanned
// long after the robot controller has called the hooks it found. So a monitor
// that is not in the APK is collected and never called: installed, and
// measuring nothing.
func TestExtremeKeepsTheMonitorInTheAPK(t *testing.T) {
	root := project(t)

	gradle := filepath.Join(root, extreme.Module, "build.gradle")
	if err := os.MkdirAll(filepath.Dir(gradle), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gradle, []byte("android {\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	source := filepath.Join(root, extreme.SourceRoot, filepath.FromSlash(extreme.TeamPackage))
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "Robot.java"), []byte("class Robot {}"), 0o644); err != nil {
		t.Fatal(err)
	}

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
	if !strings.Contains(string(block), extreme.PowerPackage) {
		t.Errorf("the exclusion does not keep %s in the APK, so the monitor would never run:\n%s",
			extreme.PowerPackage, block)
	}

	// And removing it stops claiming to keep something that is gone.
	if err := Remove(root); err != nil {
		t.Fatal(err)
	}
	block, _ = os.ReadFile(extreme.GradleFile(root))
	if strings.Contains(string(block), extreme.PowerPackage) {
		t.Error("the exclusion still keeps a package that is no longer there")
	}
}

// The path the robot writes to and the path pusher reads from are two constants
// in two languages, and nothing else makes them agree.
func TestTheRecordingPathAgreesWithTheGeneratedSource(t *testing.T) {
	if !strings.Contains(sourceFor(100), `"`+RecordingDir+`"`) {
		t.Errorf("the monitor does not write to %s, which is where pusher looks", RecordingDir)
	}
}

// Two runs of the same OpMode are one file each, and a list of them is only
// useful if they can be told apart. The name carries the time, so the list
// shows it.
func TestRecordingsAreToldApartByTime(t *testing.T) {
	first := "TeleOP-1756089600000.txt"
	second := "TeleOP-1756089999000.txt"

	opA, whenA := describe(first)
	opB, whenB := describe(second)

	if opA != "TeleOP" || opB != "TeleOP" {
		t.Fatalf("opmodes = %q, %q", opA, opB)
	}
	if whenA.IsZero() || whenB.IsZero() {
		t.Fatal("no time was read out of the names")
	}
	if !whenB.After(whenA) {
		t.Error("the later run did not come out later")
	}

	a := Recording{OpMode: opA, When: whenA}
	b := Recording{OpMode: opB, When: whenB}
	if a.Label() == b.Label() {
		t.Errorf("both runs are labelled %q, so the list cannot tell them apart", a.Label())
	}

	// An OpMode whose own name has a hyphen still parses, since only the last
	// one separates the timestamp.
	if op, _ := describe("Close-Blue-1756089600000.txt"); op != "Close-Blue" {
		t.Errorf("opmode = %q, want Close-Blue", op)
	}

	// Anything that is not one of ours is left alone rather than mangled.
	if op, when := describe("notes.txt"); op != "notes" || !when.IsZero() {
		t.Errorf("describe(notes.txt) = %q, %v", op, when)
	}
}

// A monitor that recorded nothing leaves a note saying why, and the note is the
// answer to the question somebody is asking when they find no recordings. It
// has to survive being read back rather than being refused as malformed.
func TestAProblemNoteIsReadNotRefused(t *testing.T) {
	report, err := Parse("pusher-power-problem 1\nThe robot controller handed over an event " +
		"loop with no OpMode manager on it, so the monitor could not attach.\n")
	if err != nil {
		t.Fatalf("a problem note was refused: %v", err)
	}

	if !report.Problem {
		t.Error("the note was not recognised as a problem")
	}
	if !strings.Contains(report.Note, "could not attach") {
		t.Errorf("the explanation was lost: %q", report.Note)
	}
	if report.Title() == "" || !strings.Contains(strings.ToLower(report.Title()), "nothing") {
		t.Errorf("title = %q", report.Title())
	}

	lines := report.Lines()
	if len(lines) == 0 || !strings.Contains(strings.Join(lines, " "), "could not attach") {
		t.Errorf("the note is not shown: %v", lines)
	}
}

func TestProblemFilesAreMarkedInAListing(t *testing.T) {
	ok := Recording{Name: "TeleOP-1756089600000.txt", OpMode: "TeleOP"}
	bad := Recording{Name: "problem-1756089600000.txt", Problem: true}

	if ok.Problem {
		t.Error("a recording was marked as a problem")
	}
	if bad.Label() != "problem" {
		t.Errorf("a problem note is labelled %q", bad.Label())
	}
}

// The page is the feature now, so it has to survive a report that has no series
// in it, a problem note, and a run with nothing but a hub.
func TestThePageRendersWhateverItIsGiven(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"aggregates only, no series", recording},
		{"a problem note", "pusher-power-problem 1\nCould not attach.\n"},
		{"a hub and nothing else", "pusher-power 1\nopmode X\nseconds 1\nperiod 100\n" +
			"device Control_Hub hub 10 1.0000 2.0000 0.500 0\n"},
	} {
		report, err := Parse(tc.body)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}

		out := filepath.Join(t.TempDir(), "page.html")
		if _, err := report.Render(out); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}

		page, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}

		// No network. This opens on a laptop that may be on a robot's Wi-Fi
		// with no route anywhere, so a page that fetches anything renders as an
		// empty box. Inline script is fine and is how the graph is worked; it
		// is the fetching that cannot happen, not the scripting.
		for _, forbidden := range []string{"http://", "https://", "src=", "@import"} {
			if strings.Contains(string(page), forbidden) {
				t.Errorf("%s: the page references %q, which will not load off a robot network", tc.name, forbidden)
			}
		}
		if !strings.Contains(string(page), "<html") {
			t.Errorf("%s: that is not a page", tc.name)
		}
	}
}

// The period is baked into the generated file, so the file is the only place it
// can disagree with the setting.
func TestThePeriodIsBakedIn(t *testing.T) {
	for _, ms := range []int{20, 100, 500} {
		out := sourceFor(ms)

		want := "PERIOD_MS = " + strconv.Itoa(ms) + ";"
		if !strings.Contains(out, want) {
			t.Errorf("sourceFor(%d) does not contain %q", ms, want)
		}
		if strings.Contains(out, "%PERIOD%") {
			t.Errorf("sourceFor(%d) left the placeholder in", ms)
		}
	}

	// Nonsense falls back rather than generating a monitor that spins.
	if !strings.Contains(sourceFor(0), "PERIOD_MS = 100;") {
		t.Error("a period of zero did not fall back to the default")
	}
}
