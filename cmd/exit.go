package cmd

import (
	"fmt"
	"time"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/robot"
	"github.com/andreibanu/pusher/internal/wifi"
	"github.com/spf13/cobra"
)

var exitCmd = &cobra.Command{
	Use:   "exit",
	Short: "Disconnect from the robot and go back to your usual Wi-Fi",
	Long:  `Drops the ADB connection and rejoins the network you were on before deploying.`,
	RunE:  runExit,
}

func runExit(cmd *cobra.Command, args []string) error {
	fmt.Println("[+] Disconnecting ADB...")
	if adb.IsInstalled() {
		if err := adb.Disconnect(); err != nil {
			fmt.Printf("[!] Warning: failed to disconnect ADB: %v\n", err)
		} else {
			fmt.Println("[OK] ADB disconnected")
		}
	}

	wifiMgr := wifi.NewManager()

	onRobot, err := wifiMgr.IsOnRobotNetwork()
	if err != nil {
		return fmt.Errorf("failed to check the current network: %w", err)
	}
	if !onRobot {
		fmt.Println("[OK] Not on the robot network, leaving Wi-Fi alone")
		return nil
	}

	home := config.GetHomeSSID()
	if home == "" {

		if inferred, err := wifiMgr.MostRecentNetwork(robot.SSIDs()...); err == nil && inferred != "" {
			home = inferred
			fmt.Printf("\n[*] Assuming you came from %q\n", home)
		}
	}

	if home == "" {

		fmt.Println("\n[*] No home network known; cycling Wi-Fi so the system re-picks...")
		if err := wifiMgr.PowerCycle(); err != nil {
			fmt.Printf("[!] Warning: failed to power-cycle Wi-Fi: %v\n", err)
			fmt.Println("    You may need to switch networks manually.")
			return nil
		}
		fmt.Println("[OK] Wi-Fi cycled. Your system should auto-join its usual network.")
		fmt.Println("    Tip: set a home network in 'pusher settings' for a clean switch back.")
		return nil
	}

	fmt.Printf("\n[<] Returning to %s...\n", home)
	if err := wifiMgr.Join(home, ""); err != nil {
		fmt.Printf("[!] Could not rejoin %s: %v\n", home, err)
		fmt.Println("    You will need to switch back manually.")
		return nil
	}

	if _, err := wifiMgr.WaitForIP("", 30*time.Second); err != nil {
		fmt.Printf("[!] Rejoined %s but no IP address yet: %v\n", home, err)
		return nil
	}

	fmt.Printf("[OK] Back on %s\n", home)
	return nil
}
