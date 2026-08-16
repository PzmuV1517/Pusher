//go:build !darwin && !linux && !windows

package wifi

const scanSupported = false

// ScanNote explains what pusher does about networks it cannot see yet.
const ScanNote = "Looking for networks is only supported on macOS, Linux and Windows."

func (m *Manager) visible(ssid string) (bool, error) {
	return false, ErrScanUnsupported
}
