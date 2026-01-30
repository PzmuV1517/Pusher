package wifi

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Manager handles Wi-Fi operations
type Manager struct {
	iface string
}

// NewManager creates a new Wi-Fi manager
func NewManager() *Manager {
	// Default to en0 for macOS
	return &Manager{
		iface: "en0",
	}
}

// GetCurrentSSID returns the currently connected Wi-Fi SSID
func (m *Manager) GetCurrentSSID() (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	cmd := exec.Command("networksetup", "-getairportnetwork", m.iface)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current Wi-Fi: %w", err)
	}

	// Output format: "Current Wi-Fi Network: NetworkName"
	line := strings.TrimSpace(string(output))
	if strings.Contains(line, "You are not associated with an AirPort network") {
		return "", nil
	}

	parts := strings.SplitN(line, ": ", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("unexpected output format: %s", line)
	}

	return strings.TrimSpace(parts[1]), nil
}

// Connect connects to a Wi-Fi network
func (m *Manager) Connect(ssid, password string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	cmd := exec.Command("networksetup", "-setairportnetwork", m.iface, ssid, password)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to connect to Wi-Fi: %w (output: %s)", err, string(output))
	}

	return nil
}

// ConnectWithRetry attempts to connect to Wi-Fi with retries
func (m *Manager) ConnectWithRetry(ssid, password string, maxRetries int) error {
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		err := m.Connect(ssid, password)
		if err == nil {
			// Wait a moment to ensure connection is established
			time.Sleep(2 * time.Second)

			// Verify connection
			currentSSID, checkErr := m.GetCurrentSSID()
			if checkErr == nil && currentSSID == ssid {
				return nil
			}

			if checkErr != nil {
				lastErr = checkErr
			} else {
				lastErr = fmt.Errorf("connected to wrong network: %s", currentSSID)
			}
		} else {
			lastErr = err
		}

		if i < maxRetries-1 {
			time.Sleep(2 * time.Second)
		}
	}

	return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// IsConnected checks if connected to a specific SSID
func (m *Manager) IsConnected(ssid string) (bool, error) {
	current, err := m.GetCurrentSSID()
	if err != nil {
		return false, err
	}
	return current == ssid, nil
}
