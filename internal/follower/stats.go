package follower

import "math"

// autotuneDt is the loop period blob's autotune assumes when it derives
// tauClosedLoop.
//
// It matters because nothing checks it. A robot actually running at 30 ms has
// had its pole placed faster than its loop can support, and the result is a
// controller that rings for reasons that look exactly like bad constants.
const autotuneDt = 0.020

// Gap is how far one axis' measurement stayed from what was asked of it.
type Gap struct {
	Axis  string
	Unit  string
	Worst float64
	RMS   float64
}

// Summary is the arithmetic that answers the questions people actually ask of
// one of these logs, all of it cheap enough to do on the way to drawing.
type Summary struct {
	Loops    int
	Duration float64
	Capped   bool
	Partial  bool

	MeanDt, MaxDt, MinDt float64
	Rate                 float64

	// MinTau is the fastest closed loop constant this loop rate can support,
	// four times the worst period it actually ran at.
	MinTau float64

	// SlowLoop marks a run whose worst period is slower than the one autotune
	// assumed, which is what puts the pole out of reach.
	SlowLoop bool

	SaturatedLoops int
	SaturatedPct   float64
	WorstTurnLoss  float64

	Gaps [3]Gap

	WorstHeadingDeg float64
	Reversals       int

	SettledLoops int
	SettledPct   float64

	PeakSpeedRef float64
	PeakSpeed    float64
}

// Summarise reads the whole log once and works out everything the panel shows.
func (l *Log) Summarise() Summary {
	s := Summary{
		Loops:    len(l.Rows),
		Duration: l.Duration(),
		Capped:   l.Capped(),
		Partial:  l.Partial,
		Gaps: [3]Gap{
			{Axis: "forward", Unit: "in/s"},
			{Axis: "left", Unit: "in/s"},
			{Axis: "turn", Unit: "rad/s"},
		},
	}
	if s.Loops == 0 {
		return s
	}

	var sumDt float64
	s.MinDt = math.Inf(1)

	// The reference to measurement gap is only measured while the follower is
	// actually commanding. On a settled row it commands nothing and every
	// reference is written zero, so a robot still coasting to a stop would read
	// as a huge tracking error that no amount of tuning could remove.
	var sq [3]float64
	var commanding int

	for i := range l.Rows {
		r := &l.Rows[i]

		sumDt += r.Dt
		s.MaxDt = math.Max(s.MaxDt, r.Dt)
		s.MinDt = math.Min(s.MinDt, r.Dt)

		if r.Saturated() {
			s.SaturatedLoops++
			s.WorstTurnLoss = math.Max(s.WorstTurnLoss, r.TurnLoss())
		}

		if r.Settled {
			s.SettledLoops++
		} else {
			commanding++
			for axis, gap := range [3]float64{
				r.RefF - r.VF,
				r.RefL - r.VL,
				r.RefOmega - r.Omega,
			} {
				s.Gaps[axis].Worst = math.Max(s.Gaps[axis].Worst, math.Abs(gap))
				sq[axis] += gap * gap
			}
		}

		s.WorstHeadingDeg = math.Max(s.WorstHeadingDeg, math.Abs(degrees(r.HeadingError)))
		s.PeakSpeedRef = math.Max(s.PeakSpeedRef, math.Abs(r.SpeedRef))
		s.PeakSpeed = math.Max(s.PeakSpeed, r.Speed())
	}

	if commanding > 0 {
		for axis := range s.Gaps {
			s.Gaps[axis].RMS = math.Sqrt(sq[axis] / float64(commanding))
		}
	}

	s.MeanDt = sumDt / float64(s.Loops)
	if s.MeanDt > 0 {
		s.Rate = 1 / s.MeanDt
	}
	s.MinTau = 4 * s.MaxDt
	s.SlowLoop = s.MaxDt > autotuneDt

	s.SaturatedPct = 100 * float64(s.SaturatedLoops) / float64(s.Loops)
	s.SettledPct = 100 * float64(s.SettledLoops) / float64(s.Loops)
	s.Reversals = l.Reversals()

	return s
}

// Reversals counts the times the heading error changed sign.
//
// A follower that crosses zero and comes back, over and over, is fighting
// itself: that is one shape a wobble takes, it is obvious here, and it is
// invisible in a path trace, which only shows a line that looks slightly
// thick.
//
// Counted with a deadband, because an error sitting on zero crosses it every
// loop on sensor noise alone, and a count of four thousand says nothing. Only a
// swing that gets properly to one side and then properly to the other is a
// reversal. Counted on the raw column, which is safe either way: mirroring
// flips both sides of a sign change.
func (l *Log) Reversals() int {
	// Half a degree. Below that, a heading loop is done arguing, and every
	// settle band worth having is wider than this.
	const deadband = 0.5

	count, was := 0, 0

	for _, r := range l.Rows {
		deg := degrees(r.HeadingError)
		if math.Abs(deg) < deadband {
			continue
		}

		sign := 1
		if deg < 0 {
			sign = -1
		}

		if was != 0 && sign != was {
			count++
		}
		was = sign
	}

	return count
}

func degrees(radians float64) float64 { return radians * 180 / math.Pi }
