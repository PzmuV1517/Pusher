package dash

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The dashboard answers GET_CONFIG with the whole tree it holds, as nested
// nodes tagged with their type. Classes are "custom" and nest; everything else
// is a leaf. Read out of the client bundled in the dashboard AAR, which is the
// only description of the protocol there is.

// Timeout is how long to wait for the dashboard, in total.
const Timeout = 6 * time.Second

// node is one entry in the config tree.
type node struct {
	Type  string          `json:"__type"`
	Value json.RawMessage `json:"__value"`
}

// envelope is every message the dashboard sends.
//
// The payload sits beside the type rather than under a "data" key: the robot
// serialises each message class straight out, so the field is named after
// whatever that class calls it. Read out of the client bundled in the AAR,
// which reads t.configRoot and t.opModeInfoList directly off the message.
type envelope struct {
	Type string `json:"type"`

	// ConfigRoot is the tree on a RECEIVE_CONFIG.
	ConfigRoot json.RawMessage `json:"configRoot"`

	// OpModeInfoList is what the robot has registered, on a
	// RECEIVE_OP_MODE_LIST.
	OpModeInfoList []OpMode `json:"opModeInfoList"`
}

// OpMode is one entry in the robot's own OpMode list.
type OpMode struct {
	Name  string `json:"name"`
	Group string `json:"group"`
}

// Values is every tunable the dashboard holds, keyed by "Class.field".
type Values map[string]string

// Names are the keys in order.
func (v Values) Names() []string {
	out := make([]string, 0, len(v))
	for name := range v {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Fetch asks the dashboard at addr for everything it currently holds.
func Fetch(addr string) (Values, error) {
	socket, err := dial(addr, Timeout)
	if err != nil {
		return nil, err
	}
	defer socket.Close()

	deadline := time.Now().Add(Timeout)
	socket.deadline(deadline)

	if err := socket.send(`{"type":"GET_CONFIG"}`); err != nil {
		return nil, err
	}

	// Telemetry, gamepad state and images all arrive on this socket whether or
	// not anything asked, so the answer has to be waited for rather than simply
	// read.
	for time.Now().Before(deadline) {
		message, err := socket.receive()
		if err != nil {
			return nil, err
		}

		var wrapper envelope
		if json.Unmarshal([]byte(message), &wrapper) != nil {
			continue
		}
		if wrapper.Type != "RECEIVE_CONFIG" {
			continue
		}

		var root node
		if err := json.Unmarshal(wrapper.ConfigRoot, &root); err != nil {
			return nil, fmt.Errorf("cannot read the dashboard's config: %w", err)
		}

		values := Values{}
		flatten("", root, values)
		return values, nil
	}

	return nil, fmt.Errorf("the dashboard never sent its config")
}

// flatten walks the tree into "Class.field" keys.
//
// Only leaves are recorded. A class with no tunables in it says nothing, and an
// empty section is not a value anybody wants reported.
func flatten(prefix string, n node, into Values) {
	if n.Type != "custom" {
		if text, ok := literal(n); ok && prefix != "" {
			into[prefix] = text
		}
		return
	}

	var children map[string]node
	if json.Unmarshal(n.Value, &children) != nil {
		return
	}

	for name, child := range children {
		next := name
		if prefix != "" {
			next = prefix + "." + name
		}
		flatten(next, child, into)
	}
}

// literal renders a leaf the way the source would write it, so a value from the
// robot and a value read out of a .java file can be compared as text.
func literal(n node) (string, bool) {
	var raw any
	if json.Unmarshal(n.Value, &raw) != nil {
		return "", false
	}

	switch value := raw.(type) {
	case bool:
		return strconv.FormatBool(value), true

	case float64:
		return Number(strconv.FormatFloat(value, 'f', -1, 64)), true

	case string:
		// Enums come across as their constant name and strings as their
		// contents. Only the type says which, and only a string gets quoted.
		if n.Type == "enum" {
			return value, true
		}
		return strconv.Quote(value), true
	}

	return "", false
}

// Number puts a numeric literal into one shape, so that 0.5, .5, 0.50 and 0.5d
// all compare equal.
//
// Java writes what a person typed and the dashboard writes what the double
// holds, and comparing those as text reports every field as changed.
func Number(text string) string {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "_", "")

	sign := ""
	if len(text) > 0 && (text[0] == '-' || text[0] == '+') {
		if text[0] == '-' {
			sign = "-"
		}
		text = text[1:]
	}

	// A base prefix means the digits are not decimal, and in hex d and f are
	// digits rather than the type suffix they are everywhere else.
	if based := strings.HasPrefix(strings.ToLower(text), "0x") ||
		strings.HasPrefix(strings.ToLower(text), "0b"); based {
		trimmed := strings.TrimRight(text, "lL")
		if value, err := strconv.ParseInt(sign+trimmed, 0, 64); err == nil {
			return strconv.FormatInt(value, 10)
		}
		return sign + text
	}

	// Type suffixes are part of the source, not of the value.
	if len(text) > 0 && strings.ContainsAny(text[len(text)-1:], "fFdDlL") {
		text = text[:len(text)-1]
	}

	value, err := strconv.ParseFloat(sign+text, 64)
	if err != nil {
		return sign + text
	}

	return strconv.FormatFloat(value, 'f', -1, 64)
}
