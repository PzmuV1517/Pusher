package pathtrace

import (
	"math"
	"strings"
	"testing"
)

// blob describes a straight leg with the only two points it needs: its ends.
// Both are stationary, so a two point line profiled as it arrives is a segment
// that is stopped everywhere it has a point, and the model returned nothing for
// it. Most of an auto is straight legs, so the run's estimate, every peak and
// the colour scale went with it: a page reporting 0.00 s against a measured
// 28.97 and a speed ramp topping out at 1 in/s.
//
// The demo trace hid this for the life of the feature by sampling even its
// straight legs 49 times.
func TestAStraightLegIsEstimated(t *testing.T) {
	line := [][]float64{{0, 0}, {48, 0}}

	trace := &Trace{Segments: []Segment{{Type: "line", MaxPower: 1, Curve: line}}}
	trace.Profile(DefaultLimits())

	seg := trace.Segments[0]

	if math.Abs(seg.Length-48) > 0.01 {
		t.Errorf("length = %.2f, want 48", seg.Length)
	}
	if seg.EstSeconds <= 0 {
		t.Fatal("a 48 inch drive was estimated to take no time at all")
	}
	if seg.PeakSpeed <= 0 {
		t.Error("the robot never moves on a leg it crosses in a second")
	}

	// Against the arithmetic rather than against itself: 48 inches under these
	// limits is an accelerate-cruise-brake trapezoid, and a sane answer is
	// somewhere between a second and two.
	if seg.EstSeconds < 1 || seg.EstSeconds > 2 {
		t.Errorf("estimate of %.2fs for a 48 inch leg is not believable", seg.EstSeconds)
	}
}

// Resampling must not move the path. It exists so the model has somewhere to
// put the speed, and a version of it that smoothed corners would be drawing a
// route the robot was never asked to drive.
func TestResamplingLeavesTheShapeAlone(t *testing.T) {
	corner := [][]float64{{0, 0}, {40, 0}, {40, 40}}

	dense := densify(corner, plotStep)
	if len(dense) <= len(corner) {
		t.Fatalf("nothing was inserted: %d points", len(dense))
	}

	// Every original point survives, in order.
	at := 0
	for _, want := range corner {
		found := false
		for ; at < len(dense); at++ {
			if dense[at][0] == want[0] && dense[at][1] == want[1] {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("original point %v is gone from the resampled curve", want)
		}
	}

	// And every inserted point lies on the polyline it came from.
	for _, p := range dense {
		onFirst := p[1] == 0 && p[0] >= 0 && p[0] <= 40
		onSecond := p[0] == 40 && p[1] >= 0 && p[1] <= 40
		if !onFirst && !onSecond {
			t.Errorf("point %v is not on the original path", p)
		}
	}

	if gap := longestGap(dense); gap > plotStep+1e-9 {
		t.Errorf("largest gap is %.2f, want no more than %.2f", gap, plotStep)
	}
}

// A curve blob already sampled finely is left as it is, so a spline is profiled
// against the geometry it recorded rather than a re-interpolation of it.
func TestAFinelySampledCurveIsNotTouched(t *testing.T) {
	var arc [][]float64
	for i := 0; i <= 60; i++ {
		a := float64(i) / 60 * math.Pi / 2
		arc = append(arc, []float64{30 * math.Cos(a), 30 * math.Sin(a)})
	}

	if got := len(densify(arc, plotStep)); got != len(arc) {
		t.Errorf("resampled a curve that was already fine enough: %d points became %d", len(arc), got)
	}
}

func longestGap(curve [][]float64) float64 {
	worst := 0.0
	for i := 0; i+1 < len(curve); i++ {
		if d := math.Hypot(curve[i+1][0]-curve[i][0], curve[i+1][1]-curve[i][1]); d > worst {
			worst = d
		}
	}
	return worst
}

// The 400 positions a run records were counted in the caption and then thrown
// away, so the page drew the plan and called it the run. Everything somebody
// opens this page to see is the difference between the two.
func TestTheRobotsOwnPathIsDrawn(t *testing.T) {
	trace := &Trace{
		Segments: []Segment{{Type: "line", MaxPower: 1, Curve: [][]float64{{0, 0}, {40, 0}}}},
		Samples: []Sample{
			{T: 0, X: 0, Y: 0, V: 0, Segment: 0},
			{T: 100, X: 10, Y: 3, V: 30, Segment: 0},
			{T: 200, X: 25, Y: 4, V: 45, Segment: 0},
			{T: 300, X: 40, Y: 1, V: 5, Segment: 0},
		},
	}
	trace.Profile(DefaultLimits())

	data := trace.buildRenderData(DefaultLimits())

	if len(data.Robot) != 3 {
		t.Fatalf("drew %d strokes for four recorded positions, want 3", len(data.Robot))
	}
	if !data.HasSamples {
		t.Error("the page does not know it has a driven path to show")
	}

	// Consecutive, in the order they were recorded, which is what makes the
	// drawn route a route rather than a scatter of legs.
	for i := 0; i+1 < len(data.Robot); i++ {
		if data.Robot[i].X2 != data.Robot[i+1].X1 || data.Robot[i].Y2 != data.Robot[i+1].Y1 {
			t.Errorf("stroke %d ends where stroke %d does not begin", i, i+1)
		}
	}

	// Coloured by what the robot was really doing, not by the model's opinion
	// of the path, or the page would be drawing its answer onto its evidence.
	if data.Robot[0].Colour == data.Robot[1].Colour {
		t.Error("a leg from a standstill to full speed is drawn in one colour")
	}
	if data.SpeedMax != "45" {
		t.Errorf("the ramp is labelled %s, want the fastest the robot actually went", data.SpeedMax)
	}
}

// A run that overshot went outside the planned area by definition, and sizing
// the view to the plan alone clips off the part worth looking at.
func TestTheViewFitsWhereTheRobotWent(t *testing.T) {
	trace := &Trace{
		Segments: []Segment{{Type: "line", MaxPower: 1, Curve: [][]float64{{0, 0}, {10, 0}}}},
		Samples: []Sample{
			{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 26, Y: -18},
		},
	}

	minX, minY, maxX, maxY := trace.Bounds()
	if maxX < 26 || minY > -18 {
		t.Errorf("bounds %.0f,%.0f..%.0f,%.0f leave the robot's own path off the page",
			minX, minY, maxX, maxY)
	}
}

// A trace with no samples in it still has something to say, and must not claim
// to be showing a driven path it does not have.
func TestATraceWithNoSamplesStillDraws(t *testing.T) {
	trace := &Trace{Segments: []Segment{{Type: "line", MaxPower: 1, Curve: [][]float64{{0, 0}, {40, 0}}}}}
	trace.Profile(DefaultLimits())

	data := trace.buildRenderData(DefaultLimits())

	if data.HasSamples || len(data.Robot) != 0 {
		t.Error("claimed a driven path for a run that recorded none")
	}
	if len(data.Segments) != 1 || len(data.Segments[0].Strokes) == 0 {
		t.Error("nothing was drawn at all")
	}
	if data.SpeedMax == "1" || data.SpeedMax == "0" {
		t.Errorf("the modelled ramp collapsed to %s in/s", data.SpeedMax)
	}
}

// The page has to say which speed the colours mean, because the two answers it
// shows are a measurement and a model and they are not interchangeable.
func TestThePageSaysWhichSpeedItIsShowing(t *testing.T) {
	withSamples := &Trace{
		Segments: []Segment{{Type: "line", MaxPower: 1, Curve: [][]float64{{0, 0}, {40, 0}}}},
		Samples:  []Sample{{X: 0, Y: 0, V: 10}, {X: 40, Y: 0, V: 20}},
	}
	withSamples.Profile(DefaultLimits())

	page := render(t, withSamples)
	if !strings.Contains(page, "measured 0 in/s") {
		t.Error("the ramp does not say it is showing measured speed")
	}
	if !strings.Contains(page, "where the robot actually went") {
		t.Error("the page does not explain what the solid line is")
	}
	if !strings.Contains(page, "stroke-dasharray") {
		t.Error("the planned path is not drawn differently from the driven one")
	}
}
