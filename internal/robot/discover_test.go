package robot

import (
	"fmt"
	"net"
	"strings"
	"testing"
)

// A sweep walks a subnet, and getting the arithmetic wrong is either a scan
// that misses the robot or one that opens ten thousand sockets on somebody's
// corporate network.
func TestASubnetIsWalkedWithoutTheEdges(t *testing.T) {
	_, network, err := net.ParseCIDR("10.0.0.0/24")
	if err != nil {
		t.Fatal(err)
	}
	network.IP = net.ParseIP("10.0.0.7")

	hosts := subnet(network)

	// 254 usable, less this laptop's own address.
	if len(hosts) != 253 {
		t.Fatalf("walked %d hosts of a /24, want 253", len(hosts))
	}

	seen := map[string]bool{}
	for _, host := range hosts {
		seen[host] = true
	}

	for _, absent := range []string{"10.0.0.0", "10.0.0.255", "10.0.0.7"} {
		if seen[absent] {
			t.Errorf("%s should not be in the sweep", absent)
		}
	}
	for _, present := range []string{"10.0.0.1", "10.0.0.42", "10.0.0.254"} {
		if !seen[present] {
			t.Errorf("%s is missing from the sweep", present)
		}
	}
}

// A wide network is one nobody wants pusher opening sockets across, and a robot
// on one has to be found by remembering where it was rather than by hunting.
func TestAWideNetworkIsNotSwept(t *testing.T) {
	for _, cidr := range []string{"10.0.0.0/8", "172.16.0.0/12", "10.1.0.0/16"} {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			t.Fatal(err)
		}
		network.IP = net.ParseIP("10.0.0.7")

		if hosts := subnet(network); len(hosts) != 0 {
			t.Errorf("%s would sweep %d addresses", cidr, len(hosts))
		}
	}
}

// A /25 and smaller are still worth walking, and the count has to follow the
// mask rather than assume the common case.
func TestNarrowerNetworksFollowTheirMask(t *testing.T) {
	for _, tc := range []struct {
		cidr string
		want int
	}{
		{"192.168.1.0/25", 125},
		{"192.168.1.0/26", 61},
		{"192.168.1.0/24", 253},
	} {
		_, network, err := net.ParseCIDR(tc.cidr)
		if err != nil {
			t.Fatal(err)
		}
		network.IP = net.ParseIP("192.168.1.5")

		if got := len(subnet(network)); got != tc.want {
			t.Errorf("%s walked %d hosts, want %d", tc.cidr, got, tc.want)
		}
	}
}

// An address with no port is one adb cannot use.
func TestAnAddressAlwaysCarriesItsPort(t *testing.T) {
	if got := withPort("10.0.0.42"); got != fmt.Sprintf("10.0.0.42:%d", AdbPort) {
		t.Errorf("withPort = %q", got)
	}
	if got := withPort("10.0.0.42:5555"); got != "10.0.0.42:5555" {
		t.Errorf("a port already there was changed: %q", got)
	}
}

// Something answering on the adb port is not a robot. It could be a phone, a
// tablet, or a teammate's laptop with adb left running, and pusher installing
// an FTC app onto one of those is a worse outcome than not finding the robot.
func TestOnlyListeningIsNotEnoughToBeARobot(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	addr := listener.Addr().String()

	if !reachable(addr) {
		t.Fatal("the fixture is not listening, so this proves nothing")
	}

	// check has to go further than reachable, and on something that is not a
	// robot it has to come back empty.
	if _, ok := check(addr); ok {
		t.Error("called a plain TCP listener a robot")
	}
}

func TestNothingAnsweringIsNotReachable(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()

	if reachable(addr) {
		t.Errorf("%s answered after it was closed", addr)
	}
}

// A Control Hub has wlan0 and p2p0, and they are one radio: p2p0 is Wi-Fi
// Direct on the same chip. Counting wireless interfaces rather than radios
// found a second one on every hub and reported an adapter that was not there,
// which is worse than missing one that is: it tells somebody the hard part is
// done when nothing is plugged in.
func TestWifiDirectIsNotASecondAdapter(t *testing.T) {
	hub := []Iface{
		{Name: "p2p0", Wireless: true, Phy: "phy0"},
		{Name: "sit0"},
		{Name: "wlan0", Wireless: true, Up: true, Address: "192.168.43.1", Phy: "phy0"},
	}

	if got := extraRadio(hub); got != "" {
		t.Errorf("called %q an adapter on a hub with nothing plugged in", got)
	}
}

func TestASecondRadioIsFound(t *testing.T) {
	withAdapter := []Iface{
		{Name: "p2p0", Wireless: true, Phy: "phy0"},
		{Name: "wlan0", Wireless: true, Up: true, Address: "192.168.43.1", Phy: "phy0"},
		{Name: "wlan1", Wireless: true, Phy: "phy1"},
	}

	if got := extraRadio(withAdapter); got != "wlan1" {
		t.Errorf("extraRadio = %q, want wlan1", got)
	}
}

// A kernel too old to report a phy still has to be usable, and there the name
// is all there is: anything past wlan0 was plugged in.
func TestWithoutPhysTheNameDecides(t *testing.T) {
	old := []Iface{
		{Name: "p2p0", Wireless: true},
		{Name: "wlan0", Wireless: true, Up: true, Address: "192.168.43.1"},
	}
	if got := extraRadio(old); got != "" {
		t.Errorf("called %q an adapter", got)
	}

	old = append(old, Iface{Name: "wlan1", Wireless: true})
	if got := extraRadio(old); got != "wlan1" {
		t.Errorf("extraRadio = %q, want wlan1", got)
	}
}

// The table is the difference between "that adapter does not work" and "buy
// this one", so a chipset missing from it is a dead end reported as a mystery.
// rtl8821cu was missing, on a hub whose kernel has it.
func TestTheDriverTableKnowsWhatKernelsActuallyShip(t *testing.T) {
	// Names exactly as they appear under /sys/bus/usb/drivers on a Control Hub.
	for _, driver := range []string{"rtl8821cu"} {
		if !named(usbWifiDrivers, driver) {
			t.Errorf("no name for the Wi-Fi driver %q", driver)
		}
	}
	for _, driver := range []string{"asix", "ax88179_178a", "smsc95xx", "cdc_ether", "rtl8150"} {
		if !named(usbEthernetDrivers, driver) {
			t.Errorf("no name for the Ethernet driver %q", driver)
		}
	}
}

func named(table []struct {
	Match []string
	Name  string
}, driver string) bool {
	for _, entry := range table {
		for _, want := range entry.Match {
			if strings.Contains(strings.ToUpper(driver), want) {
				return true
			}
		}
	}
	return false
}
