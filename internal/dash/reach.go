package dash

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/andreibanu/pusher/internal/adb"
)

// The dashboard listens on the robot's own network interface, so over Wi-Fi it
// is simply reachable. Over USB it is not, and adb forwards a local port to it
// instead. That is the difference between this working when plugged in and only
// working when on the robot's network.

// forwardPort is the local end of a USB forward. High enough to be free, and
// fixed so a leaked forward is reused rather than accumulating.
//
// One local port per robot port, offset the same way, so forwarding to the two
// dashboards at once does not have them fighting over the same local end.
const forwardPort = 28000

func localFor(port int) int { return forwardPort + port - Port }

// Reach is an open route to the dashboard.
type Reach struct {
	// Addr is host:port to connect to.
	Addr string

	forwarded bool
	serial    string
	local     int
}

// Close takes down a forward, if one was set up.
func (r Reach) Close() {
	if r.forwarded {
		_ = exec.Command("adb", "-s", r.serial, "forward", "--remove",
			"tcp:"+strconv.Itoa(r.local)).Run()
	}
}

// Open works out how to reach FtcDashboard on the connected robot.
func Open(serial string) (Reach, error) { return OpenPort(serial, Port) }

// OpenPort is Open for a given port on the robot, because a team runs one
// dashboard or the other and they do not listen on the same one.
func OpenPort(serial string, port int) (Reach, error) {
	if serial == "" {
		return Reach{}, fmt.Errorf("no robot connected")
	}

	// A Wi-Fi serial is already an address, and the dashboard is on the same
	// host at its own port.
	if host, _, found := strings.Cut(serial, ":"); found && host != "" {
		return Reach{Addr: fmt.Sprintf("%s:%d", host, port)}, nil
	}

	local := localFor(port)
	out, err := exec.Command("adb", "-s", serial, "forward", "tcp:"+strconv.Itoa(local),
		"tcp:"+strconv.Itoa(port)).CombinedOutput()
	if err != nil {
		return Reach{}, fmt.Errorf("cannot forward a port to the dashboard: %s",
			strings.TrimSpace(string(out)))
	}

	return Reach{
		Addr:      fmt.Sprintf("127.0.0.1:%d", local),
		forwarded: true,
		serial:    serial,
		local:     local,
	}, nil
}

// Read opens a route to the connected robot and fetches what its dashboard
// holds, whichever one that is.
//
// FtcDashboard first, then Panels. A team runs one or the other, and which one
// is not something the laptop can tell without asking.
func Read(serial string) (Values, Dashboard, error) {
	values, err := ReadFrom(serial, Port)
	if err == nil {
		return values, FtcDashboard, nil
	}

	values, panelsErr := ReadFrom(serial, PanelsPort)
	if panelsErr == nil {
		return values, PanelsDash, nil
	}

	return nil, None, fmt.Errorf("%w\n    Is FtcDashboard or Panels running? Either needs the robot app started", err)
}

// ReadFrom fetches from one dashboard, named by its port.
func ReadFrom(serial string, port int) (Values, error) {
	route, err := OpenPort(serial, port)
	if err != nil {
		return nil, err
	}
	defer route.Close()

	if port == PanelsPort {
		return PanelsFetch(route.Addr)
	}
	return Fetch(route.Addr)
}

// Robot is the connected robot, or an error saying there is not one.
func Robot() (string, error) { return adb.Target() }
