package cmd

import (
	"fmt"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/wifi"
	"github.com/spf13/cobra"
)

var exitCmd = &cobra.Command{
	Use:   "exit",
	Short: "Disconnect from robot",
	Long:  `Disconnects ADB from the robot and power-cycles Wi-Fi so macOS can auto-join your usual network.`,
	RunE:  runExit,
}

func runExit(cmd *cobra.Command, args []string) error {
	// Disconnect ADB
	fmt.Println("[+] Disconnecting ADB...")
	if adb.IsInstalled() {
		if err := adb.Disconnect(); err != nil {
			fmt.Printf("[!] Warning: failed to disconnect ADB: %v\n", err)
		} else {
			fmt.Println("[OK] ADB disconnected")
		}
	}

	// Power-cycle Wi-Fi so we disconnect from the robot network.
	wifiMgr := wifi.NewManager()
	fmt.Println("\n[*] Power-cycling Wi-Fi (off then on)...")
	if err := wifiMgr.PowerCycle(); err != nil {
		fmt.Printf("[!] Warning: failed to power-cycle Wi-Fi: %v\n", err)
		fmt.Println("    You may need to disconnect from the robot Wi-Fi manually.")
		return nil
	}

	fmt.Println("[OK] Wi-Fi power-cycled. macOS should now auto-join your usual network.")
	return nil
}
