package cmd

import (
	"errors"
	"fmt"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/wifi"
	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Join the robot's Wi-Fi and connect ADB",
	Long:  `Joins the robot's Wi-Fi network and establishes an ADB connection, without building or deploying.`,
	RunE:  runConnect,
}

func runConnect(cmd *cobra.Command, args []string) error {
	if !adb.IsInstalled() {
		return fmt.Errorf("adb not found - please install Android SDK Platform-Tools")
	}

	if device, ok := adb.FindUSBDevice(); ok {
		fmt.Printf("[OK] Hub already attached over USB: %s\n", device.Label())
		fmt.Println("[*] Run 'pusher' to build and deploy.")
		return nil
	}

	wifiMgr := wifi.NewManager()

	onRobot, err := wifiMgr.IsOnRobotNetwork()
	if err != nil {
		return fmt.Errorf("failed to check the current network: %w", err)
	}

	if onRobot {
		fmt.Println("[OK] Already on the robot network")
	} else {
		if err := ensureProfile(); err != nil {
			return err
		}

		profile, err := config.GetDefaultProfile()
		if err != nil {
			return fmt.Errorf("no robot profile configured: %w\n\nRun 'pusher settings' to add one", err)
		}

		// Started before the questions below, so the scan overlaps them rather
		// than following them.
		watcher := wifiMgr.Watch(profile.SSID)
		defer watcher.Stop()

		ssid, ssidErr := wifiMgr.CurrentSSID()
		switch {
		case ssidErr == nil && ssid != "":
			fmt.Printf("[OK] Currently on: %s\n", ssid)
		case errors.Is(ssidErr, wifi.ErrSSIDUnavailable):
			if inferred, err := wifiMgr.MostRecentNetwork(robotSSIDs()...); err == nil && inferred != "" {
				fmt.Printf("[*] The network name is hidden; assuming you are on %q\n", inferred)
			}
		}

		fmt.Printf("\n[>] Joining robot Wi-Fi: %s\n", profile.SSID)
		ip, err := joinRobot(wifiMgr, watcher, profile)
		if err != nil {
			return err
		}
		fmt.Printf("[OK] On the robot network (%s)\n", ip)
	}

	fmt.Println("\n[+] Connecting to robot via ADB...")
	if err := adb.Connect(); err != nil {
		return fmt.Errorf("failed to connect via ADB: %w", err)
	}

	fmt.Println("[OK] Connected via ADB")
	fmt.Println("[*] Run 'pusher' to build and deploy, or 'pusher exit' when you're done.")

	return nil
}
