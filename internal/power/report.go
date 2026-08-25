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
