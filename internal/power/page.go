package power

import (
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// The page is drawn rather than printed because the interesting things about
// current draw are shapes: a motor that spikes once looks nothing like one that
// sits high all match, and a table of peaks cannot tell them apart. Everything
// is inline SVG for the same reason the path visualiser is: this opens off a
// laptop that may be on a robot's Wi-Fi with no route to anywhere, so a page
// that fetches a charting library is a page that renders blank.

// palette is one colour per line, from blob's page.
var palette = []string{
	"#4C9AFF", "#FF5630", "#36B37E", "#FFAB00", "#6554C0",
	"#00B8D9", "#FF8B00", "#8777D9", "#57D9A3", "#FF7452",
}

type point struct{ X, Y float64 }

type line struct {
	Name   string
	Colour string
	Path   string
	Peak   string
	Mean   string
	Charge string
	Share  float64
	Bar    float64
	PeakAt string
	Failed int
}

type axis struct {
	Pos   float64
	Label string
}

type chart struct {
	Title  string
	Unit   string
	Width  float64
	Height float64
	Lines  []line
	XAxis  []axis
	YAxis  []axis
	Empty  bool

	// ViewBox has to cover the margins as well as the plot. Starting it at a
	// negative origin to make room for the axis labels, without also growing
	// the width and height by the same amount, pushes the right-hand end of
	// every line outside the element: the labels fit and the data does not.
	ViewBox string
}

// What the axis labels need around the plot itself.
const (
	marginLeft   = 46.0
	marginRight  = 32.0
	marginTop    = 14.0
	marginBottom = 26.0
)

func viewBox(width, height float64) string {
	return fmt.Sprintf("%.0f %.0f %.0f %.0f",
		-marginLeft, -marginTop,
		width+marginLeft+marginRight,
		height+marginTop+marginBottom)
}

// plot is the data the page draws itself from, rather than a picture of it.
//
// Drawing in the browser is what makes the graph answer questions: zooming into
// a spike, and taking a motor out of the way to see what is behind it, are both
// things somebody wants after seeing the shape once, and neither is possible
// against a path that was drawn before they asked.
type plot struct {
	Names   []string    `json:"names"`
	Colours []string    `json:"colours"`
	Times   []float64   `json:"t"`
	Amps    [][]float64 `json:"a"`
	Volts   []float64   `json:"v"`
	Total   []float64   `json:"total"`
	Kinds   []string    `json:"kinds"`
}

type pageData struct {
	OpMode    string
	Generated string
	Seconds   string
	Period    int

	Problem bool
	Note    string

	WorstName   string
	WorstPeak   string
	Hungriest   string
	TotalPeak   string
	TotalMean   string
	Sag         string
	LowVolts    string
	PeakVolts   string
	HasBattery  bool
	Dropped     int
	SampleCount int

	Current chart
	Battery chart
	Plot    template.JS
	Bars    []line
	Hubs    []line
	Cameras []Camera
	Devices []Device
}

const chartW, chartH = 900.0, 260.0

// Open shows the page in whatever the machine uses for HTML.
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

// Render writes the page and returns where it went.
func (r Report) Render(out string) (string, error) {
	if out == "" {
		name := safeName(r.OpMode)
		if r.Problem {
			name = "problem"
		}
		out = filepath.Join(os.TempDir(), fmt.Sprintf("pusher-power-%s.html", name))
	}

	tmpl, err := template.New("power").Parse(powerPage)
	if err != nil {
		return "", err
	}

	f, err := os.Create(out)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := tmpl.Execute(f, r.page()); err != nil {
		return "", err
	}
	return out, nil
}

func (r Report) page() pageData {
	d := pageData{
		OpMode:      r.OpMode,
		Generated:   time.Now().Format("2 January 2006, 15:04"),
		Seconds:     trimZeros(fmt.Sprintf("%.1f", r.Seconds)),
		Period:      r.Period,
		Problem:     r.Problem,
		Note:        r.Note,
		Dropped:     r.Series.Dropped,
		SampleCount: len(r.Series.Times),
	}

	if r.Problem {
		return d
	}

	motors := r.Motors()

	if len(motors) > 0 {
		d.WorstName = motors[0].Name
		d.WorstPeak = amps(motors[0].Peak)
	}
	d.TotalMean = amps(r.Total())

	if r.PeakVolts > 0 {
		d.HasBattery = true
		d.Sag = fmt.Sprintf("%.2f", r.Sag())
		d.LowVolts = fmt.Sprintf("%.2f", r.LowVolts)
		d.PeakVolts = fmt.Sprintf("%.2f", r.PeakVolts)
	}

	d.Current = r.currentChart(motors)
	d.Battery = r.batteryChart()
	d.Bars = r.bars(motors)
	d.Devices = r.Devices

	d.Cameras = r.Cameras

	for _, h := range r.Hubs() {
		d.Hubs = append(d.Hubs, line{Name: h.Name, Peak: amps(h.Peak), Mean: amps(h.Mean)})
	}

	if combined := r.Series.Combined(motors); len(combined) > 0 {
		d.TotalPeak = amps(maxOf(combined))
	}

	d.Plot = r.plotJSON(motors)

	if len(d.Bars) > 0 {
		hungriest := d.Bars[0]
		for _, b := range d.Bars {
			if b.Share > hungriest.Share {
				hungriest = b
			}
		}
		if hungriest.Charge != "" {
			d.Hungriest = hungriest.Name
		}
	}

	return d
}

// plotJSON is every reading, for the browser to draw.
func (r Report) plotJSON(motors []Device) template.JS {
	if !r.Series.Any() {
		return ""
	}

	p := plot{
		Times: round(r.Series.Times, 2),
		Volts: round(r.Series.Volts, 2),
		Total: round(r.Series.Combined(motors), 2),
	}

	// Motors first and in the palette order the table uses, then the hubs and
	// buses, so a colour means the same thing everywhere on the page.
	for _, d := range append(append([]Device{}, motors...), r.Hubs()...) {
		series := r.Series.Of(d.Name)
		if series == nil {
			continue
		}

		p.Names = append(p.Names, d.Name)
		p.Kinds = append(p.Kinds, d.Kind)
		p.Colours = append(p.Colours, palette[len(p.Names)%len(palette)])
		p.Amps = append(p.Amps, round(series, 2))
	}

	blob, err := json.Marshal(p)
	if err != nil {
		return ""
	}
	return template.JS(blob)
}

// round keeps the page small. Two decimals is finer than the hardware reports
// and far finer than a few hundred pixels can show.
func round(values []float64, places int) []float64 {
	scale := math.Pow(10, float64(places))

	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = math.Round(v*scale) / scale
	}
	return out
}

// currentChart is every motor's draw against time, plus their sum.
func (r Report) currentChart(motors []Device) chart {
	c := chart{Title: "Current over time", Unit: "A", Width: chartW, Height: chartH,
		ViewBox: viewBox(chartW, chartH)}

	if !r.Series.Any() || len(motors) == 0 {
		c.Empty = true
		return c
	}

	times := r.Series.Times
	span := times[len(times)-1]
	if span <= 0 {
		c.Empty = true
		return c
	}

	combined := r.Series.Combined(motors)
	top := maxOf(combined)
	for _, m := range motors {
		if peak := maxOf(r.Series.Of(m.Name)); peak > top {
			top = peak
		}
	}
	if top <= 0 {
		c.Empty = true
		return c
	}
	top = niceCeil(top)

	// The sum first and in grey, so it reads as the backdrop the individual
	// motors are measured against rather than as another motor.
	c.Lines = append(c.Lines, line{
		Name:   "all motors",
		Colour: "#8894a3",
		Path:   pathFor(times, combined, span, top),
	})

	for i, m := range motors {
		c.Lines = append(c.Lines, line{
			Name:   m.Name,
			Colour: palette[i%len(palette)],
			Path:   pathFor(times, r.Series.Of(m.Name), span, top),
			Peak:   amps(m.Peak),
		})
	}

	c.XAxis = timeAxis(span)
	c.YAxis = valueAxis(top, "A")
	return c
}

func (r Report) batteryChart() chart {
	c := chart{Title: "Battery", Unit: "V", Width: chartW, Height: 160,
		ViewBox: viewBox(chartW, 160)}

	volts := r.Series.Volts
	if !r.Series.Any() || len(volts) == 0 {
		c.Empty = true
		return c
	}

	low, high := math.MaxFloat64, 0.0
	for _, v := range volts {
		if v <= 0 {
			continue
		}
		if v < low {
			low = v
		}
		if v > high {
			high = v
		}
	}
	if high <= 0 || low == math.MaxFloat64 {
		c.Empty = true
		return c
	}

	// Zoomed to the range the battery actually moved through, with a little
	// air. A voltage axis from zero would draw every run as a flat line.
	floor := math.Floor((low-0.3)*2) / 2
	ceil := math.Ceil((high+0.3)*2) / 2

	times := r.Series.Times
	span := times[len(times)-1]

	c.Lines = []line{{
		Name:   "battery",
		Colour: "#FFAB00",
		Path:   pathBetween(times, volts, span, floor, ceil, c.Height),
	}}
	c.XAxis = timeAxis(span)
	c.YAxis = rangeAxis(floor, ceil, "V", c.Height)
	return c
}

// bars ranks the motors by how much charge they took out of the battery.
func (r Report) bars(motors []Device) []line {
	var out []line

	widest := 0.0
	for _, m := range motors {
		if m.Peak > widest {
			widest = m.Peak
		}
	}
	if widest <= 0 {
		widest = 1
	}

	totalCharge := 0.0
	for _, m := range motors {
		totalCharge += r.Series.Charge(m.Name)
	}

	for i, m := range motors {
		b := line{
			Name:   m.Name,
			Colour: palette[i%len(palette)],
			Peak:   amps(m.Peak),
			Mean:   amps(m.Mean),
			PeakAt: trimZeros(fmt.Sprintf("%.1f", m.PeakAt)),
			Bar:    m.Peak / widest * 100,
			Failed: m.Failures,
		}

		if charge := r.Series.Charge(m.Name); charge > 0 {
			b.Charge = trimZeros(fmt.Sprintf("%.1f", charge))
			if totalCharge > 0 {
				b.Share = charge / totalCharge * 100
			}
		}

		out = append(out, b)
	}

	sort.SliceStable(out, func(a, b int) bool { return out[a].Share > out[b].Share })
	return out
}

func pathFor(times, values []float64, span, top float64) string {
	return pathBetween(times, values, span, 0, top, chartH)
}

func pathBetween(times, values []float64, span, floor, ceil, height float64) string {
	if len(values) == 0 || ceil <= floor {
		return ""
	}

	var b strings.Builder
	for i := 0; i < len(values) && i < len(times); i++ {
		x := times[i] / span * chartW
		y := height - (values[i]-floor)/(ceil-floor)*height

		if y < 0 {
			y = 0
		}
		if y > height {
			y = height
		}

		if i == 0 {
			fmt.Fprintf(&b, "M%.1f %.1f", x, y)
			continue
		}
		fmt.Fprintf(&b, " L%.1f %.1f", x, y)
	}
	return b.String()
}

func timeAxis(span float64) []axis {
	var out []axis
	for i := 0; i <= 4; i++ {
		at := span * float64(i) / 4
		out = append(out, axis{
			Pos:   at / span * chartW,
			Label: trimZeros(fmt.Sprintf("%.1f", at)) + "s",
		})
	}
	return out
}

func valueAxis(top float64, unit string) []axis {
	return rangeAxis(0, top, unit, chartH)
}

func rangeAxis(floor, ceil float64, unit string, height float64) []axis {
	var out []axis
	for i := 0; i <= 4; i++ {
		v := floor + (ceil-floor)*float64(i)/4
		out = append(out, axis{
			Pos:   height - (v-floor)/(ceil-floor)*height,
			Label: trimZeros(fmt.Sprintf("%.1f", v)) + unit,
		})
	}
	return out
}

func maxOf(values []float64) float64 {
	top := 0.0
	for _, v := range values {
		if v > top {
			top = v
		}
	}
	return top
}

// niceCeil rounds an axis up to something a person would have chosen.
func niceCeil(v float64) float64 {
	if v <= 0 {
		return 1
	}

	step := math.Pow(10, math.Floor(math.Log10(v))) / 2
	if step <= 0 {
		return v
	}
	return math.Ceil(v/step) * step
}

func amps(v float64) string { return trimZeros(fmt.Sprintf("%.2f", v)) }

func trimZeros(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

func safeName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	if b.Len() == 0 {
		return "run"
	}
	return b.String()
}
