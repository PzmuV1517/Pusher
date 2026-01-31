package cmd

import (
	"fmt"
	"time"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/wifi"
	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to robot Wi-Fi only",
	Long:  `Connects to the robot's Wi-Fi network using the default profile. Does not connect ADB or build.`,
	RunE:  runConnect,
}

func runConnect(cmd *cobra.Command, args []string) error {
	// Get default profile
	profile, err := config.GetDefaultProfile()
	if err != nil {
		return fmt.Errorf("failed to get default profile: %w\n\n[!] Run 'pusher profile add' to create a profile first", err)
	}

	// Initialize Wi-Fi manager
	wifiMgr := wifi.NewManager()

	// Save current Wi-Fi
	fmt.Println("[*] Detecting current Wi-Fi...")
	currentSSID, err := wifiMgr.GetCurrentSSID()
	if err != nil {
		return fmt.Errorf("failed to get current Wi-Fi: %w", err)
	}

	if currentSSID != "" {
		fmt.Printf("[~] Current Wi-Fi: %s\n", currentSSID)
		if err := config.SaveLastWiFi(currentSSID); err != nil {
			fmt.Printf("[!] Warning: failed to save Wi-Fi state: %v\n", err)
		}
	}

	// Same logic as main push flow:
	// 1) If already on 192.168.43.x, do nothing more.
	// 2) Otherwise connect, wait 10s, and re-check IP; fail if still not in subnet.

	ip, err := wifiMgr.GetIPv4()
	if err != nil {
		return fmt.Errorf("failed to read Wi-Fi IP address: %w", err)
	}
	if ip != "" {
		fmt.Printf("[*] Current Wi-Fi IPv4 on en0: %s\n", ip)
	}

	onRobotNet, err := wifiMgr.IsOnRobotNetwork()
	if err != nil {
		return fmt.Errorf("failed to verify robot network: %w", err)
	}

	if !onRobotNet {
		fmt.Printf("\n[>] Connecting to robot Wi-Fi: %s (profile: %s)\n", profile.SSID, profile.Name)
		fmt.Println("[!] Note: You may need to run with 'sudo pusher connect' for permissions")

		if err := wifiMgr.ConnectWithRetry(profile.SSID, profile.Password, 3); err != nil {
			return fmt.Errorf("failed to connect to robot Wi-Fi: %w\n\n[!] Tip: Try running 'sudo pusher connect'", err)
		}

		fmt.Println("[*] Waiting 10 seconds for network to stabilize...")
		time.Sleep(10 * time.Second)

		ip, err = wifiMgr.GetIPv4()
		if err != nil {
			return fmt.Errorf("failed to read Wi-Fi IP address after connect: %w", err)
		}
		fmt.Printf("[*] Wi-Fi IPv4 on en0 after connect: %s\n", ip)

		onRobotNet, err = wifiMgr.IsOnRobotNetwork()
		if err != nil {
			return fmt.Errorf("failed to verify robot network after connect: %w", err)
		}
		if !onRobotNet {
			return fmt.Errorf("Wi-Fi IP %s is not in robot subnet 192.168.43.x after connect - exiting", ip)
		}
	} else {
		fmt.Println("[>] Already on robot subnet (192.168.43.x), skipping Wi-Fi connect")
	}

	fmt.Println("[OK] Connected to robot Wi-Fi (192.168.43.x)")
	fmt.Printf("\n[*] You are now connected to: %s\n", profile.SSID)
	fmt.Println("[*] Use 'pusher' to continue with ADB and build")
	fmt.Println("[*] Use 'pusher exit' to disconnect and restore previous Wi-Fi")

	return nil
}
