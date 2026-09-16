package follower

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// header is the real one, in the order the recorder writes it.
const header = "time_ms,dt,x,y,heading,vf,vl,omega," +
	"ref_f,ref_l,ref_omega,volts_f,volts_l,volts_t," +
	"static_f,static_l,static_t,extra_f,extra_l," +
	"lf,lb,rf,rb,battery,end_dist,curvature,progress," +
	"speed_ref,heading_error,settled"

// one row of it, with a different number in every column so a reader that maps
// by position rather than by name cannot accidentally pass.
const row = "1000,0.017,12,34,1.5,40,2,0.3," +
	"41,2.5,0.31,6.5,1.2,3.4," +
	"1.02,1.79,1.26,0.4,0.6," +
	"0.5,0.51,0.52,0.53,12.4,60,0.01,0.25," +
	"41.1,0.05,0"

func log(t *testing.T, text string) *Log {
	t.Helper()

	l, err := Parse(strings.NewReader(text))
	if err != nil {
		t.Fatalf("would not parse: %v", err)
	}
	return l
}

func TestEveryColumnLandsWhereItBelongs(t *testing.T) {
	l := log(t, "# blob follower v1\n"+header+"\n"+row+"\n")

	r := l.Rows[0]
	for _, tc := range []struct {
		name string
		got  float64
		want float64
	}{
		{"time_ms", r.TimeMs, 1000}, {"dt", r.Dt, 0.017},
		{"x", r.X, 12}, {"y", r.Y, 34}, {"heading", r.Heading, 1.5},
		{"vf", r.VF, 40}, {"vl", r.VL, 2}, {"omega", r.Omega, 0.3},
		{"ref_f", r.RefF, 41}, {"ref_l", r.RefL, 2.5}, {"ref_omega", r.RefOmega, 0.31},
		{"volts_f", r.VoltsF, 6.5}, {"volts_l", r.VoltsL, 1.2}, {"volts_t", r.VoltsT, 3.4},
		{"static_f", r.StaticF, 1.02}, {"static_l", r.StaticL, 1.79}, {"static_t", r.StaticT, 1.26},
		{"extra_f", r.ExtraF, 0.4}, {"extra_l", r.ExtraL, 0.6},
		{"lf", r.LF, 0.5}, {"lb", r.LB, 0.51}, {"rf", r.RF, 0.52}, {"rb", r.RB, 0.53},
		{"battery", r.Battery, 12.4}, {"end_dist", r.EndDist, 60},
		{"curvature", r.Curvature, 0.01}, {"progress", r.Progress, 0.25},
		{"speed_ref", r.SpeedRef, 41.1}, {"heading_error", r.HeadingError, 0.05},
	} {
		if tc.got != tc.want {
			t.Errorf("%s read as %v, want %v", tc.name, tc.got, tc.want)
		}
	}

	if r.Settled {
		t.Error("read settled as true from a 0")
	}
}

// Minor versions of this format only ever append columns, so a log from a newer
// blob has to keep working. Reading by name is what makes that true, and a
// reader that quietly took the columns in order would pass every other test
// here and fail on the first robot running a newer library.
func TestAnAppendedColumnDoesNotMoveTheOthers(t *testing.T) {
	l := log(t, "# blob follower v1\n"+
		header+",imu_temp\n"+
		row+",41.5\n")

	if l.Rows[0].VoltsT != 3.4 {
		t.Errorf("volts_t came back as %v with a column appended", l.Rows[0].VoltsT)
	}
	if len(l.Extra) != 1 || l.Extra[0] != "imu_temp" {
		t.Errorf("did not notice the new column: %v", l.Extra)
	}
}

// And reordering, for the same reason: position is not the contract.
func TestTheColumnsCanArriveInAnyOrder(t *testing.T) {
	names := strings.Split(header, ",")
	values := strings.Split(row, ",")

	// Reversed is the cheapest permutation that shares no position with the
	// original except the middle.
	for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
		names[i], names[j] = names[j], names[i]
		values[i], values[j] = values[j], values[i]
	}

	l := log(t, "# blob follower v1\n"+
		strings.Join(names, ",")+"\n"+
		strings.Join(values, ",")+"\n")

	if l.Rows[0].Battery != 12.4 || l.Rows[0].RefF != 41 {
		t.Errorf("read the wrong columns from a reordered header: battery %v, ref_f %v",
			l.Rows[0].Battery, l.Rows[0].RefF)
	}
}

// A major version means the columns themselves moved. Reading it anyway would
// draw a confident page of the wrong numbers, which is worse than refusing.
func TestAVersionThisDoesNotKnowIsRefused(t *testing.T) {
	_, err := Parse(strings.NewReader("# blob follower v2\n" + header + "\n" + row + "\n"))
	if err == nil {
		t.Fatal("read a format nobody has written a reader for")
	}

	for _, want := range []string{"v2", "v1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q: %v", want, err)
		}
	}
}

// blob writes three different things under /sdcard/FIRST and they are all
// somebody's idea of a run. Being handed the wrong one should say so.
func TestTheOtherThingsBlobWritesAreNotMistakenForThis(t *testing.T) {
	for _, text := range []string{
		"# blob sysid v1\ntime_ms,volts,velocity\n0,1,2\n",
		"{\"version\":1,\"opMode\":\"CloseBlue\",\"segments\":[]}\n",
		"",
	} {
		if _, err := Parse(strings.NewReader(text)); err == nil {
			t.Errorf("accepted something that is not a follower log: %q", text)
		}
	}
}

func TestAMissingColumnIsNamedRatherThanGuessedAt(t *testing.T) {
	without := strings.Replace(header, ",battery", "", 1)
	values := strings.Replace(row, ",12.4", "", 1)

	_, err := Parse(strings.NewReader("# blob follower v1\n" + without + "\n" + values + "\n"))
	if err == nil {
		t.Fatal("read a log with no battery column, which every volts to power conversion needs")
	}
	if !strings.Contains(err.Error(), "battery") {
		t.Errorf("did not say which column was missing: %v", err)
	}
}

// The recorder rewrites the whole file every couple of seconds, so a pull can
// land in the middle of one. That costs the last row and nothing else, and it
// is the normal way to read a run that is still going.
func TestAHalfWrittenLastRowIsDroppedRatherThanZeroed(t *testing.T) {
	l := log(t, "# blob follower v1\n"+header+"\n"+row+"\n"+"1017,0.017,12,34,1.5,40,2,0.3,41")

	if len(l.Rows) != 1 {
		t.Fatalf("kept %d rows, want the complete one only", len(l.Rows))
	}
	if !l.Partial {
		t.Error("dropped the half written row without saying so")
	}
}

// A short row anywhere else is a corrupt file rather than a race, and pretending
// its missing columns are zeros would put a loop of nothing into the middle of
// the charts.
func TestAShortRowInTheMiddleIsAnError(t *testing.T) {
	_, err := Parse(strings.NewReader("# blob follower v1\n" + header + "\n" +
		"1017,0.017,12\n" + row + "\n"))

	if err == nil {
		t.Fatal("read a truncated row in the middle of a file as if it were a loop")
	}
}

func TestAFileWithNoLoopsInItSaysSo(t *testing.T) {
	_, err := Parse(strings.NewReader("# blob follower v1\n" + header + "\n"))
	if err == nil {
		t.Fatal("accepted a header with nothing under it")
	}
}

// Recognition has to be cheap and has to be right, because it is what decides
// whether a file handed to the visualiser is a path trace or one of these.
func TestAFollowerLogIsRecognisedFromItsFirstLine(t *testing.T) {
	dir := t.TempDir()

	yes := filepath.Join(dir, "follower-1700000000000.csv")
	no := filepath.Join(dir, "trace.json")

	if err := os.WriteFile(yes, []byte("# blob follower v1\n"+header+"\n"+row+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(no, []byte("{\"version\":1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !Looks(yes) {
		t.Error("did not recognise a follower log")
	}
	if Looks(no) || Looks(filepath.Join(dir, "gone.csv")) {
		t.Error("recognised something that is not one")
	}

	// And the name carries the only thing that identifies the run, since the
	// file itself never says which OpMode wrote it.
	l, err := Load(yes)
	if err != nil {
		t.Fatal(err)
	}
	if l.When.UnixMilli() != 1700000000000 {
		t.Errorf("start time read as %v", l.When)
	}
}
