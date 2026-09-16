package follower

import (
	"math"
	"testing"
)

// mixed is a loop where nothing saturated: the wheels carry exactly what the
// controller asked for.
func mixed(voltsF, voltsL, voltsT, battery float64) Row {
	r := Row{VoltsF: voltsF, VoltsL: voltsL, VoltsT: voltsT, Battery: battery}
	r.LF, r.LB, r.RF, r.RB = r.Wanted()
	return r
}

// The inverse is the valuable derived quantity in the whole file, and its two
// negations are the easiest thing here to get wrong. On a loop that did not
// saturate it has to give back exactly what went in, or every conclusion drawn
// from "asked against delivered" is drawn from arithmetic rather than from the
// robot.
func TestTheInverseGivesBackTheVoltsWhenNothingSaturated(t *testing.T) {
	for _, tc := range []struct{ f, l, a float64 }{
		{6.5, 1.2, 3.4},
		{-4.0, 2.5, -1.75},
		{0, 0, 2},
		{7.8, -3.3, 0},
	} {
		r := mixed(tc.f, tc.l, tc.a, 12.4)

		f, l, a := r.Delivered()
		if math.Abs(f-tc.f) > 1e-12 || math.Abs(l-tc.l) > 1e-12 || math.Abs(a-tc.a) > 1e-12 {
			t.Errorf("asked (%v %v %v), delivered came back (%v %v %v)", tc.f, tc.l, tc.a, f, l, a)
		}
		if r.Saturated() {
			t.Errorf("called a loop inside full scale saturated: %v", r)
		}
		if r.TurnLoss() != 0 {
			t.Errorf("found turn loss on a loop that lost nothing: %v", r.TurnLoss())
		}
	}
}

// Each axis has to come back on its own axis. A sign error that swapped left
// for turn would still round trip if all three were tested together.
func TestEachAxisComesBackOnItsOwnAxis(t *testing.T) {
	left := mixed(0, 5, 0, 12)
	if f, l, a := left.Delivered(); math.Abs(f) > 1e-12 || math.Abs(l-5) > 1e-12 || math.Abs(a) > 1e-12 {
		t.Errorf("a pure left command came back as (%v %v %v)", f, l, a)
	}

	turn := mixed(0, 0, 5, 12)
	if f, l, a := turn.Delivered(); math.Abs(f) > 1e-12 || math.Abs(l) > 1e-12 || math.Abs(a-5) > 1e-12 {
		t.Errorf("a pure turn command came back as (%v %v %v)", f, l, a)
	}
}

// The mix's own signs, spelled out, because every other check here would pass
// just as well with both negations removed.
func TestPositiveLeftVoltsDriveTheLeftSideForward(t *testing.T) {
	r := mixed(0, 5, 0, 12)

	// Positive volts_l is out of the robot's left, which strafes left: the two
	// wheels that push a mecanum robot left are the left front and the right
	// back.
	if !(r.LF < 0 && r.RB < 0 && r.LB > 0 && r.RF > 0) {
		t.Errorf("a left command mixed to lf %.3f lb %.3f rf %.3f rb %.3f", r.LF, r.LB, r.RF, r.RB)
	}

	// And positive volts_t is CCW, which drives the right side forward.
	turn := mixed(0, 0, 5, 12)
	if !(turn.RF > 0 && turn.RB > 0 && turn.LF < 0 && turn.LB < 0) {
		t.Errorf("a CCW command mixed to lf %.3f lb %.3f rf %.3f rb %.3f",
			turn.LF, turn.LB, turn.RF, turn.RB)
	}
}

// Saturation is worked out from the volts, not from the wheels, because the
// logged wheels are always inside full scale whether or not anything was taken
// off them to get there. Checking them instead would report that no loop in any
// log ever saturated.
func TestSaturationIsFoundInTheVoltsRatherThanTheWheels(t *testing.T) {
	r := Row{VoltsF: 9, VoltsL: 4, VoltsT: 3, Battery: 12}

	// What blob actually writes: the turn kept, translation scaled to fit.
	f, l, a := r.VoltsF/r.Battery, -r.VoltsL/r.Battery, -r.VoltsT/r.Battery
	room := 1 - math.Abs(a)
	scale := room / (math.Abs(f) + math.Abs(l))
	f, l = f*scale, l*scale
	r.LF, r.LB, r.RF, r.RB = f+l+a, f-l+a, f-l-a, f+l-a

	if !r.Saturated() {
		t.Fatal("did not notice a loop the mix could not deliver")
	}

	// Every logged wheel is inside full scale, which is exactly why the logged
	// wheels cannot be the test.
	for _, w := range []float64{r.LF, r.LB, r.RF, r.RB} {
		if math.Abs(w) > 1.001 {
			t.Fatalf("the test's own mix did not desaturate: %v", w)
		}
	}

	// The turn survived, which is what blob promises while it is following a
	// path, so this is not the case worth warning about.
	if loss := r.TurnLoss(); loss != 0 {
		t.Errorf("reported %.3f of the turn lost when all of it survived", loss)
	}

	// And the translation did not, which is the loss that is real.
	got, _, _ := r.Delivered()
	if got >= r.VoltsF {
		t.Errorf("delivered %.3f V forward out of %.3f V asked, which is not short", got, r.VoltsF)
	}
}

// The other way round: when the turn is the axis that loses out, the follower
// was not in path following mode, and that is worth saying out loud rather than
// leaving somebody to tune a loop that was never running.
func TestATurnThatLosesOutIsReported(t *testing.T) {
	r := Row{VoltsF: 10, VoltsL: 0, VoltsT: 6, Battery: 12}

	f, l, a := r.VoltsF/r.Battery, -r.VoltsL/r.Battery, -r.VoltsT/r.Battery
	wheels := []float64{f + l + a, f - l + a, f - l - a, f + l - a}

	worst := 0.0
	for _, w := range wheels {
		worst = math.Max(worst, math.Abs(w))
	}
	for i := range wheels {
		wheels[i] /= worst
	}
	r.LF, r.LB, r.RF, r.RB = wheels[0], wheels[1], wheels[2], wheels[3]

	loss := r.TurnLoss()
	if loss <= 0 {
		t.Fatal("scaled the whole command down and reported no turn loss")
	}
	if math.Abs(loss-(1-1/worst)) > 1e-9 {
		t.Errorf("turn loss %.4f, want %.4f", loss, 1-1/worst)
	}
}

// A battery reading of zero is a divide by zero away from an entire page of
// infinities, and one bad loop should not take the log with it.
func TestALoopWithNoBatteryReadingDoesNotPoisonThePage(t *testing.T) {
	r := Row{VoltsF: 6, VoltsT: 2, Battery: 0}

	if r.Saturated() {
		t.Error("called a loop with no battery reading saturated")
	}
	f, l, a := r.Delivered()
	if math.IsNaN(f) || math.IsNaN(l) || math.IsNaN(a) {
		t.Error("produced a NaN rather than a zero")
	}
}

// The sign trap, written down as a test so that removing the negation fails
// here rather than on a robot.
func TestTheHeadingErrorIsMirroredInTheFile(t *testing.T) {
	// blob's convention: positive asks for a clockwise turn, which is a
	// negative field heading error.
	r := Row{HeadingError: 0.25}

	if r.FieldHeadingError() != -0.25 {
		t.Errorf("field heading error came back as %v, want %v", r.FieldHeadingError(), -0.25)
	}
}

// Splitting the feedforward off is what says whether a command is the model
// driving or the controller correcting.
func TestTheControllersOwnContributionIsWhatIsLeft(t *testing.T) {
	r := Row{
		VoltsF: 6.5, StaticF: 1.0, ExtraF: 0.5,
		VoltsL: 2.0, StaticL: 1.8, ExtraL: 0.2,
		VoltsT: 3.4, StaticT: 1.2,
	}

	f, l, a := r.Feedback()
	if math.Abs(f-5.0) > 1e-12 || math.Abs(l-0.0) > 1e-12 || math.Abs(a-2.2) > 1e-12 {
		t.Errorf("feedback came out (%v %v %v)", f, l, a)
	}
}
