package power

import (
	"bufio"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Device is what one motor or hub drew over a run.
type Device struct {
	Name string
	Kind string

	Samples int
	Mean    float64
	Peak    float64
	PeakAt  float64

	// Failures counts readings the device refused. A device that answered
	// sometimes is still worth reporting, with the caveat attached.
	Failures int
}

// Report is one recorded run.
type Report struct {
	OpMode  string
	Seconds float64

	// Period is how often the robot sampled, in milliseconds, which is the
	// resolution of everything here.
	Period int

	LowVolts  float64
	PeakVolts float64

	// Problem and Note carry a note the monitor left instead of a recording.
	Problem bool
	Note    string

	// Cameras are what the monitor could learn about anything that draws power
	// without being on a rail the hub measures. See Camera.
	Cameras []Camera

	// Series is every reading, for drawing. Empty on a recording written before
	// the monitor kept them, which is why nothing here depends on it.
	Series Series

	Devices []Device
}

// Parse reads a recording written by the monitor.
//
// Unknown lines are skipped rather than refused: the robot may be running a
// newer monitor than this pusher, and a field nobody here understands is not a
// reason to throw away the ones it does.
func Parse(content string) (Report, error) {
	var out Report

	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var note strings.Builder

	header := false

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}

		switch fields[0] {
		case "pusher-power":
			header = true

		// A note the monitor left about why it recorded nothing. Read rather
		// than refused: it is the answer to the question somebody is asking.
		case "pusher-power-problem":
			header = true
			out.Problem = true

		case "opmode":
			if len(fields) > 1 {
				out.OpMode = fields[1]
			}

		case "seconds":
			if len(fields) > 1 {
				out.Seconds, _ = strconv.ParseFloat(fields[1], 64)
			}

		case "period":
			if len(fields) > 1 {
				out.Period, _ = strconv.Atoi(fields[1])
			}

		case "volts":
			if len(fields) > 2 {
				out.LowVolts, _ = strconv.ParseFloat(fields[1], 64)
				out.PeakVolts, _ = strconv.ParseFloat(fields[2], 64)
			}

		case "device":
			if device, ok := parseDevice(fields); ok {
				out.Devices = append(out.Devices, device)
			}

		case "limelight":
			if len(fields) > 4 {
				connected, _ := strconv.ParseFloat(fields[2], 64)
				fps, _ := strconv.ParseFloat(fields[3], 64)
				cpu, _ := strconv.ParseFloat(fields[4], 64)
				out.Cameras = append(out.Cameras, Camera{
					Name: fields[1], Connected: connected, FPS: fps, CPU: cpu,
				})
			}

		case "series":
			out.Series.Names = fields[1:]
			out.Series.Amps = make([][]float64, len(out.Series.Names))

		case "truncated":
			if len(fields) > 2 {
				kept, _ := strconv.Atoi(fields[1])
				total, _ := strconv.Atoi(fields[2])
				out.Series.Dropped = total - kept
			}

		case "s":
			readSample(&out.Series, fields)

		default:
			// Everything after a problem header is the explanation.
			if out.Problem {
				note.WriteString(strings.Join(fields, " ") + " ")
			}
		}
	}

	if !header {
		return out, fmt.Errorf("this is not a pusher power recording")
	}

	if out.Problem {
		out.Note = strings.TrimSpace(note.String())
		if out.Note == "" {
			out.Note = "The monitor left a note with nothing in it."
		}
		return out, nil
	}

	if len(out.Devices) == 0 {
		return out, fmt.Errorf("the recording has no readings in it")
	}

	// Biggest draw first, which is the question somebody opened this to answer.
	sort.SliceStable(out.Devices, func(a, b int) bool {
		return out.Devices[a].Peak > out.Devices[b].Peak
	})

	return out, nil
}

// Camera is a Limelight, and is deliberately not a current reading.
//
// It is not on a rail the hub can measure, so whatever it draws is inside the
// hub's total and nowhere else. What it can say is how hard it is working,
// which is the closest thing to a power figure available for it.
type Camera struct {
	Name string

	// Connected is the percentage of the run it answered for.
	Connected float64
	FPS       float64
	CPU       float64
}

// Series is every reading, lined up on one clock.
type Series struct {
	Names []string
	Times []float64
	Volts []float64

	// Amps is one row per device, in the order Names gives them.
	Amps [][]float64

	// Dropped is how many ticks were taken but not kept, when a run went on
	// longer than the monitor holds.
	Dropped int
}

// Any reports whether there is anything to draw.
func (s Series) Any() bool { return len(s.Times) > 1 && len(s.Amps) > 0 }

// Of is one device's readings, by name.
func (s Series) Of(name string) []float64 {
	for i, n := range s.Names {
		if n == name && i < len(s.Amps) {
			return s.Amps[i]
		}
	}
	return nil
}

// Charge is how much charge a device drew over the run, in amp-seconds.
//
// The integral rather than the peak, which is a different question and often a
// different answer: a motor that pulls 20A for a tenth of a second costs the
// battery far less than one sitting at 4A all match.
func (s Series) Charge(name string) float64 {
	amps := s.Of(name)
	if len(amps) < 2 || len(s.Times) < 2 {
		return 0
	}

	var total float64
	for i := 1; i < len(amps) && i < len(s.Times); i++ {
		dt := s.Times[i] - s.Times[i-1]
		if dt > 0 && dt < 5 {
			total += (amps[i] + amps[i-1]) / 2 * dt
		}
	}
	return total
}

// Combined is every motor's draw added up at each tick, which is the load the
// battery actually saw.
func (s Series) Combined(motors []Device) []float64 {
	if !s.Any() {
		return nil
	}

	out := make([]float64, len(s.Times))
	for _, d := range motors {
		amps := s.Of(d.Name)
		for i := range out {
			if i < len(amps) {
				out[i] += amps[i]
			}
		}
	}
	return out
}

// device <name> <kind> <samples> <mean> <peak> <peakAt> <failures>
func parseDevice(fields []string) (Device, bool) {
	if len(fields) < 7 {
		return Device{}, false
	}

	samples, err := strconv.Atoi(fields[3])
	if err != nil {
		return Device{}, false
	}

	mean, err := strconv.ParseFloat(fields[4], 64)
	if err != nil {
		return Device{}, false
	}
	peak, err := strconv.ParseFloat(fields[5], 64)
	if err != nil {
		return Device{}, false
	}
	peakAt, err := strconv.ParseFloat(fields[6], 64)
	if err != nil {
		return Device{}, false
	}

	out := Device{
		Name:    fields[1],
		Kind:    fields[2],
		Samples: samples,
		Mean:    mean,
		Peak:    peak,
		PeakAt:  peakAt,
	}

	if len(fields) > 7 {
		out.Failures, _ = strconv.Atoi(fields[7])
	}

	return out, true
}

// s <time> <volts> <amps...>
func readSample(into *Series, fields []string) {
	if len(fields) < 3 || len(into.Names) == 0 {
		return
	}

	at, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return
	}
	volts, _ := strconv.ParseFloat(fields[2], 64)

	into.Times = append(into.Times, at)
	into.Volts = append(into.Volts, volts)

	for i := range into.Names {
		var amps float64
		if i+3 < len(fields) {
			amps, _ = strconv.ParseFloat(fields[i+3], 64)
		}
		into.Amps[i] = append(into.Amps[i], amps)
	}
}

// Motors are the readings from motors alone.
//
// The hub's own reading is the sum of everything plugged into it, so mixing the
// two in one ranking would put the hub at the top of every list and say nothing.
func (r Report) Motors() []Device {
	var out []Device
	for _, d := range r.Devices {
		if d.Kind == "motor" {
			out = append(out, d)
		}
	}
	return out
}

// Hubs are the readings from the hubs themselves.
func (r Report) Hubs() []Device {
	var out []Device
	for _, d := range r.Devices {
		if d.Kind != "motor" {
			out = append(out, d)
		}
	}
	return out
}

// Total is the mean draw of every motor added up, which is roughly what the
// battery was supplying to move the robot.
func (r Report) Total() float64 {
	var sum float64
	for _, d := range r.Motors() {
		sum += d.Mean
	}
	return sum
}

// Sag is how far the battery fell over the run.
func (r Report) Sag() float64 {
	if r.LowVolts <= 0 || r.PeakVolts <= 0 {
		return 0
	}
	return r.PeakVolts - r.LowVolts
}

// Lines renders the report, for whatever is going to show it.
//
// One renderer rather than two. The menu and the command showed the same
// numbers on the day they were written and would have stopped agreeing the
// first time either was touched.
func (r Report) Lines() []string {
	var out []string

	if r.Problem {
		return []string{"The monitor recorded nothing, and left this:", "", r.Note}
	}

	add := func(format string, args ...any) {
		out = append(out, fmt.Sprintf(format, args...))
	}

	motors := r.Motors()
	if len(motors) > 0 {
		add("%-20s %8s %8s %9s", "motor", "peak", "average", "peak at")

		for _, d := range motors {
			failed := ""
			if d.Failures > 0 {
				failed = fmt.Sprintf("  (%d reads failed)", d.Failures)
			}
			add("%-20s %7.2fA %7.2fA %8.1fs%s", trim(d.Name, 20), d.Peak, d.Mean, d.PeakAt, failed)
		}

		add("")
		add("%-20s %7.2fA drawn by the motors together, on average", "", r.Total())
	}

	for _, hub := range r.Hubs() {
		add("%-20s %7.2fA peak, %.2fA average", trim(hub.Name, 15)+" (hub)", hub.Peak, hub.Mean)
	}

	if r.PeakVolts > 0 {
		add("%-20s %7.2fV down to %.2fV, a sag of %.2fV", "battery", r.PeakVolts, r.LowVolts, r.Sag())
	}

	add("")
	add("Sampled every %dms. Peaks shorter than that are not in here.", r.Period)

	return out
}

// Title is how one run is named in a list.
func (r Report) Title() string {
	if r.Problem {
		return "Nothing was recorded"
	}
	return fmt.Sprintf("%s, %.0fs", r.OpMode, r.Seconds)
}

func trim(name string, width int) string {
	if len(name) <= width {
		return name
	}
	return name[:width-1] + "…"
}
