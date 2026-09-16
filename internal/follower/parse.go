package follower

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Row is one control loop, start to finish.
//
// The order here is the order the loop happens in: the clock, what the
// localizer measured, what the profile asked for, what each controller
// produced, what the feedforward terms added, and what reached the motors after
// mixing and desaturation. A row is a complete account of one loop, which is
// what makes the whole thing replayable offline.
type Row struct {
	// TimeMs is milliseconds since the OpMode started. Dt is the period the
	// controller actually used, in seconds, and every coefficient was
	// recomputed from it rather than from a nominal rate.
	TimeMs float64
	Dt     float64

	// The localizer, which is what the loop had to work with. X and Y are
	// inches on the field, Heading is radians CCW from +X and is a plain field
	// heading. VF and VL are inches per second out of the robot's front and
	// left, so they are robot frame, exactly as the controller saw them. Omega
	// is radians per second, CCW positive.
	X, Y, Heading float64
	VF, VL, Omega float64

	// What the profile asked for, in the same frames and signs as the
	// measurements above it, so a reference and its measurement can be
	// subtracted without thinking about it.
	RefF, RefL, RefOmega float64

	// SpeedRef is the scalar speed before it was split between the two
	// translational axes. EndDist is inches left to the end of the path.
	// Curvature is signed, 1/in, positive curving left and zero on a line.
	// Progress is 0..1 along the arc.
	SpeedRef, EndDist, Curvature, Progress float64

	// What the controller put out, in volts, all of them totals.
	VoltsF, VoltsL, VoltsT float64

	// How much of that total was feedforward: the kS term, and the centripetal
	// term on the two translational axes. The RST's own contribution is the
	// total less these, which is the difference between a command that is
	// feedback and one that is feedforward.
	StaticF, StaticL, StaticT float64
	ExtraF, ExtraL            float64

	// What actually reached the motors: unit power in -1..1, after mixing and
	// after desaturation, left front, left back, right front, right back. This
	// group is why the log exists. Everything above is what the controller
	// wanted; this is what the battery and the mix allowed.
	LF, LB, RF, RB float64

	// Battery is the pack voltage measured that loop. Every volts to power
	// conversion divides by it, so it is not context, it is a term.
	Battery float64

	// HeadingError is NOT in the same convention as Heading. It is blob's
	// mirrored convention, where a positive error is asking for a CLOCKWISE
	// turn. Field heading error is the negative of it, which is what
	// FieldHeadingError returns. Comparing this against Omega or RefOmega
	// without negating it makes a robot turning correctly look like one turning
	// the wrong way, and that has caused real bugs inside blob.
	HeadingError float64

	// Settled is the follower commanding nothing. On a settled row every
	// reference, every volt and every feedforward term is written zero, and
	// that is real rather than missing.
	Settled bool
}

// FieldHeadingError is the heading error in the same convention as everything
// else: radians, CCW positive.
//
// The file's own column is mirrored. This is the only place that negation
// should happen, so that comparing the error against Omega is safe by default
// instead of safe if somebody remembered.
func (r Row) FieldHeadingError() float64 { return -r.HeadingError }

// Log is one recorded run.
type Log struct {
	Version int
	Name    string
	When    time.Time
	Rows    []Row

	// Columns is the header exactly as it arrived, so a file with a column this
	// version of pusher does not know about can say so rather than look
	// identical to one without it.
	Columns []string
	Extra   []string

	// Partial marks a final row that was cut off mid-write. The recorder
	// rewrites the whole file every couple of seconds, so a pull can land in
	// the middle of one; the row is dropped rather than parsed into zeros.
	Partial bool
}

// Capped reports whether the recorder stopped taking rows, which it does
// silently at MaxRows.
func (l *Log) Capped() bool { return len(l.Rows) >= MaxRows }

// Duration is how long the recorded part of the run lasted, in seconds.
func (l *Log) Duration() float64 {
	if len(l.Rows) == 0 {
		return 0
	}
	return (l.Rows[len(l.Rows)-1].TimeMs - l.Rows[0].TimeMs) / 1000
}

// columns are the ones this format cannot be read without. Future versions may
// append to the end of the header, so a file is read by name and an unknown
// column is kept out of the way rather than treated as a problem.
var columns = []string{
	"time_ms", "dt",
	"x", "y", "heading", "vf", "vl", "omega",
	"ref_f", "ref_l", "ref_omega",
	"volts_f", "volts_l", "volts_t",
	"static_f", "static_l", "static_t", "extra_f", "extra_l",
	"lf", "lb", "rf", "rb",
	"battery", "end_dist", "curvature", "progress",
	"speed_ref", "heading_error", "settled",
}

const marker = "# blob follower v"

// Looks reports whether a file is a follower log, cheaply and without reading
// it all, so a path handed to the visualiser can be recognised rather than
// guessed at from its extension.
func Looks(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	line, err := bufio.NewReader(f).ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(line), marker)
}

// Load reads a follower log off disk.
func Load(path string) (*Log, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", filepath.Base(path), err)
	}
	defer f.Close()

	log, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}

	log.Name = filepath.Base(path)
	log.When = startedAt(log.Name)
	return log, nil
}

// Parse reads a follower log.
func Parse(r io.Reader) (*Log, error) {
	scan := bufio.NewScanner(r)

	// Rows are short, but a pull that landed mid-write can leave one that is
	// not, and the default 64 KB limit would turn that into a scan error rather
	// than a dropped row.
	scan.Buffer(make([]byte, 0, 64*1024), 1<<20)

	if !scan.Scan() {
		return nil, fmt.Errorf("the file is empty")
	}

	version, err := versionOf(scan.Text())
	if err != nil {
		return nil, err
	}

	if !scan.Scan() {
		return nil, fmt.Errorf("the file has a version line and nothing else")
	}

	header := splitRow(scan.Text())
	at, extra, err := index(header)
	if err != nil {
		return nil, err
	}

	log := &Log{Version: version, Columns: header, Extra: extra}

	// The last line is held back rather than parsed as it arrives, because
	// whether it is short only matters if nothing follows it: a short line in
	// the middle of a file is a corrupt file, and a short line at the end is a
	// pull that raced the recorder.
	var pending []string
	var line int

	for scan.Scan() {
		line++
		text := strings.TrimSpace(scan.Text())
		if text == "" {
			continue
		}

		fields := splitRow(text)
		if pending != nil {
			row, err := read(pending, at, line-1)
			if err != nil {
				return nil, err
			}
			log.Rows = append(log.Rows, row)
		}
		pending = fields
	}
	if err := scan.Err(); err != nil {
		return nil, fmt.Errorf("cannot read the rows: %w", err)
	}

	if pending != nil {
		row, err := read(pending, at, line)
		if err != nil {
			// The recorder rewrites the file whole, so the only line that can
			// be half written is the last one. Dropping it costs one loop out
			// of thousands and keeps a mid-run pull readable.
			log.Partial = true
		} else {
			log.Rows = append(log.Rows, row)
		}
	}

	if len(log.Rows) == 0 {
		return nil, fmt.Errorf("the header is there but no loops were recorded")
	}
	return log, nil
}

// versionOf reads the marker line, and refuses anything it does not know.
//
// Refuses rather than guesses. The version is the only promise the file makes
// about what its columns mean, and a reader that shrugs and carries on would
// draw a confident chart of the wrong numbers.
func versionOf(line string) (int, error) {
	text := strings.TrimSpace(line)
	if !strings.HasPrefix(text, marker) {
		return 0, fmt.Errorf("this is not a blob follower log: it starts with %q rather than %q",
			fit(text), marker+strconv.Itoa(FormatVersion))
	}

	version, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(text, marker)))
	if err != nil {
		return 0, fmt.Errorf("the version line %q does not end in a number", fit(text))
	}

	if version != FormatVersion {
		return 0, fmt.Errorf("this log is format v%d and pusher reads v%d.\n"+
			"    A different major version means the columns themselves moved, so reading\n"+
			"    it anyway would chart the wrong numbers. Update pusher, or use the blob\n"+
			"    version that wrote it", version, FormatVersion)
	}
	return version, nil
}

// index maps each column name to where it sits, and reports what it has never
// heard of.
func index(header []string) (map[string]int, []string, error) {
	at := make(map[string]int, len(header))
	for i, name := range header {
		at[strings.TrimSpace(name)] = i
	}

	var missing []string
	for _, want := range columns {
		if _, ok := at[want]; !ok {
			missing = append(missing, want)
		}
	}
	if len(missing) > 0 {
		return nil, nil, fmt.Errorf("the header is missing %s, which this format cannot be read without",
			strings.Join(missing, ", "))
	}

	known := map[string]bool{}
	for _, name := range columns {
		known[name] = true
	}

	var extra []string
	for _, name := range header {
		if name = strings.TrimSpace(name); name != "" && !known[name] {
			extra = append(extra, name)
		}
	}

	return at, extra, nil
}

func splitRow(line string) []string {
	return strings.Split(strings.TrimRight(strings.TrimSpace(line), ","), ",")
}

// read turns one line into a loop.
func read(fields []string, at map[string]int, line int) (Row, error) {
	var row Row
	var bad error

	get := func(name string) float64 {
		i := at[name]
		if i >= len(fields) {
			if bad == nil {
				bad = fmt.Errorf("row %d stops after %d of %d columns", line, len(fields), len(at))
			}
			return 0
		}

		value, err := strconv.ParseFloat(strings.TrimSpace(fields[i]), 64)
		if err != nil {
			if bad == nil {
				bad = fmt.Errorf("row %d has %q in the %s column, which is not a number",
					line, fit(fields[i]), name)
			}
			return 0
		}
		return value
	}

	row.TimeMs, row.Dt = get("time_ms"), get("dt")
	row.X, row.Y, row.Heading = get("x"), get("y"), get("heading")
	row.VF, row.VL, row.Omega = get("vf"), get("vl"), get("omega")
	row.RefF, row.RefL, row.RefOmega = get("ref_f"), get("ref_l"), get("ref_omega")
	row.SpeedRef, row.EndDist = get("speed_ref"), get("end_dist")
	row.Curvature, row.Progress = get("curvature"), get("progress")
	row.VoltsF, row.VoltsL, row.VoltsT = get("volts_f"), get("volts_l"), get("volts_t")
	row.StaticF, row.StaticL, row.StaticT = get("static_f"), get("static_l"), get("static_t")
	row.ExtraF, row.ExtraL = get("extra_f"), get("extra_l")
	row.LF, row.LB, row.RF, row.RB = get("lf"), get("lb"), get("rf"), get("rb")
	row.Battery, row.HeadingError = get("battery"), get("heading_error")
	row.Settled = get("settled") != 0

	return row, bad
}

// fit keeps a quoted fragment of somebody else's file short enough to read.
func fit(text string) string {
	const most = 40

	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= most {
		return string(runes)
	}
	return string(runes[:most]) + "..."
}
