package follower

import (
	"fmt"
	"html/template"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// The page is drawn in the browser rather than written out as a picture, for
// the same reason the profiler's is: the first thing anybody does with a plot
// of a loop that is ringing is zoom into the ringing. A run is five minutes of
// sixty hertz, so the whole thing at once is a smear, and a picture of the
// smear is a picture of the question rather than of the answer.
//
// Everything is inline. This opens on a laptop that may be sitting on a robot's
// access point with no route anywhere, and a page that fetches a charting
// library from a CDN renders blank.

// Open shows a rendered page in the browser.
func Open(path string) {
	var c *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", path)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		c = exec.Command("xdg-open", path)
	}

	c.Start()
}

type card struct {
	Key   string
	Value string
	Unit  string
	Sub   string
	Warn  bool
}

type pageData struct {
	Title   string
	Sub     string
	Cards   []card
	Notices []string
	Extra   string
	Plot    template.JS
}

// Render writes the log as a standalone HTML page and says where it went.
func (l *Log) Render(out string) (string, error) {
	if out == "" {
		name := strings.TrimSuffix(l.Name, ".csv")
		if name == "" {
			name = "run"
		}
		out = filepath.Join(os.TempDir(), "pusher-"+name+".html")
	}

	tmpl, err := template.New("follower").Parse(followerPage)
	if err != nil {
		return "", fmt.Errorf("bad template: %w", err)
	}

	f, err := os.Create(out)
	if err != nil {
		return "", fmt.Errorf("cannot write %s: %w", out, err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, l.page()); err != nil {
		return "", err
	}
	return out, nil
}

func (l *Log) page() pageData {
	s := l.Summarise()

	title := "Follower log"
	if !l.When.IsZero() {
		title += " · " + l.When.Format("2 Jan 15:04:05")
	}

	d := pageData{
		Title: title,
		Sub: fmt.Sprintf("%s loops over %.1f s, format v%d, %s",
			thousands(s.Loops), s.Duration, l.Version, l.Name),
		Plot: template.JS(l.plot()),
	}

	// The turn axis losing volts to desaturation is not a tuning problem and
	// not a mechanical one, so it goes above everything rather than into a
	// column somebody has to notice.
	if s.WorstTurnLoss > 0 {
		d.Notices = append(d.Notices, fmt.Sprintf(
			"The turn command lost up to %.0f%% of itself to desaturation. blob keeps the turn "+
				"and scales translation down while it is following a path, so this means the "+
				"follower was not in path following mode on those loops.", 100*s.WorstTurnLoss))
	}
	if s.SlowLoop {
		d.Notices = append(d.Notices, fmt.Sprintf(
			"The worst loop took %.0f ms, and blob's autotune derives tauClosedLoop assuming %.0f ms. "+
				"A pole placed faster than the loop can support rings, and the ringing looks "+
				"exactly like badly chosen constants. Nothing below %.3f s is usable at this rate.",
			1000*s.MaxDt, 1000*autotuneDt, s.MinTau))
	}
	if s.Capped {
		d.Notices = append(d.Notices, fmt.Sprintf(
			"The recorder stops at %s rows and nothing in the file says so, so this run is "+
				"cut off at that point rather than finished.", thousands(MaxRows)))
	}
	if s.Partial {
		d.Notices = append(d.Notices, "The last row was half written when the file was pulled, "+
			"so it was dropped. The recorder rewrites the whole file every couple of seconds, "+
			"which means this log is a run that may still be going.")
	}
	if len(l.Extra) > 0 {
		d.Extra = "This log carries columns this version of pusher does not know about (" +
			strings.Join(l.Extra, ", ") + "). They are not drawn. A newer pusher probably reads them."
	}

	d.Cards = []card{
		{
			Key: "Loops", Value: thousands(s.Loops),
			Sub: fmt.Sprintf("%.1f s recorded", s.Duration),
		},
		{
			Key: "Loop rate", Value: fmt.Sprintf("%.0f", s.Rate), Unit: "Hz",
			Sub: fmt.Sprintf("mean %.1f ms, max %.1f ms", 1000*s.MeanDt, 1000*s.MaxDt),
		},
		{
			Key: "Fastest usable tau", Value: fmt.Sprintf("%.3f", s.MinTau), Unit: "s",
			Sub: "4 x the worst loop period", Warn: s.SlowLoop,
		},
		{
			Key: "Mix saturated", Value: fmt.Sprintf("%.1f", s.SaturatedPct), Unit: "%",
			Sub:  fmt.Sprintf("%s of %s loops", thousands(s.SaturatedLoops), thousands(s.Loops)),
			Warn: s.SaturatedPct > 20,
		},
		{
			Key: "Turn lost to mix", Value: fmt.Sprintf("%.0f", 100*s.WorstTurnLoss), Unit: "%",
			Sub: "worst single loop", Warn: s.WorstTurnLoss > 0,
		},
	}

	for _, gap := range s.Gaps {
		d.Cards = append(d.Cards, card{
			Key:   gap.Axis + " gap",
			Value: fmt.Sprintf("%.2f", gap.Worst), Unit: gap.Unit,
			Sub: fmt.Sprintf("worst, %.2f RMS while commanding", gap.RMS),
		})
	}

	d.Cards = append(d.Cards,
		card{
			Key: "Heading error", Value: fmt.Sprintf("%.1f", s.WorstHeadingDeg), Unit: "deg",
			Sub: fmt.Sprintf("worst, %d sign changes", s.Reversals),
		},
		card{
			Key: "Settled", Value: fmt.Sprintf("%.0f", s.SettledPct), Unit: "%",
			Sub: "loops commanding nothing",
		},
		card{
			Key: "Speed", Value: fmt.Sprintf("%.0f", s.PeakSpeed), Unit: "in/s",
			Sub: fmt.Sprintf("peak reached, %.0f asked for", s.PeakSpeedRef),
		},
	)

	return d
}

// plot builds the payload the page draws from.
//
// Written by hand rather than marshalled, because the default float formatting
// is the difference between a three megabyte page and a ten megabyte one:
// twenty thousand loops of full precision doubles is mostly digits nobody can
// see at one pixel per loop.
func (l *Log) plot() string {
	n := len(l.Rows)

	get := func(f func(Row) float64) []float64 {
		out := make([]float64, n)
		for i, r := range l.Rows {
			out[i] = f(r)
		}
		return out
	}

	var b strings.Builder
	b.WriteString("{")

	// Time in seconds from the first row, which is what every axis is drawn
	// against.
	start := l.Rows[0].TimeMs
	series(&b, "t", get(func(r Row) float64 { return (r.TimeMs - start) / 1000 }), 4)

	series(&b, "dt", get(func(r Row) float64 { return r.Dt * 1000 }), 3)

	series(&b, "vf", get(func(r Row) float64 { return r.VF }), 3)
	series(&b, "ref_f", get(func(r Row) float64 { return r.RefF }), 3)
	series(&b, "vl", get(func(r Row) float64 { return r.VL }), 3)
	series(&b, "ref_l", get(func(r Row) float64 { return r.RefL }), 3)
	series(&b, "omega", get(func(r Row) float64 { return r.Omega }), 4)
	series(&b, "ref_omega", get(func(r Row) float64 { return r.RefOmega }), 4)

	series(&b, "volts_f", get(func(r Row) float64 { return r.VoltsF }), 3)
	series(&b, "volts_l", get(func(r Row) float64 { return r.VoltsL }), 3)
	series(&b, "volts_t", get(func(r Row) float64 { return r.VoltsT }), 3)
	series(&b, "static_f", get(func(r Row) float64 { return r.StaticF }), 3)
	series(&b, "static_l", get(func(r Row) float64 { return r.StaticL }), 3)
	series(&b, "static_t", get(func(r Row) float64 { return r.StaticT }), 3)

	// The delivered twist is computed here rather than in the page. It is the
	// one derived quantity in the file whose signs are easy to get wrong, and
	// two copies of that arithmetic is one copy too many.
	series(&b, "deliv_t", get(func(r Row) float64 {
		_, _, t := r.Delivered()
		return t
	}), 3)

	series(&b, "lf", get(func(r Row) float64 { return r.LF }), 4)
	series(&b, "lb", get(func(r Row) float64 { return r.LB }), 4)
	series(&b, "rf", get(func(r Row) float64 { return r.RF }), 4)
	series(&b, "rb", get(func(r Row) float64 { return r.RB }), 4)

	series(&b, "battery", get(func(r Row) float64 { return r.Battery }), 3)

	// Drawn in the field's convention, CCW positive, rather than the file's
	// mirrored one. Everything else on the page is CCW positive, and a single
	// plot in the opposite sense is how somebody concludes the robot is turning
	// the wrong way when it is not.
	series(&b, "herr", get(func(r Row) float64 { return degrees(r.FieldHeadingError()) }), 3)

	spans(&b, "sat", l.spansWhere(func(r Row) bool { return r.Saturated() }))
	spans(&b, "settled", l.spansWhere(func(r Row) bool { return r.Settled }))

	edges, counts := l.histogram()
	series(&b, "histEdges", edges, 3)

	b.WriteString("\"histCounts\":[")
	for i, c := range counts {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(strconv.Itoa(c))
	}
	b.WriteString("],")

	s := l.Summarise()
	fmt.Fprintf(&b, "\"meanDt\":%.3f,\"maxDt\":%.3f,\"minTau\":%.4f",
		1000*s.MeanDt, 1000*s.MaxDt, s.MinTau)

	b.WriteString("}")
	return b.String()
}

func series(b *strings.Builder, name string, values []float64, decimals int) {
	fmt.Fprintf(b, "%q:[", name)

	for i, v := range values {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(round(v, decimals))
	}

	b.WriteString("],")
}

// round is one number as short as it can be and still mean the same thing at
// one pixel per loop.
func round(v float64, decimals int) string {
	// A NaN or an infinity in the middle of an array is a page that does not
	// parse at all, so a bad number is dropped to zero rather than allowed to
	// take the whole log with it.
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "0"
	}

	text := strconv.FormatFloat(v, 'f', decimals, 64)
	if strings.Contains(text, ".") {
		text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	}
	if text == "" || text == "-0" {
		return "0"
	}
	return text
}

func spans(b *strings.Builder, name string, ranges [][2]int) {
	fmt.Fprintf(b, "%q:[", name)

	for i, r := range ranges {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(b, "[%d,%d]", r[0], r[1])
	}

	b.WriteString("],")
}

// spansWhere turns a per loop flag into the runs of loops it was true for,
// because a band drawn once is cheaper and far more legible than one drawn per
// loop at four hundred loops to the pixel.
func (l *Log) spansWhere(is func(Row) bool) [][2]int {
	var out [][2]int
	start := -1

	for i, r := range l.Rows {
		if is(r) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			out = append(out, [2]int{start, i - 1})
			start = -1
		}
	}
	if start >= 0 {
		out = append(out, [2]int{start, len(l.Rows) - 1})
	}
	return out
}

// histogram buckets the loop period, in milliseconds.
//
// The shape is the point rather than the mean: a loop that is 17 ms except when
// it is 40 has a second hump, and a mean of 19 hides it completely.
func (l *Log) histogram() ([]float64, []int) {
	const buckets = 48

	lo, hi := math.Inf(1), 0.0
	for _, r := range l.Rows {
		ms := r.Dt * 1000
		lo, hi = math.Min(lo, ms), math.Max(hi, ms)
	}
	if !(hi > lo) {
		hi = lo + 1
	}

	width := (hi - lo) / buckets
	edges := make([]float64, buckets+1)
	for i := range edges {
		edges[i] = lo + width*float64(i)
	}

	counts := make([]int, buckets)
	for _, r := range l.Rows {
		i := int((r.Dt*1000 - lo) / width)
		if i >= buckets {
			i = buckets - 1
		}
		if i < 0 {
			i = 0
		}
		counts[i]++
	}

	return edges, counts
}

func thousands(n int) string {
	text := strconv.Itoa(n)
	if len(text) <= 3 {
		return text
	}

	var b strings.Builder
	for i, digit := range text {
		if i > 0 && (len(text)-i)%3 == 0 {
			b.WriteString(",")
		}
		b.WriteRune(digit)
	}
	return b.String()
}
