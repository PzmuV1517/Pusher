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

// Which dashboard answered. Worth carrying, because a message about what the
// robot did or did not register reads very differently depending on which one
// was asked, and a team runs one or the other.
type Dashboard string

const (
	// None means neither answered.
	None Dashboard = ""
	// FtcDashboard is the ACME one, on port 8000.
	FtcDashboard Dashboard = "FtcDashboard"
	// PanelsDash is Panels, on 8002.
	PanelsDash Dashboard = "Panels"
)

// Registered opens a route to the connected robot and asks whichever dashboard
// it is running what it has.
//
// Both are tried because pusher cannot tell from the laptop which one is on the
// robot, and asking the wrong one is indistinguishable from the robot having
// registered nothing. That mattered: the one report of a reload registering
// nothing came from a project running Panels, where this check had nothing to
// say and a deploy that had emptied the OpMode list printed success.
//
// Neither answering costs two refused connections rather than two timeouts:
// nothing listening is a refusal, and the robot's own network does not drop it.
func Registered(serial string) ([]OpMode, Dashboard, error) {
	modes, err := RegisteredOn(serial, Port)
	if err == nil {
		return modes, FtcDashboard, nil
	}

	modes, panelsErr := RegisteredOn(serial, PanelsPort)
	if panelsErr == nil {
		return modes, PanelsDash, nil
	}

	return nil, None, fmt.Errorf("no dashboard answered on the robot: %w", err)
}

// RegisteredOn asks one dashboard, named by its port.
func RegisteredOn(serial string, port int) ([]OpMode, error) {
	route, err := OpenPort(serial, port)
	if err != nil {
		return nil, err
	}
	defer route.Close()

	if port == PanelsPort {
		return PanelsOpModes(route.Addr)
	}
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
