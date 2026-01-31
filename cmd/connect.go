package cmd

import (
	"fmt"

	"github.com/andreibanu/pusher/internal/wifi"
	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Check robot Wi-Fi connection",
	Long:  `Checks whether you are connected to the robot's Wi-Fi (192.168.43.x) without changing networks.`,
	RunE:  runConnect,
}

func runConnect(cmd *cobra.Command, args []string) error {
	wifiMgr := wifi.NewManager()
	ip, err := wifiMgr.GetIPv4()
	if err != nil {
		return fmt.Errorf("failed to read Wi-Fi IP address: %w", err)
	}
	if ip == "" {
		fmt.Println("[!] No IPv4 address on Wi-Fi interface (en0).")
		fmt.Println("    Please connect to the robot's Wi-Fi (192.168.43.x) in macOS Wi-Fi settings.")
		return fmt.Errorf("robot Wi-Fi not connected")
	}

	onRobotNet, err := wifiMgr.IsOnRobotNetwork()
	if err != nil {
		return fmt.Errorf("failed to verify robot network: %w", err)
	}
	if !onRobotNet {
		fmt.Printf("[!] Wi-Fi IPv4 on en0 is %s, not 192.168.43.x.\n", ip)
		fmt.Println("    Please connect to the robot's Wi-Fi in macOS Wi-Fi settings.")
		return fmt.Errorf("robot Wi-Fi not connected")
	}

	fmt.Println("[OK] Robot Wi-Fi detected (192.168.43.x)")
	fmt.Println("[*] Run 'pusher' to build and deploy.")

	return nil
}
