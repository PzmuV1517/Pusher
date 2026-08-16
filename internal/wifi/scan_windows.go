//go:build windows

package wifi

const scanSupported = true

// ScanNote explains what pusher does about networks it cannot see yet.
const ScanNote = `pusher looks for the robot's network in the background while the project
builds, so the hub is already in range by the time it tries to join. It reads
the list Windows keeps of nearby networks, which needs no permission.

Turn it off with PUSHER_NO_SCAN=1 if scanning upsets your Wi-Fi.`

func (m *Manager) visible(ssid string) (bool, error) {
	out, err := netsh("wlan", "show", "networks")
	if err != nil {
		return false, err
	}
	return netshSawSSID(out, ssid), nil
}
