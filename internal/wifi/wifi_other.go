//go:build !darwin

package wifi

import "fmt"

// Manager is a stub Wi-Fi manager for non-macOS platforms.
// It exists so the project can build on CI (Linux/Windows), but
// all Wi-Fi-dependent commands are effectively disabled.
type Manager struct {
	iface string
}

// NewManager creates a new Wi-Fi manager.
func NewManager() *Manager {
	return &Manager{iface: ""}
}

// GetIPv4 always returns an empty IP on non-macOS platforms.
func (m *Manager) GetIPv4() (string, error) {
	return "", nil
}

// IsOnRobotNetwork always reports false on non-macOS platforms.
func (m *Manager) IsOnRobotNetwork() (bool, error) {
	return false, nil
}

// PowerCycle returns an unsupported error on non-macOS platforms.
func (m *Manager) PowerCycle() error {
	return fmt.Errorf("Wi-Fi power-cycling is only supported on macOS")
}
