package pathtrace

import (
	"fmt"
	"html/template"
	"math"
	"os"
)

const viewSize = 1000.0

type renderSegment struct {
	Index     int
	Label     string
	Source    string
	Type      string
	MaxPower  string
	Length    string
	Est       string
	Actual    string
	Peak      string
	Slow      bool
	Strokes   []stroke
	Markers   []marker
	TargetX   float64
	TargetY   float64
	HasTarget bool
}

type stroke struct {
	X1, Y1, X2, Y2 float64
	Colour         string
}

type marker struct {
	X, Y  float64
	Kind  string
	Title string
}

type renderData struct {
	OpMode      string
	Generated   string
	Segments    []renderSegment
	EstTotal    string
	ActualTotal string
	Delta       string
	SlowestName string
	SlowestTime string
	TotalLength string
	SpeedMax    string
	LegendStops []legendStop
	ViewSize    float64
	GridLines   []gridLine
	Robot       []stroke
	HasSamples  bool
}

type legendStop struct {
	Offset string
	Colour string
	Label  string
}

type gridLine struct {
	X1, Y1, X2, Y2 float64
	Major          bool
}

// Render writes the trace as a standalone HTML page.
func (t *Trace) Render(path string, lim Limits) error {
	data := t.buildRenderData(lim)

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot write %s: %w", path, err)
	}
	defer f.Close()

	tmpl, err := template.New("vis").Parse(pageTemplate)
	if err != nil {
		return fmt.Errorf("bad template: %w", err)
	}
	return tmpl.Execute(f, data)
}

func (t *Trace) buildRenderData(lim Limits) renderData {
	minX, minY, maxX, maxY := t.Bounds()
	_, vMax := t.SpeedRange()

	spanX, spanY := maxX-minX, maxY-minY
	span := math.Max(spanX, spanY)
	if span <= 0 {
		span = 1
	}
	scale := viewSize / span
	offX := (viewSize - spanX*scale) / 2
	offY := (viewSize - spanY*scale) / 2

	tx := func(x float64) float64 { return offX + (x-minX)*scale }
	ty := func(y float64) float64 { return viewSize - (offY + (y-minY)*scale) }

	// The driven path is what the robot did, and until now it was counted in
	// the caption and then thrown away: 400 recorded positions summarised as
	// the number 400. Drawn from the samples in the order they were taken, so
	// it is a route through time rather than a plan, which is also what makes
	// it connected. The planned geometry stays underneath it, because the
	// question this page answers is what the difference between them was.
	var robot []stroke
	_, measured := t.MeasuredRange()

	for i := 0; i+1 < len(t.Samples); i++ {
		a, b := t.Samples[i], t.Samples[i+1]
		robot = append(robot, stroke{
			X1: tx(a.X), Y1: ty(a.Y),
			X2: tx(b.X), Y2: ty(b.Y),
			Colour: heatColour((a.V + b.V) / 2 / measured),
		})
	}

	est, actual := t.Totals()
	totalLen := 0.0

	slowestName, slowestTime := "", 0.0

	var segs []renderSegment
	for i := range t.Segments {
		s := &t.Segments[i]
		totalLen += s.Length

		act := s.ActualSeconds(t.DurationMs)
		if act > slowestTime {
			slowestTime, slowestName = act, s.Label
		}

		rs := renderSegment{
			Index:    s.Index + 1,
			Label:    s.Label,
			Source:   s.Source,
			Type:     s.Type,
			MaxPower: fmt.Sprintf("%.2f", s.MaxPower),
			Length:   fmt.Sprintf("%.1f", s.Length),
			Est:      fmt.Sprintf("%.2f", s.EstSeconds),
			Actual:   fmt.Sprintf("%.2f", act),
			Peak:     fmt.Sprintf("%.1f", s.PeakSpeed),
			Slow:     s.PeakSpeed < 0.5*vMax,
		}

		for j := 0; j+1 < len(s.Curve); j++ {
			v := 0.0
			if j < len(s.Speeds) {
				v = (s.Speeds[j] + s.Speeds[j+1]) / 2
			}
			rs.Strokes = append(rs.Strokes, stroke{
				X1: tx(s.Curve[j][0]), Y1: ty(s.Curve[j][1]),
				X2: tx(s.Curve[j+1][0]), Y2: ty(s.Curve[j+1][1]),
				Colour: heatColour(v / vMax),
			})
		}

		if s.Intercept != nil {
			rs.Markers = append(rs.Markers, marker{
				X: tx(s.Intercept.X), Y: ty(s.Intercept.Y), Kind: "intercept",
				Title: fmt.Sprintf("intercept (%.1f, %.1f)", s.Intercept.X, s.Intercept.Y),
			})
		}
		for _, w := range s.Waypoints {
			rs.Markers = append(rs.Markers, marker{
				X: tx(w[0]), Y: ty(w[1]), Kind: "waypoint",
				Title: fmt.Sprintf("waypoint (%.1f, %.1f)", w[0], w[1]),
			})
		}
		rs.TargetX, rs.TargetY = tx(s.Target.X), ty(s.Target.Y)
		rs.HasTarget = true

		segs = append(segs, rs)
	}

	// The ramp is labelled with whatever the drawn path is coloured by, so the
	// number under it is the one somebody can check against.
	ramp := vMax
	if len(robot) > 0 {
		ramp = measured
	}

	var stops []legendStop
	for i := 0; i <= 4; i++ {
		f := float64(i) / 4
		stops = append(stops, legendStop{
			Offset: fmt.Sprintf("%.0f%%", f*100),
			Colour: heatColour(f),
			Label:  fmt.Sprintf("%.0f", f*ramp),
		})
	}

	delta := "n/a"
	if actual > 0 {
		delta = fmt.Sprintf("%+.2f s (%.0f%%)", est-actual, (est/actual-1)*100)
	}

	return renderData{
		OpMode:      t.OpMode,
		Generated:   fmt.Sprintf("%d segments, %d motion samples", len(t.Segments), len(t.Samples)),
		Segments:    segs,
		EstTotal:    fmt.Sprintf("%.2f", est),
		ActualTotal: fmt.Sprintf("%.2f", actual),
		Delta:       delta,
		SlowestName: slowestName,
		SlowestTime: fmt.Sprintf("%.2f", slowestTime),
		TotalLength: fmt.Sprintf("%.0f", totalLen),
		SpeedMax:    fmt.Sprintf("%.0f", ramp),
		Robot:       robot,
		LegendStops: stops,
		ViewSize:    viewSize,
		GridLines:   gridFor(minX, minY, maxX, maxY, tx, ty),
		HasSamples:  len(robot) > 0,
	}
}

func gridFor(minX, minY, maxX, maxY float64, tx, ty func(float64) float64) []gridLine {
	var lines []gridLine
	start := math.Floor(minX/24) * 24
	for x := start; x <= maxX; x += 24 {
		lines = append(lines, gridLine{X1: tx(x), Y1: ty(minY), X2: tx(x), Y2: ty(maxY), Major: math.Abs(x) < 1e-9})
	}
	start = math.Floor(minY/24) * 24
	for y := start; y <= maxY; y += 24 {
		lines = append(lines, gridLine{X1: tx(minX), Y1: ty(y), X2: tx(maxX), Y2: ty(y), Major: math.Abs(y) < 1e-9})
	}
	return lines
}

func heatColour(f float64) string {
	f = math.Max(0, math.Min(1, f))

	stops := [][3]float64{
		{56, 108, 255},
		{0, 190, 210},
		{54, 179, 126},
		{255, 190, 60},
		{255, 86, 48},
	}

	x := f * float64(len(stops)-1)
	i := int(x)
	if i >= len(stops)-1 {
		i = len(stops) - 2
	}
	frac := x - float64(i)

	r := stops[i][0] + (stops[i+1][0]-stops[i][0])*frac
	g := stops[i][1] + (stops[i+1][1]-stops[i][1])*frac
	b := stops[i][2] + (stops[i+1][2]-stops[i][2])*frac

	return fmt.Sprintf("#%02x%02x%02x", int(r), int(g), int(b))
}
