package follower

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func sample() *Log {
	var rows []Row

	for i := 0; i < 400; i++ {
		r := Row{
			TimeMs:  float64(i * 17),
			Dt:      0.017,
			Battery: 12.4,
			RefF:    40, VF: 38,
			RefOmega: 0.4, Omega: 0.35,
			VoltsF: 6.5, VoltsL: 1.2, VoltsT: 3.4,
			StaticF: 1.02, StaticL: 1.79, StaticT: 1.26,
			HeadingError: 0.02,
			SpeedRef:     40,
		}
		if i > 300 {
			r = Row{TimeMs: float64(i * 17), Dt: 0.017, Battery: 12.4, Settled: true}
		}
		r.LF, r.LB, r.RF, r.RB = r.Wanted()
		rows = append(rows, r)
	}

	return &Log{Version: 1, Name: "follower-1700000000000.csv", Rows: rows}
}

// The payload is written by hand rather than marshalled, to keep a five minute
// run from becoming a ten megabyte page. Hand written JSON that does not parse
// is a blank page with an error in a console nobody opens, so it is checked
// here rather than discovered there.
func TestThePayloadIsValidJSON(t *testing.T) {
	var payload map[string]json.RawMessage

	if err := json.Unmarshal([]byte(sample().plot()), &payload); err != nil {
		t.Fatalf("the page's own data does not parse: %v", err)
	}

	for _, key := range []string{"t", "dt", "vf", "ref_f", "volts_t", "deliv_t", "lf", "battery",
		"herr", "sat", "settled", "histEdges", "histCounts", "meanDt", "maxDt", "minTau"} {
		if _, ok := payload[key]; !ok {
			t.Errorf("the page needs %q and the payload does not carry it", key)
		}
	}

	var t0 []float64
	if err := json.Unmarshal(payload["t"], &t0); err != nil || len(t0) != 400 {
		t.Errorf("the time axis came out as %d points: %v", len(t0), err)
	}
	if t0[0] != 0 {
		t.Errorf("the time axis starts at %v rather than at the first loop", t0[0])
	}
}

// A NaN is not valid JSON, so one bad loop would take the whole page with it.
func TestOneImpossibleLoopDoesNotTakeThePageWithIt(t *testing.T) {
	l := sample()
	l.Rows[10].Battery = 0
	l.Rows[10].VF = math.Inf(1)

	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(l.plot()), &payload); err != nil {
		t.Fatalf("a single unusable loop broke the payload: %v", err)
	}
}

// Shaded loops are drawn as runs rather than one rectangle per loop, since at
// four hundred loops to the pixel the difference is a page that opens and one
// that hangs.
func TestShadedLoopsAreGroupedIntoRuns(t *testing.T) {
	l := &Log{Rows: []Row{
		{Settled: false}, {Settled: true}, {Settled: true}, {Settled: false}, {Settled: true},
	}}

	got := l.spansWhere(func(r Row) bool { return r.Settled })
	want := [][2]int{{1, 2}, {4, 4}}

	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("grouped them as %v, want %v", got, want)
	}
}

func TestThePageIsWrittenAndSaysWhatItIs(t *testing.T) {
	out := filepath.Join(t.TempDir(), "page.html")

	if _, err := sample().Render(out); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	page := string(body)

	for _, want := range []string{
		"Follower log",
		"asked against measured",
		"asked against delivered",
		"Loop period",
		"var D = ",
		// The sign trap, on the page rather than only in the source, because
		// the person reading the chart is the one who needs it.
		"mirrored",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page never mentions %q", want)
		}
	}

	// html/template escapes the payload if it is ever handed over as a string
	// rather than as JS, and the page then draws nothing at all.
	if regexp.MustCompile(`var D = &(34|quot);`).MatchString(page) {
		t.Error("the payload was escaped into a string, so the page cannot read it")
	}
}
