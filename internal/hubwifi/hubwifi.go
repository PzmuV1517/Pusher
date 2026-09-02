// Package hubwifi puts a Control Hub onto a network of your own, over a USB
// Wi-Fi adapter, without disturbing the access point it serves to the Driver
// Station.
//
// The hub's own radio is its access point and cannot join anything: the SDK has
// no client mode for it, and RobotControllerAccessPointAssistant has no connect
// method at all. A second radio is a different question, and the answer is a
// USB adapter whose chipset the hub's kernel has a driver for.
//
// What follows is the sequence that works, and every step of it is here because
// the obvious version of that step does not. Proven on a REV Control Hub v1.0
// running Control Hub OS 1.1.6 with a TP-Link Archer T4U v3.
package hubwifi

import (
	"fmt"
	"io"
	"strings"

	"github.com/andreibanu/pusher/internal/adb"
)

// Iface is the interface a USB adapter comes up as. wlan0 is the hub's own
// radio, running the access point.
const Iface = "wlan1"

// CtrlDir is where wpa_supplicant is told to put its control socket.
//
// Android's own directory, not one of ours. It exists with the right ownership
// already, and a supplicant asked to create its own under /data/local/tmp
// starts, answers wpa_cli, and never scans.
const CtrlDir = "/data/misc/wifi/sockets"

// Where the pieces live on the hub. Under /data, which survives a reboot and is
// writable, rather than /system, which is neither.
const (
	Dir        = "/data/local/tmp/pusher-wifi"
	ModulePath = Dir + "/wifi.ko"
	ConfPath   = Dir + "/wpa.conf"
	LeasePath  = Dir + "/lease.sh"
)

// LocalNetworkTable is the routing table Android files its own networks under.
//
// The number matters. Android routes by policy rather than through one table,
// and an interface it does not manage appears in none of the rules: a reply
// from an ordinary process carries fwmark 0, matches "lookup local_network",
// finds nothing, and falls through to the unreachable catch-all. So the routes
// go where the rules already look. Adding a rule instead does not work unless
// it is given an explicit priority, because a bare `ip rule add` lands after
// that catch-all and is never reached.
const LocalNetworkTable = 97

// What a scan can fail with, so a caller can tell a hub with no adapter from a
// radio that would not answer.
var (
	errNoAdapter = fmt.Errorf("no %s on the robot, so there is no radio to look with.\n"+
		"    Plug the adapter in and run the setup once, which loads the driver", Iface)
	errScanFailed = fmt.Errorf("the robot's adapter would not scan")
)

// Options is what to join and with what.
type Options struct {
	SSID     string
	Password string

	// Module is a driver to load first, when the hub has no interface yet.
	// Empty means the adapter is expected to be working already.
	Module string

	// Persist installs the boot hook, so the hub rejoins on every power cycle.
	Persist bool
}

// Result is where the hub ended up.
type Result struct {
	Address string
	Gateway string
}

// runner runs one command on the hub, as root.
type runner struct {
	serial string
	root   string
	out    io.Writer
}

// run runs one command on the hub and keeps what it said, succeed or fail.
//
// What a refusing command prints is the only thing that explains it, and it
// prints that on the way to exiting non-zero.
func (r runner) run(args ...string) (string, error) {
	cmd := append(strings.Fields(r.root), args...)
	return adb.ShellOutput(r.serial, cmd...)
}

func (r runner) quiet(args ...string) {
	_, _ = r.run(args...)
}

func (r runner) say(format string, args ...any) {
	fmt.Fprintf(r.out, format+"\n", args...)
}

// rootPrefix works out how to become root on this hub.
//
// Three forms exist and hubs differ. AOSP's su takes a uid as its first
// argument and has no -c at all, which is what "invalid uid/gid '-c'" means on
// one that does. Confirmed by asking who we are rather than by whether the
// command appeared to work.
func rootPrefix(serial string) (string, error) {
	for _, prefix := range []string{"", "su 0", "su root", "su -c"} {
		args := append(strings.Fields(prefix), "id", "-u")

		out, err := adb.Shell(serial, args...)
		if err == nil && strings.TrimSpace(out) == "0" {
			return prefix, nil
		}
	}

	return "", fmt.Errorf("cannot become root on this robot, so its network cannot be changed")
}
