package cmd

import (
	"fmt"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/wifi"
	"github.com/spf13/cobra"
)

var exitCmd = &cobra.Command{
	Use:   "exit",
	Short: "Disconnect and restore Wi-Fi",
	Long:  `Disconnects ADB and restores the previous Wi-Fi connection.`,
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

	// Restore Wi-Fi
	lastWiFi, err := config.GetLastWiFi()
	if err != nil || lastWiFi == "" {
		fmt.Println("\nNo previous Wi-Fi to restore.")
		return nil
	}

	fmt.Printf("\n[>] Restoring Wi-Fi connection to: %s\n", lastWiFi)

	wifiMgr := wifi.NewManager()

	// Try to connect back to the last Wi-Fi
	// Note: We don't have the password, so this will only work if it's a known network
	currentSSID, err := wifiMgr.GetCurrentSSID()
	if err == nil && currentSSID == lastWiFi {
		fmt.Println("[OK] Already connected to previous Wi-Fi")
		return nil
	}

	// Attempt to connect without password (for known networks)
	if err := wifiMgr.Connect(lastWiFi, ""); err != nil {
		fmt.Printf("[!] Could not automatically restore Wi-Fi: %v\n", err)
		fmt.Printf("Please manually reconnect to: %s\n", lastWiFi)
		return nil
	}

	fmt.Println("[OK] Wi-Fi restored")
	return nil
}
