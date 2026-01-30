package adb

import (
	"fmt"
	"os/exec"
	"strings"
)

const (
	RobotIP = "192.168.43.1"
	RobotPort = "5555"
)

// IsInstalled checks if adb is installed and available
func IsInstalled() bool {
	_, err := exec.LookPath("adb")
	return err == nil
}

// Connect connects to the robot via adb
func Connect() error {
	if !IsInstalled() {
		return fmt.Errorf("adb not found - please install Android SDK Platform-Tools")
	}

	addr := fmt.Sprintf("%s:%s", RobotIP, RobotPort)
	cmd := exec.Command("adb", "connect", addr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb connect failed: %w (output: %s)", err, string(output))
	}

	outputStr := strings.ToLower(string(output))
	if !strings.Contains(outputStr, "connected") && !strings.Contains(outputStr, "already connected") {
		return fmt.Errorf("adb connect failed: %s", string(output))
	}

	return nil
}

// Disconnect disconnects from the robot
func Disconnect() error {
	if !IsInstalled() {
		return fmt.Errorf("adb not found")
	}

	cmd := exec.Command("adb", "disconnect")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb disconnect failed: %w (output: %s)", err, string(output))
	}

	return nil
}

// IsConnected checks if already connected to the robot
func IsConnected() bool {
	if !IsInstalled() {
		return false
	}

	cmd := exec.Command("adb", "devices")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	addr := fmt.Sprintf("%s:%s", RobotIP, RobotPort)
	return strings.Contains(string(output), addr)
}
