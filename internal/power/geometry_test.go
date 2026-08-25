package power

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// Everything drawn has to be inside the viewBox, or it spills outside the
// element and lands on whatever is next to it. This was wrong the first time:
// the box was moved to make room for the axis labels but never grown, so the
// right-hand end of every line and the bottom row of labels fell outside it.
func TestNothingIsDrawnOutsideTheViewBox(t *testing.T) {
	report := sampleReport(t)

	for _, c := range []chart{report.currentChart(report.Motors()), report.batteryChart()} {
		if c.Empty {
			t.Fatalf("%s: nothing to check", c.Title)
		}

		minX, minY, w, h := box(t, c.ViewBox)
		maxX, maxY := minX+w, minY+h

		// The plot itself.
		if 0 < minX || c.Width > maxX {
			t.Errorf("%s: the plot spans 0..%.0f across a box of %.0f..%.0f", c.Title, c.Width, minX, maxX)
		}
		if 0 < minY || c.Height > maxY {
			t.Errorf("%s: the plot spans 0..%.0f down a box of %.0f..%.0f", c.Title, c.Height, minY, maxY)
		}

		// The labels, which sit outside the plot on purpose.
		for _, a := range c.YAxis {
			// Drawn at x=-8, anchored end, so it runs left by its own width.
			if -8-textWidth(a.Label) < minX {
				t.Errorf("%s: y label %q starts left of the box", c.Title, a.Label)
			}
			if a.Pos < minY || a.Pos > maxY {
				t.Errorf("%s: y label %q sits at %.0f, outside %.0f..%.0f", c.Title, a.Label, a.Pos, minY, maxY)
			}
		}

		for _, a := range c.XAxis {
			half := textWidth(a.Label) / 2
			if a.Pos-half < minX || a.Pos+half > maxX {
				t.Errorf("%s: x label %q is centred at %.0f and runs outside %.0f..%.0f",
					c.Title, a.Label, a.Pos, minX, maxX)
			}
			// Drawn at the plot's bottom edge with dy=16.
			if c.Height+16 > maxY {
				t.Errorf("%s: x labels are drawn at %.0f, below the box bottom %.0f", c.Title, c.Height+16, maxY)
			}
		}

		// And every point of every line.
		for _, l := range c.Lines {
			for _, p := range points(t, l.Path) {
				if p.X < minX || p.X > maxX || p.Y < minY || p.Y > maxY {
					t.Errorf("%s: %s reaches (%.0f, %.0f), outside the box", c.Title, l.Name, p.X, p.Y)
					break
				}
			}
		}
	}
}

// textWidth is a generous guess at how wide a label renders at 11px.
func textWidth(label string) float64 { return float64(len(label)) * 7.5 }

func box(t *testing.T, viewBox string) (x, y, w, h float64) {
	t.Helper()

	fields := strings.Fields(viewBox)
	if len(fields) != 4 {
		t.Fatalf("viewBox %q is not four numbers", viewBox)
	}

	out := make([]float64, 4)
	for i, f := range fields {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			t.Fatalf("viewBox %q: %v", viewBox, err)
		}
		out[i] = v
	}
	return out[0], out[1], out[2], out[3]
}

func points(t *testing.T, path string) []point {
	t.Helper()

	var out []point
	for _, chunk := range strings.Fields(strings.NewReplacer("M", " ", "L", " ").Replace(path)) {
		v, err := strconv.ParseFloat(chunk, 64)
		if err != nil {
			continue
		}
		out = append(out, point{X: v})
	}

	// The fields alternate x, y.
	var pairs []point
	for i := 0; i+1 < len(out); i += 2 {
		pairs = append(pairs, point{X: out[i].X, Y: out[i+1].X})
	}
	return pairs
}

// sampleReport is a run with a series in it, shaped like a real one.
func sampleReport(t *testing.T) Report {
	t.Helper()

	var b strings.Builder
	b.WriteString("pusher-power 1\nopmode TeleOP\nseconds 60.000\nperiod 100\nvolts 10.80 13.20\n")
	b.WriteString("device shooter motor 600 6.0000 22.0000 30.000 0\n")
	b.WriteString("device drive motor 600 4.0000 11.0000 12.000 0\n")
	b.WriteString("device Control_Hub hub 600 12.0000 30.0000 30.000 0\n")
	b.WriteString("series shooter drive Control_Hub\n")

	for i := 0; i <= 600; i++ {
		at := float64(i) / 10
		fmt.Fprintf(&b, "s %.2f %.2f %.2f %.2f %.2f\n",
			at, 13.2-float64(i%40)*0.06, float64(i%22), float64(i%11), float64(i%30))
	}

	report, err := Parse(b.String())
	if err != nil {
		t.Fatal(err)
	}
	return report
}
