package hubwifi

import "fmt"

// The boot script does not run once and stop. It watches.
//
// A one-shot handles the reboot and nothing else, and the adapter being pulled
// out and put back is the ordinary case: somebody moves the robot, knocks the
// dongle, swaps a port for a camera. With a one-shot that means the hub is off
// the network until a laptop is fetched and a command is run, which is exactly
// the errand this feature exists to remove.
//
// So it checks every half minute, and does the least that is missing: nothing
// when the address is there, the driver when the interface has gone, the
// association when the driver is loaded but nothing is joined. That covers the
// adapter coming back, the access point rebooting, and the lease expiring, all
// without anybody being told.

const superviseSeconds = 30

// bootScript is the whole sequence again, as the hub itself runs it.
//
// Written out rather than calling back to the laptop, because at boot there is
// no laptop. Every step is the one that works rather than the obvious one, for
// the reasons given where each is done from the laptop side.
func (r runner) bootScript(opt Options) string {
	return fmt.Sprintf(`#!/system/bin/sh
# Written by pusher. Keeps this hub on %q over its USB Wi-Fi adapter.
#
# Runs at boot and then watches: if the adapter is unplugged and put back, or
# the access point restarts, this puts it back on without anybody asking.
#
# Remove %s to stop it, or run `+"`pusher relay forget`"+` from a laptop.

IFACE=%s
CTRL=%s
CONF=%s
LEASE=%s
MODULE=%s
TABLE=%d

have_iface() { [ -e "/sys/class/net/$IFACE" ]; }
address()    { ip -4 addr show "$IFACE" 2>/dev/null | grep -o 'inet [0-9.]*' | head -1 | cut -d' ' -f2; }

join() {
  have_iface || insmod "$MODULE" 2>/dev/null
  sleep 3
  have_iface || return 1

  ip link set "$IFACE" up

  # One supplicant. More than one shares the radio and none of them associates.
  for pid in $(ps | grep wpa_supplican[t] | awk '{print $2}'); do kill -9 "$pid"; done
  sleep 1
  rm -f "$CTRL/$IFACE"

  # The interface does not come back from an association in a state a fresh
  # supplicant can scan from.
  ip link set "$IFACE" down
  sleep 1
  ip link set "$IFACE" up

  wpa_supplicant -B -i "$IFACE" -Dnl80211 -c "$CONF"
  sleep 2

  # A loaded config is not an enabled, selected network.
  wpa_cli -p "$CTRL" -i "$IFACE" enable_network 0
  wpa_cli -p "$CTRL" -i "$IFACE" select_network 0

  i=0
  while [ $i -lt 30 ]; do
    case "$(wpa_cli -p "$CTRL" -i "$IFACE" status 2>/dev/null | grep wpa_state=)" in
      *COMPLETED*) break ;;
    esac
    sleep 2
    i=$((i + 1))
  done

  # Android's DHCP clients get a lease and do nothing with it, so the lease
  # script applies it.
  busybox udhcpc -i "$IFACE" -s "$LEASE" -t 8 -T 3 -n -q

  IP=$(address)
  [ -n "$IP" ] || return 1

  # Reachable from the network, not merely on it: Android routes by policy and
  # has no table for this interface, so the routes go where its rules look.
  NET=$(echo "$IP" | cut -d. -f1-3).0/24
  GW=$(ip route show dev "$IFACE" 2>/dev/null | grep -o 'default via [0-9.]*' | head -1 | cut -d' ' -f3)
  [ -n "$GW" ] || GW=$(echo "$IP" | cut -d. -f1-3).1

  ip route add "$NET" dev "$IFACE" table $TABLE 2>/dev/null
  ip route add default via "$GW" dev "$IFACE" table $TABLE 2>/dev/null
  return 0
}

# The adapter takes a moment to enumerate after a cold boot.
sleep 20

while true; do
  if [ -z "$(address)" ]; then
    join
  fi
  sleep %d
done
`,
		opt.SSID, InitScript,
		Iface, CtrlDir, ConfPath, LeasePath, ModulePath, LocalNetworkTable,
		superviseSeconds)
}
