package dash

import (
	"encoding/json"
	"fmt"
	"time"
)

// The robot volunteers its OpMode list: there is no message asking for one, and
// FtcDashboard sends RECEIVE_OP_MODE_LIST when a client connects and again
// whenever the list changes. So this connects and waits rather than asks.
//
// The list comes from RegisteredOpModes.getOpModes(), which is the same thing
// the Driver Station shows. That makes it the only way to find out from a
// laptop what the robot actually registered, as opposed to what it was sent.

// ListTimeout is shorter than Timeout: this runs after every reload, and a
// robot with no dashboard on it must cost the deploy as little as possible.
const ListTimeout = 3 * time.Second

// OpModes is what the robot currently has registered.
func OpModes(addr string) ([]OpMode, error) {
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

		var wrapper envelope
		if json.Unmarshal([]byte(message), &wrapper) != nil {
			continue
		}
		if wrapper.Type != "RECEIVE_OP_MODE_LIST" {
			continue
		}

		return wrapper.OpModeInfoList, nil
	}

	return nil, fmt.Errorf("the dashboard never sent its OpMode list")
}

// Registered opens a route to the connected robot and asks what it has.
func Registered(serial string) ([]OpMode, error) {
	route, err := Open(serial)
	if err != nil {
		return nil, err
	}
	defer route.Close()

	return OpModes(route.Addr)
}

// Names is every registered OpMode name.
func Names(modes []OpMode) []string {
	out := make([]string, 0, len(modes))
	for _, mode := range modes {
		out = append(out, mode.Name)
	}
	return out
}
