package dash

import (
	"encoding/json"
	"testing"
)

// The payloads here are built from the wire types in the published AARs:
// com.bylazar.opmodecontrol's OpModesList of OpModeDetails, and
// com.bylazar.configurables' map of class name to GenericTypeJson. Field names
// are gson's, which is to say the Kotlin property names.

// frame wraps a payload the way Panels' SocketMessage does.
func frame(plugin, message string, data any) string {
	body, err := json.Marshal(map[string]any{
		"pluginID":  plugin,
		"messageID": message,
		"data":      data,
	})
	if err != nil {
		panic(err)
	}
	return string(body)
}

func TestPanelsOpModeListIsRead(t *testing.T) {
	body := frame(opModeControlPlugin, "opModesList", map[string]any{
		"opModes": []map[string]any{
			{"name": "TestTeleop", "group": "", "flavour": "TELEOP", "source": "ANDROID_STUDIO"},
			{"name": "CloseBlue", "group": "auto", "flavour": "AUTONOMOUS", "source": "ANDROID_STUDIO"},
		},
	})

	var got panelsFrame
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatal(err)
	}
	if got.PluginID != opModeControlPlugin || got.MessageID != "opModesList" {
		t.Fatalf("frame did not round trip: %+v", got)
	}

	var list panelsOpModes
	if err := json.Unmarshal(got.Data, &list); err != nil {
		t.Fatal(err)
	}

	names := Names(list.OpModes)
	if len(names) != 2 || names[0] != "TestTeleop" || names[1] != "CloseBlue" {
		t.Errorf("read %v", names)
	}
	if list.OpModes[1].Group != "auto" {
		t.Errorf("group did not survive: %q", list.OpModes[1].Group)
	}
}

// Panels reports Class.getName(), which carries the package. The source side
// produces the simple name, so keeping the package would make every tunable
// look like one the project never declared.
func TestPanelsValuesAreKeyedTheWayTheSourceIs(t *testing.T) {
	fields := map[string][]map[string]any{
		"org.firstinspires.ftc.teamcode.Tuning": {
			{"className": "org.firstinspires.ftc.teamcode.Tuning", "fieldName": "kP",
				"type": "DOUBLE", "value": "0.0130", "customValues": []any{}},
			{"className": "org.firstinspires.ftc.teamcode.Tuning", "fieldName": "useVision",
				"type": "BOOLEAN", "value": "true", "customValues": []any{}},
			{"className": "org.firstinspires.ftc.teamcode.Tuning", "fieldName": "label",
				"type": "STRING", "value": "blue", "customValues": []any{}},
			{"className": "org.firstinspires.ftc.teamcode.Tuning", "fieldName": "mode",
				"type": "ENUM", "value": "FAST", "customValues": []any{}},
		},
	}

	values := Values{}
	var byClass map[string][]panelsField

	body, _ := json.Marshal(fields)
	if err := json.Unmarshal(body, &byClass); err != nil {
		t.Fatal(err)
	}
	for _, list := range byClass {
		for _, field := range list {
			flattenPanels(field, "", values)
		}
	}

	want := map[string]string{
		"Tuning.kP":        "0.013",
		"Tuning.useVision": "true",
		"Tuning.label":     `"blue"`,
		"Tuning.mode":      "FAST",
	}

	for key, expect := range want {
		if got := values[key]; got != expect {
			t.Errorf("%s = %q, want %q", key, got, expect)
		}
	}
	if len(values) != len(want) {
		t.Errorf("read %d values, want %d: %v", len(values), len(want), values)
	}
}

// A number from Panels arrives as whatever the double prints as, and a number
// in the source is whatever somebody typed. Comparing those as raw text reports
// every field as changed, which is the failure the FtcDashboard reader already
// avoids and the Panels one has to avoid the same way.
func TestPanelsNumbersCompareAgainstSource(t *testing.T) {
	field := panelsField{ClassName: "Tuning", FieldName: "kP", Type: "DOUBLE", Value: "0.50"}

	values := Values{}
	flattenPanels(field, "", values)

	if got, want := values["Tuning.kP"], Normalise(".5"); got != want {
		t.Errorf("robot says %q and source says %q, so an untouched field reads as tuning", got, want)
	}
}

// Types with no literal in the source have nothing to compare against, and
// reporting them as changed on every run is worse than leaving them out.
func TestPanelsSkipsWhatCannotBeCompared(t *testing.T) {
	values := Values{}

	for _, kind := range []string{"LIST", "MAP", "ARRAY", "UNKNOWN", "UNSUPPORTED", "ERROR"} {
		flattenPanels(panelsField{ClassName: "Tuning", FieldName: "x", Type: kind, Value: "?"}, "", values)
	}

	if len(values) != 0 {
		t.Errorf("recorded values that cannot be compared: %v", values)
	}
}

// A compound tunable arrives with its parts nested underneath it, and the parts
// are the comparable half.
func TestPanelsReadsNestedFields(t *testing.T) {
	field := panelsField{
		ClassName: "org.firstinspires.ftc.teamcode.Tuning",
		FieldName: "pid",
		Type:      "CUSTOM",
		Custom: []panelsField{
			{FieldName: "p", Type: "DOUBLE", Value: "1.0"},
			{FieldName: "i", Type: "DOUBLE", Value: "0"},
		},
	}

	values := Values{}
	flattenPanels(field, "", values)

	if got := values["Tuning.pid.p"]; got != "1" {
		t.Errorf("Tuning.pid.p = %q", got)
	}
	if _, recorded := values["Tuning.pid"]; recorded {
		t.Error("recorded the compound itself, which has no initialiser to compare against")
	}
}

// Panels marks a tunable class with @Configurable and no name. This worked by
// accident, @Config being a prefix of it, which is not the same as working.
func TestConfigurableClassesAreRead(t *testing.T) {
	source := `package org.firstinspires.ftc.teamcode;

import com.bylazar.configurables.annotations.Configurable;

@Configurable
public class Tuning {
    public static double kP = 0.013;
    public static boolean useVision = true;
}
`

	fields := FromFile("Tuning.java", source)
	if len(fields) != 2 {
		t.Fatalf("read %d fields from an @Configurable class, want 2: %+v", len(fields), fields)
	}
	if fields[0].Key() != "Tuning.kP" {
		t.Errorf("key = %q, want Tuning.kP", fields[0].Key())
	}
}

// Reading off a real socket, not just out of a string. Panels volunteers its
// messages the way FtcDashboard does, so the same fake server serves both, and
// what is being tested here is that the reader picks its own message out of
// everything else arriving on that socket.
func TestPanelsOpModesOffTheWire(t *testing.T) {
	addr := fakeDashboard(t,
		frame("com.bylazar.telemetry", "telemetry", map[string]any{"lines": []string{"x"}}),
		frame(configurablesPlugin, "configurables", map[string]any{}),
		frame(opModeControlPlugin, "activeOpMode", map[string]any{"status": "STOPPED"}),
		frame(opModeControlPlugin, "opModesList", map[string]any{
			"opModes": []map[string]any{{"name": "TestTeleop", "group": ""}},
		}),
	)

	modes, err := PanelsOpModes(addr)
	if err != nil {
		t.Fatalf("PanelsOpModes: %v", err)
	}
	if len(modes) != 1 || modes[0].Name != "TestTeleop" {
		t.Errorf("read %+v", modes)
	}
}

func TestPanelsValuesOffTheWire(t *testing.T) {
	addr := fakeDashboard(t,
		frame(opModeControlPlugin, "opModesList", map[string]any{"opModes": []any{}}),
		frame(configurablesPlugin, "initialConfigurables", map[string]any{}),
		frame(configurablesPlugin, "configurables", map[string][]map[string]any{
			"org.firstinspires.ftc.teamcode.Tuning": {
				{"className": "org.firstinspires.ftc.teamcode.Tuning", "fieldName": "kP",
					"type": "DOUBLE", "value": "0.013"},
			},
		}),
	)

	values, err := PanelsFetch(addr)
	if err != nil {
		t.Fatalf("PanelsFetch: %v", err)
	}
	if got := values["Tuning.kP"]; got != "0.013" {
		t.Errorf("Tuning.kP = %q, want 0.013", got)
	}
}

// A robot running FtcDashboard must not have its frames read as Panels ones,
// and the reverse, or a reader would report the other dashboard's silence as an
// empty robot.
func TestNeitherReaderAnswersToTheOther(t *testing.T) {
	ftc := fakeDashboard(t, `{"type":"RECEIVE_OP_MODE_LIST","opModeInfoList":[{"name":"TestTeleop"}]}`)
	if _, err := PanelsOpModes(ftc); err == nil {
		t.Error("the Panels reader accepted an FtcDashboard message")
	}

	panels := fakeDashboard(t, frame(opModeControlPlugin, "opModesList", map[string]any{
		"opModes": []map[string]any{{"name": "TestTeleop"}},
	}))
	if _, err := OpModes(panels); err == nil {
		t.Error("the FtcDashboard reader accepted a Panels message")
	}
}
