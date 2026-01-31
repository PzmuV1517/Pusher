//go:build darwin

package wifi

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"
)

// Manager handles Wi-Fi related queries.
// On macOS we assume the Wi-Fi interface is en0 and do not try to
// change networks – we only inspect the current IP.
type Manager struct {
	iface string
}

// NewManager creates a new Wi-Fi manager.
func NewManager() *Manager {
	return &Manager{iface: "en0"}
}

// GetIPv4 returns the IPv4 address of the Wi-Fi interface (e.g., en0).
func (m *Manager) GetIPv4() (string, error) {
	iface, err := net.InterfaceByName(m.iface)
	if err != nil {
		return "", fmt.Errorf("failed to get interface %s: %w", m.iface, err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return "", fmt.Errorf("failed to get addresses for %s: %w", m.iface, err)
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok {
			ip4 := ipNet.IP.To4()
			if ip4 != nil {
				return ip4.String(), nil
			}
		}
	}

	return "", nil
}

// IsOnRobotNetwork reports whether the Wi-Fi IPv4 is in the FTC robot subnet.
// Current FTC default robot controller hotspot uses 192.168.43.x.
func (m *Manager) IsOnRobotNetwork() (bool, error) {
	ip, err := m.GetIPv4()
	if err != nil {
		return false, err
	}
	if ip == "" {
		return false, nil
	}
	return strings.HasPrefix(ip, "192.168.43."), nil
}

// PowerCycle turns Wi-Fi off and back on using networksetup.
// This forces a disconnect from the current network and lets
// macOS auto-join whichever network is preferred.
func (m *Manager) PowerCycle() error {
	// Turn Wi-Fi off
	offCmd := exec.Command("networksetup", "-setairportpower", m.iface, "off")
	if output, err := offCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to turn Wi-Fi off: %w (output: %s)", err, string(output))
	}

	// Small pause to let the interface settle
	time.Sleep(2 * time.Second)

	// Turn Wi-Fi back on
	onCmd := exec.Command("networksetup", "-setairportpower", m.iface, "on")
	if output, err := onCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to turn Wi-Fi on: %w (output: %s)", err, string(output))
	}

	return nil
}
