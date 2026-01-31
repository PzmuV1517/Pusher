package adb

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	RobotIP   = "192.168.43.1"
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
	fmt.Printf("[*] Attempting ADB connection to %s...\n", addr)
	
	maxRetries := 5
	var lastErr error
	
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			fmt.Printf("[*] ADB retry %d/%d...\n", i+1, maxRetries)
			time.Sleep(3 * time.Second)
		}
		
		cmd := exec.Command("adb", "connect", addr)
		output, err := cmd.CombinedOutput()
		outputStr := strings.TrimSpace(string(output))
		
		fmt.Printf("[*] ADB response: %s\n", outputStr)
		
		if err != nil {
			lastErr = fmt.Errorf("adb command failed: %w", err)
			continue
		}

		lowerOutput := strings.ToLower(outputStr)
		if strings.Contains(lowerOutput, "connected") || strings.Contains(lowerOutput, "already connected") {
			fmt.Println("[OK] ADB connection established")
			return nil
		}
		
		lastErr = fmt.Errorf("unexpected response: %s", outputStr)
	}

	return fmt.Errorf("ADB connection failed after %d attempts: %w\n\n[!] Troubleshooting:\n  1. Ensure you're connected to the robot's Wi-Fi\n  2. Enable ADB debugging on Robot Controller\n  3. Try 'adb connect %s' manually\n  4. Check robot app is running", maxRetries, lastErr, addr)
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
