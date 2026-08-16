package pathtrace

import (
	"math"
	"time"
)

// Seeing what the visualiser produces should not require a robot, a recorded
// run, and a competition-illegal build. This is a made up auto with the shape
// of a real one: legs of different lengths, a couple of them power limited so
// they colour differently, and a total that lands slightly behind the model,
// which is what a real trace does.

// leg is one path in the made up route.
type leg struct {
	label   string
	kind    string
	from    Point
	control *Point
	to      Point
	power   float64
	// slip is how much slower the run actually went than the model says, as a
	// fraction. Real runs lose a little to the wall and to settling.
	slip float64
}

// demoRoute is a plausible autonomous: leave the wall, score, collect, push
// into the zone, park across the field.
var demoRoute = []leg{
	{"leaveWall", "line", Point{12, 60, 0}, nil, Point{36, 60, 0}, 1.0, 0.06},
	{"scorePreload", "curve", Point{36, 60, 0}, &Point{54, 62, 0}, Point{56, 96, 1.57}, 0.9, 0.10},
	{"toFirstSample", "curve", Point{56, 96, 1.57}, &Point{42, 112, 0}, Point{34, 120, 3.14}, 0.85, 0.08},
	{"pushIntoZone", "line", Point{34, 120, 3.14}, nil, Point{34, 133, 3.14}, 0.4, 0.22},
	{"backOut", "line", Point{34, 133, 3.14}, nil, Point{34, 116, 3.14}, 0.7, 0.05},
	{"park", "curve", Point{34, 116, 3.14}, &Point{78, 140, 0}, Point{116, 118, 0}, 1.0, 0.07},
}

// Demo is a made up autonomous run, for looking at the visualiser without a
// robot and without a recorded trace.
func Demo() *Trace {
	lim := DefaultLimits()

	t := &Trace{
		Version:      1,
		OpMode:       "DemoAuto (sample)",
		RecordedAtMs: time.Now().UnixMilli(),
	}

	elapsed := 0.0

	for i, l := range demoRoute {
		curve := l.points()

		_, _, seconds, _ := profileCurve(curve, l.power, lim)
		actual := seconds * (1 + l.slip)

		seg := Segment{
			Index:    i,
			Type:     l.kind,
			StartMs:  int64(elapsed * 1000),
			EndMs:    int64((elapsed + actual) * 1000),
			MaxPower: l.power,
			Start:    l.from,
			Target:   l.to,
			Curve:    curve,
			Label:    l.label,
			Source:   "DemoAuto.java:" + itoa(40+i*12),
		}

		// One leg carries the markers, so both kinds appear on the page without
		// every leg being cluttered with them.
		if i == 2 {
			mid := curve[len(curve)/2]
			seg.Intercept = &Point{X: mid[0], Y: mid[1], H: l.to.H}
			seg.Waypoints = [][]float64{{l.from.X, l.from.Y}, {l.to.X, l.to.Y}}
		}

		t.Segments = append(t.Segments, seg)
		t.Samples = append(t.Samples, sampleLeg(i, curve, elapsed, actual)...)

		elapsed += actual
	}

	t.DurationMs = int64(elapsed * 1000)
	return t
}

// points walks the leg into the polyline the renderer draws and the model
// profiles.
func (l leg) points() [][]float64 {
	const steps = 48

	out := make([][]float64, 0, steps+1)
	for i := 0; i <= steps; i++ {
		u := float64(i) / steps

		if l.control == nil {
			out = append(out, []float64{
				l.from.X + (l.to.X-l.from.X)*u,
				l.from.Y + (l.to.Y-l.from.Y)*u,
			})
			continue
		}

		// Quadratic bezier, which is what a two point path with one control
		// looks like closely enough to profile the same way.
		inv := 1 - u
		out = append(out, []float64{
			inv*inv*l.from.X + 2*inv*u*l.control.X + u*u*l.to.X,
			inv*inv*l.from.Y + 2*inv*u*l.control.Y + u*u*l.to.Y,
		})
	}

	return out
}

// sampleLeg makes the motion samples a real run records, at 20ms.
func sampleLeg(index int, curve [][]float64, startSeconds, seconds float64) []Sample {
	const period = 0.02

	count := int(seconds / period)
	if count < 2 {
		count = 2
	}

	out := make([]Sample, 0, count)
	for i := 0; i < count; i++ {
		u := float64(i) / float64(count-1)

		at := u * float64(len(curve)-1)
		j := int(at)
		if j >= len(curve)-1 {
			j = len(curve) - 2
		}
		f := at - float64(j)

		x := curve[j][0] + (curve[j+1][0]-curve[j][0])*f
		y := curve[j][1] + (curve[j+1][1]-curve[j][1])*f

		// Trapezoid: up to speed, hold, back down. Close enough to look like a
		// run rather than a straight line.
		v := math.Min(1, math.Min(u, 1-u)*6)

		out = append(out, Sample{
			T:        int64((startSeconds + u*seconds) * 1000),
			X:        x,
			Y:        y,
			H:        math.Atan2(curve[j+1][1]-curve[j][1], curve[j+1][0]-curve[j][0]),
			V:        v * DefaultLimits().TopSpeed,
			Progress: u,
			Segment:  index,
		})
	}

	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
