//go:build linux

package wifi

const scanSupported = true

// ScanNote explains what pusher does about networks it cannot see yet.
const ScanNote = `pusher looks for the robot's network in the background while the project
builds, so the hub is already in range by the time it tries to join. It asks
nmcli for a fresh scan, which needs no permission.

Turn it off with PUSHER_NO_SCAN=1 if scanning upsets your Wi-Fi.`

// --rescan yes forces a fresh sweep rather than reusing NetworkManager's list,
// which is what makes a hub that has just been switched on show up.
func (m *Manager) visible(ssid string) (bool, error) {
	out, err := nmcli("-t", "-f", "SSID", "device", "wifi", "list", "--rescan", "yes")
	if err != nil {
		return false, err
	}
	return nmcliSawSSID(out, ssid), nil
}
