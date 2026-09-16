package follower

import (
	"math"
	"testing"
)

// On a settled row the follower commands nothing and every reference is written
// zero. That is real data, not a hole, but counting it as tracking error would
// report a robot coasting to a stop as the worst following error of the run and
// hide whatever the real one was.
func TestTheTrackingGapIgnoresTheLoopsThatWereNotCommanding(t *testing.T) {
	l := &Log{Rows: []Row{
		{TimeMs: 0, Dt: 0.016, RefF: 40, VF: 38},
		{TimeMs: 16, Dt: 0.016, RefF: 40, VF: 37},
		// Settled: the references are zero and the robot is still moving.
		{TimeMs: 32, Dt: 0.016, RefF: 0, VF: 30, Settled: true},
	}}

	s := l.Summarise()

	if math.Abs(s.Gaps[0].Worst-3) > 1e-9 {
		t.Errorf("worst forward gap %.3f, want 3 from the commanding loops", s.Gaps[0].Worst)
	}
	if math.Abs(s.Gaps[0].RMS-math.Sqrt((4+9)/2.0)) > 1e-9 {
		t.Errorf("RMS forward gap %.3f, want the two commanding loops only", s.Gaps[0].RMS)
	}
	if math.Abs(s.SettledPct-100.0/3) > 1e-9 {
		t.Errorf("settled %.2f%%", s.SettledPct)
	}
}

// The loop rate is not a detail. blob's autotune derives tauClosedLoop assuming
// 20 ms, so a robot that is actually slower has had its pole placed where its
// loop cannot follow, and the ringing that produces looks exactly like badly
// chosen constants.
func TestTheSlowestLoopSetsWhatTheConstantCanBe(t *testing.T) {
	l := &Log{Rows: []Row{
		{TimeMs: 0, Dt: 0.016},
		{TimeMs: 16, Dt: 0.030},
		{TimeMs: 46, Dt: 0.016},
	}}

	s := l.Summarise()

	if math.Abs(s.MaxDt-0.030) > 1e-9 {
		t.Errorf("worst period %.4f", s.MaxDt)
	}
	if math.Abs(s.MinTau-0.120) > 1e-9 {
		t.Errorf("fastest usable tau %.4f, want four times the worst period", s.MinTau)
	}
	if !s.SlowLoop {
		t.Error("did not notice a loop slower than the one autotune assumes")
	}

	fast := &Log{Rows: []Row{{Dt: 0.016}, {Dt: 0.018}}}
	if fast.Summarise().SlowLoop {
		t.Error("warned about a loop that is inside what autotune assumes")
	}
}

// A follower crossing back and forth through zero is one shape a wobble takes,
// and it is invisible in a path trace. Noise sitting on zero is not the same
// thing and must not be counted as it.
func TestOscillationIsCountedAndNoiseIsNot(t *testing.T) {
	var rows []Row
	for i := 0; i < 60; i++ {
		// Two degrees of swing, three full cycles, so six crossings.
		rows = append(rows, Row{HeadingError: 0.035 * math.Sin(float64(i)*math.Pi/10)})
	}

	if got := (&Log{Rows: rows}).Reversals(); got != 5 {
		t.Errorf("counted %d sign changes in three cycles of a two degree wobble", got)
	}

	var quiet []Row
	for i := 0; i < 600; i++ {
		// A hundredth of a degree of jitter, sitting on zero, which crosses it
		// on almost every loop.
		quiet = append(quiet, Row{HeadingError: 0.0002 * math.Sin(float64(i))})
	}

	if got := (&Log{Rows: quiet}).Reversals(); got != 0 {
		t.Errorf("counted %d sign changes in sensor noise", got)
	}
}

// The recorder stops at MaxRows and writes nothing to say it did, so a run that
// was cut off looks exactly like one that finished.
func TestARunThatHitTheCapIsNotMistakenForOneThatFinished(t *testing.T) {
	rows := make([]Row, MaxRows)
	for i := range rows {
		rows[i].TimeMs = float64(i * 16)
		rows[i].Dt = 0.016
	}

	if !(&Log{Rows: rows}).Capped() {
		t.Error("a log of exactly the cap was treated as a complete run")
	}
	if (&Log{Rows: rows[:MaxRows-1]}).Capped() {
		t.Error("a log one row short of the cap was called cut off")
	}
}

// Peak asked against peak reached, which is the question somebody asks before
// deciding the path is too slow.
func TestPeakSpeedComparesWhatWasAskedWithWhatHappened(t *testing.T) {
	l := &Log{Rows: []Row{
		{Dt: 0.016, SpeedRef: 50, VF: 30, VL: 40},
		{Dt: 0.016, SpeedRef: 20, VF: 10, VL: 0},
	}}

	s := l.Summarise()
	if s.PeakSpeedRef != 50 {
		t.Errorf("peak asked %.1f", s.PeakSpeedRef)
	}
	if math.Abs(s.PeakSpeed-50) > 1e-9 {
		t.Errorf("peak reached %.3f, want the hypotenuse of the two axes", s.PeakSpeed)
	}
}
