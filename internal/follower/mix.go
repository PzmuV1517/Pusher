package follower

import "math"

// The mix, and how to run it backwards.
//
// blob turns three volt commands into four wheel powers, and the two negations
// in it are the whole reason this is written down once rather than inline
// wherever it is needed:
//
//	f =  volts_f / battery
//	l = -volts_l / battery      // positive l drives the robot RIGHT in the mix
//	t = -volts_t / battery      // positive t turns the robot CLOCKWISE in the mix
//
//	lf = f + l + t    lb = f - l + t    rf = f - l - t    rb = f + l - t
//
// Running it backwards recovers the twist the four logged wheel commands
// actually carry, which is the valuable derived quantity in the whole file:
// everything else in a row is what the controller wanted, and this is what it
// got. On unsaturated loops the inverse reproduces volts_* exactly. On
// saturated ones it is lossy by design, and that loss is the thing worth
// looking at.

// Wanted is the four wheel commands the controller asked for, before anything
// scaled them.
//
// Not logged, because they are what the log's own columns are the survivors of.
// Recomputing them is the only way to tell a loop that was desaturated from one
// that happened to come out below full scale.
func (r Row) Wanted() (lf, lb, rf, rb float64) {
	if r.Battery <= 0 {
		return 0, 0, 0, 0
	}

	f := r.VoltsF / r.Battery
	l := -r.VoltsL / r.Battery
	t := -r.VoltsT / r.Battery

	return f + l + t, f - l + t, f - l - t, f + l - t
}

// Delivered is the twist the logged wheel commands actually carry, in volts and
// in the same frame and signs as VoltsF, VoltsL and VoltsT.
func (r Row) Delivered() (f, l, t float64) {
	b := r.Battery
	return (r.LF + r.LB + r.RF + r.RB) / 4 * b,
		-(r.LF - r.LB - r.RF + r.RB) / 4 * b,
		-(r.LF + r.LB - r.RF - r.RB) / 4 * b
}

// saturation is where a wheel command counts as over full scale. Slack enough
// that a loop sitting exactly at 1.0 is not called starved on a rounding error.
const saturation = 1.001

// Saturated reports whether the mix could not deliver what the controller asked
// for on this loop.
//
// Worked out from the volts and the battery rather than from the logged wheel
// commands, because desaturation is exactly what makes those two disagree: the
// logged commands are always inside the range, whether or not anything was
// taken off them to get there.
func (r Row) Saturated() bool {
	lf, lb, rf, rb := r.Wanted()

	return math.Abs(lf) > saturation || math.Abs(lb) > saturation ||
		math.Abs(rf) > saturation || math.Abs(rb) > saturation
}

// TurnLoss is the fraction of the turn command that did not survive
// desaturation, 0 when all of it did.
//
// When the mix saturates blob keeps the turn and scales translation down, so
// this is normally zero even on starved loops. It being anything else means the
// turn was the axis that lost out, which means the follower was not in
// path following mode, and that is worth knowing before reading anything else
// on the page.
func (r Row) TurnLoss() float64 {
	if !r.Saturated() {
		return 0
	}

	lf, lb, rf, rb := r.Wanted()
	wanted := (lf + lb - rf - rb) / 4

	if math.Abs(wanted) < 1e-6 {
		return 0
	}

	got := (r.LF + r.LB - r.RF - r.RB) / 4
	loss := 1 - math.Abs(got)/math.Abs(wanted)

	// The inverse reproduces the volts to about 1e-15, so anything at that
	// scale is arithmetic rather than a starved axis. Reporting it turned a
	// run that never lost a millivolt of turn authority into a page warning
	// that the follower was not following a path.
	if loss < 1e-3 {
		return 0
	}
	return loss
}

// Feedback is what the RST controller itself contributed on each axis, which is
// the total less whatever the feedforward terms put in.
//
// Splitting them is what says whether a command is the model driving the robot
// or the controller correcting it. A large total that is nearly all
// feedforward is a loop that is barely working; the same total that is nearly
// all feedback is a loop that is doing all the work.
func (r Row) Feedback() (f, l, t float64) {
	return r.VoltsF - r.StaticF - r.ExtraF,
		r.VoltsL - r.StaticL - r.ExtraL,
		r.VoltsT - r.StaticT
}

// Speed is how fast the robot was actually going, whichever way it was pointed.
func (r Row) Speed() float64 { return math.Hypot(r.VF, r.VL) }
