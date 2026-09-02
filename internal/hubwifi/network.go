package hubwifi

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/andreibanu/pusher/internal/adb"
)

// writeConf puts the network on the hub.
//
// p2p_disabled is the line that makes this work at all. The adapter's driver
// presents a second P2P radio of its own, and Android's wpa_supplicant is built
// with P2P support: initialising it against that second radio fails, and the
// failure surfaces as "Failed to add interface wlan1" long after nl80211 has
// come up perfectly, which points at everything except the cause.
func (r runner) writeConf(opt Options) error {
	conf := fmt.Sprintf(`ctrl_interface=%s
update_config=1
p2p_disabled=1
network={
    ssid="%s"
    psk="%s"
    key_mgmt=WPA-PSK
}
`, CtrlDir, escape(opt.SSID), escape(opt.Password))

	return r.push(conf, ConfPath)
}

// escape keeps a quote or a backslash in a passphrase from ending the string it
// is in.
func escape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
}

// push writes text to a file on the hub.
func (r runner) push(content, path string) error {
	local, err := os.CreateTemp("", "pusher-hub-*")
	if err != nil {
		return err
	}
	defer os.Remove(local.Name())

	if _, err := local.WriteString(content); err != nil {
		local.Close()
		return err
	}
	local.Close()

	if err := adb.Push(r.serial, local.Name(), path); err != nil {
		return fmt.Errorf("cannot write %s on the robot: %w", path, err)
	}
	return nil
}

var inetRe = regexp.MustCompile(`inet ([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)`)

// address gets the hub an address and reports it.
//
// Android's DHCP clients obtain a lease and then do nothing with it. dhcptool
// performs the handshake and prints the result, expecting netd to apply it, and
// netd manages wlan0 and knows nothing about this interface. busybox udhcpc
// hands its result to a script named with -s, and the path it defaults to does
// not exist here, so it negotiates a perfectly good lease and drops it.
//
// So the lease is applied by a script of ours.
func (r runner) address() (Result, error) {
	r.say("[*] Asking for an address")

	if err := r.push(leaseScript, LeasePath); err != nil {
		return Result{}, err
	}
	r.quiet("chmod", "755", LeasePath)

	r.quiet("busybox", "udhcpc", "-i", Iface, "-s", LeasePath, "-t", "8", "-T", "3", "-n", "-q")

	for i := 0; i < 10; i++ {
		if found := r.current(); found.Address != "" {
			r.say("[OK] %s has %s", Iface, found.Address)
			return found, nil
		}
		time.Sleep(time.Second)
	}

	return Result{}, fmt.Errorf("associated with the network but it gave out no address.\n" +
		"    Either its DHCP server did not answer, or busybox is missing from this hub")
}

// current is the address the interface holds now, if any.
func (r runner) current() Result {
	out, err := r.run("ip", "-4", "addr", "show", Iface)
	if err != nil {
		return Result{}
	}

	m := inetRe.FindStringSubmatch(out)
	if m == nil {
		return Result{}
	}

	address := m[1]

	gateway := ""
	if route, err := r.run("ip", "route", "show", "dev", Iface); err == nil {
		if g := regexp.MustCompile(`default via ([0-9.]+)`).FindStringSubmatch(route); g != nil {
			gateway = g[1]
		}
	}
	if gateway == "" {
		// The router is almost always the first address on the network, and a
		// guess that is checked by the route working is better than no route.
		if i := strings.LastIndex(address, "."); i > 0 {
			gateway = address[:i] + ".1"
		}
	}

	return Result{Address: address, Gateway: gateway}
}

// route makes the hub reachable from the network it just joined.
//
// Without this the hub answers ping and refuses every TCP connection by
// silence: the request arrives, the service accepts it, and the reply cannot be
// routed. Android routes by policy, one table per network it manages, and an
// interface it does not manage appears in no rule. A reply from an ordinary
// process carries fwmark 0, matches "lookup local_network", finds nothing, and
// falls through to the unreachable catch-all.
//
// The routes therefore go into local_network, which is table 97, because that
// is where the rules already look. Adding a rule instead is the obvious move
// and the wrong one: a bare `ip rule add` is given a priority below the
// unreachable catch-all and is never consulted.
func (r runner) route(found Result) error {
	r.say("[*] Making it reachable from that network")

	network := subnetOf(found.Address)
	if network == "" {
		return fmt.Errorf("cannot work out the network %s is on", found.Address)
	}

	r.quiet("ip", "route", "add", network, "dev", Iface, "table", fmt.Sprint(LocalNetworkTable))
	if found.Gateway != "" {
		r.quiet("ip", "route", "add", "default", "via", found.Gateway, "dev", Iface,
			"table", fmt.Sprint(LocalNetworkTable))
	}

	return nil
}

// subnetOf assumes a /24, which every network a robot is taken to is.
func subnetOf(address string) string {
	i := strings.LastIndex(address, ".")
	if i <= 0 {
		return ""
	}
	return address[:i] + ".0/24"
}

// leaseScript is what udhcpc calls with the lease it negotiated.
const leaseScript = `#!/system/bin/sh
# Written by pusher. Applies what udhcpc negotiated, which no DHCP client on
# this hub will do by itself.
case "$1" in
  bound|renew)
    /system/bin/ip addr flush dev "$interface" 2>/dev/null
    PREFIX=24
    case "$subnet" in
      255.255.255.0)   PREFIX=24 ;;
      255.255.254.0)   PREFIX=23 ;;
      255.255.252.0)   PREFIX=22 ;;
      255.255.0.0)     PREFIX=16 ;;
      255.255.255.128) PREFIX=25 ;;
      255.255.255.192) PREFIX=26 ;;
    esac
    /system/bin/ip addr add "$ip/$PREFIX" dev "$interface"
    /system/bin/ip link set "$interface" up
    [ -n "$router" ] && /system/bin/ip route add default via "$router" dev "$interface" 2>/dev/null
    ;;
esac
exit 0
`
