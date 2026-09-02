package robot

import (
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/wifi"
)

// Switching networks to reach the robot is the thing everybody puts up with and
// nobody wants: the laptop leaves whatever it was on, the robot's access point
// has no route anywhere, and getting back is another wait at the other end.
//
// None of that is necessary when the robot is already somewhere the laptop can
// reach. A hub that has joined the same network is just a host on it, and adb
// does not care which network a device is on as long as it can open a socket to
// it. So this looks for the robot where the laptop already is, and only falls
// back to taking over the radio when it is not there.
//
// Inspired by Dhruv, FTC 32001L, whose ADB relay bridged adb from the robot's
// access point to the local network with a Linux box in between. The want is
// theirs; this goes at it from the other end.

// AdbPort is where a robot listens for adb over the network.
const AdbPort = 5555

// dialTimeout is how long one host gets to answer. A robot on the same network
// answers in single digit milliseconds; this is generous for a busy one and
// short enough that a sweep of a full subnet stays under a couple of seconds.
const dialTimeout = 300 * time.Millisecond

// sweepWorkers is how many hosts are tried at once.
const sweepWorkers = 64

// maxSweep is the largest subnet worth sweeping. A /24 is 254 hosts, which is
// what a home or venue network almost always is. Anything wider is a network
// nobody wants pusher opening ten thousand sockets on.
const maxSweep = 1024

// Found is a robot pusher located on the network.
type Found struct {
	Addr  string
	Model string
}

// Locate looks for the robot on the network the laptop is already on.
//
// Cheapest first: where it was last seen, then the hub's own access point,
// then a sweep of the local subnets. The first two are one socket each, so the
// common case of "it is where it was yesterday" costs nothing worth measuring.
func Locate(out io.Writer, sweep bool) (Found, error) {
	if !adb.IsInstalled() {
		return Found{}, fmt.Errorf("%w - install Android SDK Platform-Tools", adb.ErrNoADB)
	}

	if remembered := config.GetRobotAddress(); remembered != "" {
		if found, ok := check(remembered); ok {
			adb.UseAddress(found.Addr)
			return found, nil
		}
		fmt.Fprintf(out, "[*] Not at %s any more, looking further\n", remembered)
	}

	if found, ok := check(adb.RobotIP); ok {
		adb.UseAddress(found.Addr)
		return found, nil
	}

	if !sweep {
		return Found{}, fmt.Errorf("the robot is not where it was last seen, and not on its own access point")
	}

	hosts := localHosts()
	if len(hosts) == 0 {
		return Found{}, fmt.Errorf("this laptop is not on a network with room to look")
	}

	fmt.Fprintf(out, "[*] Looking for the robot across %d addresses...\n", len(hosts))

	for _, addr := range listening(hosts) {
		if found, ok := check(addr); ok {
			adb.UseAddress(found.Addr)
			return found, nil
		}
	}

	return Found{}, fmt.Errorf("no robot answered on this network.\n" +
		"    It has to have joined this network itself, and have adb over Wi-Fi turned on")
}

// Remember writes down where the robot was, so the next run finds it in one
// socket instead of a sweep.
//
// Labelled with the network the laptop was on, which is the one pusher can know
// and the one somebody is choosing between: the shop, the venue, home.
func Remember(found Found) {
	network := "this network"
	if ssid, err := wifi.NewManager().CurrentSSID(); err == nil && ssid != "" {
		network = ssid
	}

	_ = config.RememberSpot(network, found.Addr)
}

// Try points pusher at a remembered address, if the robot is still there.
func Try(addr string) (Found, bool) {
	found, ok := check(addr)
	if !ok {
		return Found{}, false
	}

	adb.UseAddress(found.Addr)
	return found, true
}

// check asks whether there is a robot at an address, and connects to it.
//
// Two questions rather than one. Something answering on the adb port is not a
// robot: it could be a phone, a tablet, or a teammate's laptop with adb left
// running, and pusher installing an FTC app onto one of those is a worse
// outcome than not finding the robot at all.
func check(host string) (Found, bool) {
	addr := withPort(host)

	if !reachable(addr) {
		return Found{}, false
	}
	if err := adb.ConnectAt(io.Discard, addr); err != nil {
		return Found{}, false
	}

	model, ok := isRobot(addr)
	if !ok {
		// Left as it was found. A device that answered and turned out to be
		// somebody's phone should not be left attached to this adb server.
		_ = adb.DisconnectFrom(addr)
		return Found{}, false
	}

	return Found{Addr: addr, Model: model}, true
}

// isRobot reports whether the device at addr is one, and what it calls itself.
//
// A Control Hub says so in its model. A phone running the robot controller does
// not, so the app itself is the second question: either answer means this is a
// robot, and neither means it is somebody else's device.
func isRobot(addr string) (string, bool) {
	model, err := adb.Shell(addr, "getprop", "ro.product.model")
	if err != nil {
		return "", false
	}
	model = strings.TrimSpace(model)

	if strings.Contains(strings.ToLower(model), "control hub") {
		return model, true
	}

	out, err := adb.Shell(addr, "pm", "list", "packages", "com.qualcomm.ftcrobotcontroller", "2>/dev/null", "||", "true")
	if err == nil && strings.Contains(out, "com.qualcomm.ftcrobotcontroller") {
		if model == "" {
			model = "robot controller"
		}
		return model, true
	}

	return "", false
}

// reachable reports whether anything is listening for adb at addr.
func reachable(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func withPort(host string) string {
	if strings.Contains(host, ":") {
		return host
	}
	return fmt.Sprintf("%s:%d", host, AdbPort)
}

// listening returns the hosts with the adb port open, in the order they answered.
func listening(hosts []string) []string {
	var (
		mu   sync.Mutex
		open []string
		wg   sync.WaitGroup
	)

	queue := make(chan string)

	for i := 0; i < sweepWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for host := range queue {
				if !reachable(withPort(host)) {
					continue
				}
				mu.Lock()
				open = append(open, host)
				mu.Unlock()
			}
		}()
	}

	for _, host := range hosts {
		queue <- host
	}
	close(queue)
	wg.Wait()

	return open
}

// localHosts is every address on the subnets this laptop is on, excluding its
// own.
func localHosts() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	seen := map[string]bool{}
	var hosts []string

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			net4, ok := addr.(*net.IPNet)
			if !ok || net4.IP.To4() == nil {
				continue
			}

			for _, host := range subnet(net4) {
				if !seen[host] {
					seen[host] = true
					hosts = append(hosts, host)
				}
			}
		}
	}

	return hosts
}

// subnet is every usable address in one network, or nothing when the network is
// too wide to be worth walking.
func subnet(n *net.IPNet) []string {
	ones, bits := n.Mask.Size()
	if bits != 32 || ones < 32-10 {
		return nil
	}

	size := 1 << uint(bits-ones)
	if size > maxSweep {
		return nil
	}

	base := n.IP.Mask(n.Mask).To4()
	if base == nil {
		return nil
	}

	mine := n.IP.To4().String()

	var out []string
	for i := 1; i < size-1; i++ {
		ip := make(net.IP, 4)
		copy(ip, base)

		ip[3] += byte(i)
		ip[2] += byte(i >> 8)

		if text := ip.String(); text != mine {
			out = append(out, text)
		}
	}

	return out
}
