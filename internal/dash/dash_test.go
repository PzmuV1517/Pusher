package dash

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

// The dashboard writes what the double holds and java writes what somebody
// typed. Comparing those as text reports every field as changed, which is the
// whole report being wrong.
func TestValuesCompareRegardlessOfHowTheyWereWritten(t *testing.T) {
	same := [][2]string{
		{"0.5", ".5"},
		{"0.5", "0.50"},
		{"0.5", "0.5d"},
		{"12", "12.0"},
		{"12", "12f"},
		{"1000", "1_000"},
		{"-0.25", "-.25"},
		{"0.001", "1e-3"},
	}

	for _, pair := range same {
		if a, b := Normalise(pair[0]), Normalise(pair[1]); a != b {
			t.Errorf("%q and %q should compare equal, got %q and %q",
				pair[0], pair[1], a, b)
		}
	}

	if Normalise("0.5") == Normalise("0.6") {
		t.Error("different numbers compare equal")
	}
}

// An enum arrives as the bare constant name whatever the source qualified it
// with, and a hex literal must not lose its trailing digits to suffix trimming.
func TestNonNumericValuesAreNormalised(t *testing.T) {
	cases := map[string]string{
		"LightColor.RED": "RED",
		"RED":            "RED",
		"true":           "true",
		`"hello"`:        `"hello"`,
		"0xFF":           "255",
		"0x1D":           "29",
	}

	for in, want := range cases {
		if got := Normalise(in); got != want {
			t.Errorf("Normalise(%q) = %q, want %q", in, got, want)
		}
	}
}

const configured = `package org.firstinspires.ftc.teamcode;

import com.acmerobotics.dashboard.config.Config;

@Config
public class Tuning {
    private Robot robot;

    public static double kP = 0.012;
    public static boolean useStall = false;
    public static final double NOT_TUNABLE = 9.0;
    public static String label = "arm";
    public double notStatic = 1.0;
    static double notPublic = 2.0;

    // public static double commented = 5.0;
    public static String brace = "} public static double fake = 1.0;";

    public static class Inner {
        public static double nested = 3.0;
    }

    public void method() {
        double local = 4.0;
    }
}
`

func TestOnlyTunableFieldsAreRead(t *testing.T) {
	fields := FromFile("Tuning.java", configured)

	got := map[string]string{}
	for _, f := range fields {
		if f.Section != "Tuning" {
			t.Errorf("%s filed under %q, want Tuning", f.Name, f.Section)
		}
		got[f.Name] = f.Value
	}

	want := map[string]string{
		"kP":       "0.012",
		"useStall": "false",
		"label":    `"arm"`,
		"brace":    `"} public static double fake = 1.0;"`,
	}

	for name, value := range want {
		if got[name] != value {
			t.Errorf("%s = %q, want %q", name, got[name], value)
		}
	}

	// final cannot be set from the dashboard, an instance field is not
	// reflected, a nested class is its own section, and neither a comment nor a
	// string literal declares anything.
	for _, name := range []string{"NOT_TUNABLE", "notStatic", "notPublic", "nested", "local", "commented", "fake"} {
		if _, found := got[name]; found {
			t.Errorf("%s was read as a tunable", name)
		}
	}
}

// One declaration can declare several fields. Reading only the first loses the
// rest and files their values under the wrong name.
func TestEveryDeclaratorIsReadNotJustTheFirst(t *testing.T) {
	source := `@Config
public class Constants {
    public static double hP = 1.2, hI = 0, hD = 0.11;
    public static int only = 5;
}`

	got := map[string]string{}
	for _, f := range FromFile("Constants.java", source) {
		got[f.Name] = f.Value
	}

	want := map[string]string{"hP": "1.2", "hI": "0", "hD": "0.11", "only": "5"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for name, value := range want {
		if got[name] != value {
			t.Errorf("%s = %q, want %q", name, got[name], value)
		}
	}
}

// The robot reports what an expression evaluated to and the source says how it
// is worked out. Comparing those as text flags the field on every single run,
// so it is left out instead.
func TestComputedInitialisersAreNotCompared(t *testing.T) {
	source := `@Config
public class Constants {
    public static double angle = Math.toRadians(3);
    public static double scaled = 2 * 3;
    public static double plain = 0.05;
    public static PodType pod = goBILDA_4_BAR_POD;
}`

	computed := map[string]bool{}
	for _, f := range FromFile("Constants.java", source) {
		computed[f.Name] = f.Computed
	}

	for _, name := range []string{"angle", "scaled"} {
		if !computed[name] {
			t.Errorf("%s should be marked computed", name)
		}
	}
	for _, name := range []string{"plain", "pod"} {
		if computed[name] {
			t.Errorf("%s should be comparable", name)
		}
	}

	code := Source{"C.angle": {Value: "toRadians(3)", Computed: true}}
	result := Compare(Values{"C.angle": "0.05235987755982988"}, code, nil)

	if len(result.Unsaved) != 0 {
		t.Errorf("a computed field was reported as changed: %+v", result.Unsaved)
	}
	if result.Computed != 1 {
		t.Errorf("computed = %d, want 1", result.Computed)
	}
}

func TestTheSectionNameFollowsTheAnnotation(t *testing.T) {
	named := `@Config("Drive")
public class DriveConstants {
    public static double kV = 1.0;
}`

	fields := FromFile("DriveConstants.java", named)
	if len(fields) != 1 {
		t.Fatalf("got %d fields, want 1", len(fields))
	}
	if fields[0].Key() != "Drive.kV" {
		t.Errorf("got %q, want Drive.kV", fields[0].Key())
	}
}

func TestAnUnannotatedClassDeclaresNothing(t *testing.T) {
	plain := `public class Plain {
    public static double kP = 1.0;
}`

	if fields := FromFile("Plain.java", plain); len(fields) != 0 {
		t.Errorf("got %d fields from a class with no @Config", len(fields))
	}
}

// Without a previous reading, agreeing with the source is indistinguishable
// from never having been touched, and saying otherwise would be a guess.
func TestSavedNeedsAPreviousReading(t *testing.T) {
	code := Source{
		"Tuning.kP": {Section: "Tuning", Name: "kP", Value: "0.02"},
		"Tuning.kD": {Section: "Tuning", Name: "kD", Value: "0.5"},
	}
	live := Values{"Tuning.kP": "0.02", "Tuning.kD": "0.9"}

	first := Compare(live, code, nil)
	if len(first.Saved) != 0 {
		t.Errorf("reported %d saved with no history", len(first.Saved))
	}
	if len(first.Unsaved) != 1 || first.Unsaved[0].Key != "Tuning.kD" {
		t.Errorf("unsaved = %+v, want just Tuning.kD", first.Unsaved)
	}
	if first.Untouched != 1 {
		t.Errorf("untouched = %d, want 1", first.Untouched)
	}

	// kP held something else last time and matches the source now, so it was
	// tuned and then written down.
	second := Compare(live, code, Values{"Tuning.kP": "0.01", "Tuning.kD": "0.9"})
	if len(second.Saved) != 1 || second.Saved[0].Key != "Tuning.kP" {
		t.Errorf("saved = %+v, want just Tuning.kP", second.Saved)
	}
	if second.Untouched != 0 {
		t.Errorf("untouched = %d, want 0", second.Untouched)
	}
}

func TestAValueTheSourceDoesNotDeclareIsNotAChange(t *testing.T) {
	result := Compare(Values{"Old.gone": "1"}, Source{}, nil)

	if len(result.Unsaved) != 0 {
		t.Errorf("reported %d unsaved for a field the source lacks", len(result.Unsaved))
	}
	if len(result.Unknown) != 1 {
		t.Errorf("unknown = %v, want one entry", result.Unknown)
	}
}

// Two robots do not hold the same tuning, and a Wi-Fi serial is not a filename.
func TestSnapshotsAreKeptPerRobot(t *testing.T) {
	usb := SnapshotPath("/cfg", "1234ABCD")
	wifi := SnapshotPath("/cfg", "192.168.43.1:5555")

	if usb == wifi {
		t.Error("two robots share a snapshot")
	}
	for _, path := range []string{usb, wifi} {
		if base := filepath.Base(path); base == "" || filepath.Dir(path) == path {
			t.Errorf("%q is not a usable path", path)
		}
	}
	if filepath.Base(wifi) != "192_168_43_1_5555.json" {
		t.Errorf("wifi snapshot is %q", filepath.Base(wifi))
	}
}

func TestASnapshotSurvivesARoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dash", "robot.json")

	values := Values{"Tuning.kP": "0.02"}
	if err := Save(path, values); err != nil {
		t.Fatal(err)
	}

	back, taken := Load(path)
	if back["Tuning.kP"] != "0.02" {
		t.Errorf("read back %v", back)
	}
	if time.Since(taken) > time.Minute {
		t.Errorf("taken = %v", taken)
	}
}

func TestMissingSnapshotIsNotAnError(t *testing.T) {
	values, taken := Load(filepath.Join(t.TempDir(), "nothing.json"))
	if values != nil || !taken.IsZero() {
		t.Errorf("got %v, %v for a file that does not exist", values, taken)
	}
}

// The tree the dashboard sends nests classes as "custom" nodes and tags every
// leaf with its type, which is the only thing that says a string from an enum.
func TestTheConfigTreeFlattensToDottedNames(t *testing.T) {
	payload := `{
      "__type": "custom",
      "__value": {
        "Tuning": {
          "__type": "custom",
          "__value": {
            "kP":    {"__type": "double",  "__value": 0.0155},
            "on":    {"__type": "boolean", "__value": true},
            "label": {"__type": "string",  "__value": "arm"},
            "mode":  {"__type": "enum",    "__value": "FAST"}
          }
        },
        "Empty": {"__type": "custom", "__value": {}}
      }
    }`

	var root node
	if err := json.Unmarshal([]byte(payload), &root); err != nil {
		t.Fatal(err)
	}

	got := Values{}
	flatten("", root, got)

	want := Values{
		"Tuning.kP":    "0.0155",
		"Tuning.on":    "true",
		"Tuning.label": `"arm"`,
		"Tuning.mode":  "FAST",
	}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("%s = %q, want %q", key, got[key], value)
		}
	}
}

// A string and an enum both arrive as JSON strings, and only the type says
// which. Quoting the wrong one makes it never match the source.
func TestAStringIsQuotedAndAnEnumIsNot(t *testing.T) {
	str, _ := literal(node{Type: "string", Value: json.RawMessage(`"FAST"`)})
	enum, _ := literal(node{Type: "enum", Value: json.RawMessage(`"FAST"`)})

	if str != `"FAST"` {
		t.Errorf("string literal = %q", str)
	}
	if enum != "FAST" {
		t.Errorf("enum literal = %q", enum)
	}
}

// The payload sits beside the type, not under a "data" key. Reading it the
// other way silently produced an empty config on every real robot, and no test
// noticed because the only tested half was the tree walk underneath.
//
// The shapes here are the ones the client bundled in the dashboard AAR reads:
// t.configRoot and t.opModeInfoList, straight off the message.
func TestTheConfigArrivesBesideTheTypeNotUnderIt(t *testing.T) {
	const message = `{"type":"RECEIVE_CONFIG","configRoot":{"__type":"custom","__value":` +
		`{"Tuning":{"__type":"custom","__value":{"kP":{"__type":"double","__value":1.5}}}}}}`

	var wrapper envelope
	if err := json.Unmarshal([]byte(message), &wrapper); err != nil {
		t.Fatal(err)
	}
	if wrapper.Type != "RECEIVE_CONFIG" {
		t.Fatalf("type = %q", wrapper.Type)
	}

	var root node
	if err := json.Unmarshal(wrapper.ConfigRoot, &root); err != nil {
		t.Fatalf("the config was not found beside the type: %v", err)
	}

	got := Values{}
	flatten("", root, got)

	if got["Tuning.kP"] != "1.5" {
		t.Errorf("Tuning.kP = %q, want 1.5 (got %v)", got["Tuning.kP"], got)
	}
}

func TestTheOpModeListArrivesBesideTheType(t *testing.T) {
	const message = `{"type":"RECEIVE_OP_MODE_LIST","opModeInfoList":` +
		`[{"name":"RST TUNING","group":"tuning2"},{"name":"TeleOp Red","group":"$$$$$$$"}]`

	var wrapper envelope
	if err := json.Unmarshal([]byte(message+"}"), &wrapper); err != nil {
		t.Fatal(err)
	}

	names := Names(wrapper.OpModeInfoList)
	if len(names) != 2 || names[0] != "RST TUNING" || names[1] != "TeleOp Red" {
		t.Errorf("names = %v, want the two the robot listed", names)
	}
	if wrapper.OpModeInfoList[0].Group != "tuning2" {
		t.Errorf("group = %q", wrapper.OpModeInfoList[0].Group)
	}
}
