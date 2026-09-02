package robot

import (
	"fmt"
	"net"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/andreibanu/pusher/internal/adb"
)

// Everything that talks to the robot does it over a port, and every one of them
// is a different number somebody has to remember or go and look up. Worse, the
// answer changes: on the hub's own network they all live at 192.168.43.1, and
// on a network the robot has joined they are wherever it landed.
//
// So this asks the robot where it is and then knocks on every door, and reports
// only what actually answered. An address pusher prints is one somebody can
// paste into a browser, not one it believes should work.
//
// Ports read out of the published artifacts rather than from memory:
// FtcDashboard on 8000, Panels on 8001 with its socket on 8002 (Panels.kt), and
// the Panels Limelight proxy on 5800, 5801 and 5805, which it forwards to the
// Limelight itself at 172.29.0.1.

// Service is one thing the robot serves.
type Service struct {
	Name string
	Port int

	// Scheme is empty for things that are not web pages, which are worth
	// reporting as reachable but not as a link.
	Scheme string

	// Note says what it is for, when the name does not.
	Note string

	Reachable bool
}

// URL is the address to open, empty when this is not something a browser opens.
func (s Service) URL(host string) string {
	if s.Scheme == "" || host == "" {
		return ""
	}
	return fmt.Sprintf("%s://%s:%d", s.Scheme, host, s.Port)
}

// services is everything worth knocking on.
var services = []Service{
	{Name: "adb", Port: AdbPort, Note: "what pusher deploys over"},
	{Name: "Robot Controller", Port: 8080, Scheme: "http", Note: "the hub's own manage page"},
	{Name: "FtcDashboard", Port: 8000, Scheme: "http", Note: "tuning and telemetry"},
	{Name: "Panels", Port: 8001, Scheme: "http", Note: "tuning and telemetry"},
	{Name: "Panels socket", Port: 8002, Note: "what `pusher dash diff` reads"},
	{Name: "Limelight", Port: 5801, Scheme: "http", Note: "through the Panels proxy"},
	{Name: "Limelight stream", Port: 5800, Scheme: "http", Note: "through the Panels proxy"},
	{Name: "Limelight socket", Port: 5805, Note: "through the Panels proxy"},
}

// Survey is where the robot is and what it is currently serving.
type Survey struct {
	Serial   string
	Host     string
	Hostname string
	Model    string

	// Addresses is every address the robot has, as the robot itself reports
	// them, which is the only place the truth lives when pusher reached it over
	// USB and has no idea what network it is on.
	Addresses []string

	// Local is the .local name, when this network actually resolves one.
	Local string

	Services []Service
}

// Take asks the robot where it is and knocks on every door.
func Take(serial string) (Survey, error) {
	s := Survey{Serial: serial}

	if model, err := adb.Shell(serial, "getprop", "ro.product.model"); err == nil {
		s.Model = strings.TrimSpace(model)
	}
	if name, err := adb.Shell(serial, "getprop", "net.hostname"); err == nil {
		s.Hostname = strings.TrimSpace(name)
	}

	s.Addresses = addresses(serial)

	// Over Wi-Fi the serial is already the address pusher is using, and that is
	// the one worth knocking on: the robot may have several and only one of
	// them is on a network this laptop can see.
	if host, _, found := strings.Cut(serial, ":"); found && host != "" {
		s.Host = host
	} else if len(s.Addresses) > 0 {
		s.Host = s.Addresses[0]
	}

	// Reported only if it resolves. A name that does not is worse than no name:
	// somebody pastes it, it fails, and now they are debugging the wrong thing.
	if s.Hostname != "" {
		candidate := s.Hostname + ".local"
		if _, err := net.LookupHost(candidate); err == nil {
			s.Local = candidate
		}
	}

	s.Services = knock(s.Host)
	return s, nil
}

// knock tries every service at once and reports which answered.
func knock(host string) []Service {
	out := make([]Service, len(services))
	copy(out, services)

	if host == "" {
		return out
	}

	var wg sync.WaitGroup
	for i := range out {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			out[i].Reachable = reachable(fmt.Sprintf("%s:%d", host, out[i].Port))
		}(i)
	}
	wg.Wait()

	return out
}

var ipLine = regexp.MustCompile(`inet ([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)`)

// addresses is every IPv4 address the robot holds, as it reports them.
//
// Asked of the robot rather than worked out from the serial, because over USB
// the serial is not an address at all and the whole question is what network
// the robot is on.
func addresses(serial string) []string {
	out, err := adb.Shell(serial, "ip", "-4", "-o", "addr", "2>/dev/null", "||", "ifconfig", "2>/dev/null", "||", "true")
	if err != nil {
		return nil
	}

	seen := map[string]bool{}
	var found []string

	for _, match := range ipLine.FindAllStringSubmatch(out, -1) {
		ip := match[1]

		// The loopback and the hub's own Limelight link are addresses, and
		// neither is one this laptop can reach.
		if strings.HasPrefix(ip, "127.") || strings.HasPrefix(ip, "172.29.") {
			continue
		}
		if !seen[ip] {
			seen[ip] = true
			found = append(found, ip)
		}
	}

	sort.Strings(found)
	return found
}

// waitFor gives a service a moment to come up, for the cases where something
// was only just started.
func waitFor(host string, port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if reachable(fmt.Sprintf("%s:%d", host, port)) {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}
