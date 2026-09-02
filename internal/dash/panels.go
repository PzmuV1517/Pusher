package dash

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Panels is the other dashboard, and a team runs one or the other. Reading only
// FtcDashboard meant the check that a reload actually registered anything was
// silent for everybody on Panels, which is exactly the team it had to work for:
// the one report of a reload registering nothing came from a project running
// Panels, and pusher had nothing to say about it.
//
// Read out of the published AAR rather than from documentation. Panels serves
// its web UI on 8001 and a WebSocket on 8002 (Panels.kt), and every frame is
// {pluginID, messageID, data} (SocketMessage). The OpMode list arrives without
// being asked for: Socket$ClientSocket.onOpen calls every plugin's newClient
// hook, and com.bylazar.opmodecontrol's sends opModesList straight away. So
// this connects and waits, the same shape as the FtcDashboard reader.

// PanelsPort is the Panels WebSocket on the robot. Its web UI is on 8001,
// which is not this.
const PanelsPort = 8002

// opModeControlPlugin is the Panels plugin that owns the OpMode list. It is
// distributed separately from Panels itself, so a project can have Panels and
// still never send one of these.
const opModeControlPlugin = "com.bylazar.opmodecontrol"

// panelsFrame is one message off the Panels socket.
type panelsFrame struct {
	PluginID  string          `json:"pluginID"`
	MessageID string          `json:"messageID"`
	Data      json.RawMessage `json:"data"`
}

// panelsOpModes is the payload of an opModesList.
//
// OpMode is reused for the entries: Panels calls the type OpModeDetails and
// carries four more fields, but it names the two that matter identically, and
// the two dashboards agreeing on the wire is worth more than a second type.
type panelsOpModes struct {
	OpModes []OpMode `json:"opModes"`
}

// PanelsOpModes is what Panels says the robot has registered.
func PanelsOpModes(addr string) ([]OpMode, error) {
	socket, err := dial(addr, ListTimeout)
	if err != nil {
		return nil, err
	}
	defer socket.Close()

	deadline := time.Now().Add(ListTimeout)
	socket.deadline(deadline)

	for time.Now().Before(deadline) {
		message, err := socket.receive()
		if err != nil {
			return nil, err
		}

		var frame panelsFrame
		if json.Unmarshal([]byte(message), &frame) != nil {
			continue
		}
		if frame.PluginID != opModeControlPlugin || frame.MessageID != "opModesList" {
			continue
		}

		var list panelsOpModes
		if err := json.Unmarshal(frame.Data, &list); err != nil {
			return nil, fmt.Errorf("panels sent an OpMode list that could not be read: %w", err)
		}

		return list.OpModes, nil
	}

	return nil, fmt.Errorf("panels never sent its OpMode list")
}

// The tuning half of Panels lives in a second plugin again, and its wire types
// read cleanly onto the model already here: every field arrives as
// GenericTypeJson {className, fieldName, type, value, customValues}, grouped by
// class. So this flattens into the same "Class.field" keys FtcDashboard's tree
// produces, and the comparison against source does not have to know which
// dashboard answered.

// configurablesPlugin is the Panels plugin that owns the tunables. Separate
// from the OpMode list plugin, so a project can have one and not the other.
const configurablesPlugin = "com.bylazar.configurables"

// panelsField is one tunable as Panels reports it.
type panelsField struct {
	ClassName string        `json:"className"`
	FieldName string        `json:"fieldName"`
	Type      string        `json:"type"`
	Value     string        `json:"value"`
	Custom    []panelsField `json:"customValues"`
}

// PanelsFetch asks Panels at addr for everything it currently holds.
//
// Waited for rather than requested. Panels sends the whole set to every client
// that connects, and there is no message that asks for one.
func PanelsFetch(addr string) (Values, error) {
	socket, err := dial(addr, Timeout)
	if err != nil {
		return nil, err
	}
	defer socket.Close()

	deadline := time.Now().Add(Timeout)
	socket.deadline(deadline)

	for time.Now().Before(deadline) {
		message, err := socket.receive()
		if err != nil {
			return nil, err
		}

		var frame panelsFrame
		if json.Unmarshal([]byte(message), &frame) != nil {
			continue
		}
		if frame.PluginID != configurablesPlugin || frame.MessageID != "configurables" {
			continue
		}

		var byClass map[string][]panelsField
		if err := json.Unmarshal(frame.Data, &byClass); err != nil {
			return nil, fmt.Errorf("cannot read what Panels holds: %w", err)
		}

		values := Values{}
		for _, fields := range byClass {
			for _, field := range fields {
				flattenPanels(field, "", values)
			}
		}
		return values, nil
	}

	return nil, fmt.Errorf("panels never sent its configurables")
}

// flattenPanels records one field and whatever hangs off it.
func flattenPanels(field panelsField, prefix string, into Values) {
	key := prefix + "." + field.FieldName
	if prefix == "" {
		key = simpleName(field.ClassName) + "." + field.FieldName
	}

	if text, ok := panelsLiteral(field); ok {
		into[key] = text
	}

	for _, child := range field.Custom {
		flattenPanels(child, key, into)
	}
}

// panelsLiteral renders a value the way the source would write it, matching
// what the FtcDashboard reader does so that either dashboard's answer can be
// compared against the same .java file.
//
// Only the types a source file states as a literal. A list or a map has no
// initialiser to compare against, and reporting one as changed every time is
// worse than leaving it out.
func panelsLiteral(field panelsField) (string, bool) {
	switch field.Type {
	case "BOOLEAN", "ENUM":
		return field.Value, true
	case "INT", "LONG", "DOUBLE", "FLOAT":
		return Number(field.Value), true
	case "STRING":
		return strconv.Quote(field.Value), true
	}
	return "", false
}

// simpleName is the class name without its package, because that is what a
// dashboard section is called and what the source side produces.
//
// Panels reports Class.getName(), which is fully qualified, where FtcDashboard
// reports the simple name. Keeping the package would make every field look
// like one the source never declared.
func simpleName(class string) string {
	if i := strings.LastIndexAny(class, ".$"); i >= 0 {
		return class[i+1:]
	}
	return class
}

// PanelsValues opens a route to the connected robot and asks Panels.
func PanelsValues(serial string) (Values, error) {
	route, err := OpenPort(serial, PanelsPort)
	if err != nil {
		return nil, err
	}
	defer route.Close()

	return PanelsFetch(route.Addr)
}
