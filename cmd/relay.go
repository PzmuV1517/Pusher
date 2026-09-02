package cmd

import (
	"fmt"
	"os"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/hubwifi"
	"github.com/andreibanu/pusher/internal/robot"
	"github.com/spf13/cobra"
)

var relayCmd = &cobra.Command{
	Use:   "relay",
	Short: "Put the robot on your own Wi-Fi, over a USB adapter",
	Long: `Loads a Wi-Fi driver onto the Control Hub, joins the network you name, and
makes the robot reachable there, so deploys stop needing you to switch networks.

The hub's own radio is its access point and cannot join anything: the SDK has no
client mode for it. This needs a USB Wi-Fi adapter the hub's kernel can drive.
` + "`pusher ip --adapters`" + ` says whether yours is one.

Inspired by Dhruv, FTC 32001L, whose ADB relay bridged adb over a Linux box on
the robot's network. This gets at the same thing from the other end.`,
}

var relaySetupCmd = &cobra.Command{
	Use:   "setup [network] [password]",
	Short: "Join the robot to a network and keep it there",
	RunE:  runRelaySetup,
}

var relayForgetCmd = &cobra.Command{
	Use:   "forget",
	Short: "Undo it, and leave the robot as it was",
	RunE:  runRelayForget,
}

func init() {
	relayCmd.AddCommand(relaySetupCmd)
	relayCmd.AddCommand(relayForgetCmd)
}

func runRelaySetup(cmd *cobra.Command, args []string) error {
	serial, err := requireRobot()
	if err != nil {
		return err
	}

	ssid, password := config.GetHubNetwork()
	if len(args) > 0 {
		ssid = args[0]
	}
	if len(args) > 1 {
		password = args[1]
	}

	if ssid == "" {
		return fmt.Errorf("name the network for the robot to join:\n" +
			"    pusher relay setup \"Your WiFi\" \"your password\"")
	}

	// Remembered, because the boot hook and every later run need it and nobody
	// wants to type a passphrase twice.
	_ = config.RememberHubNetwork(ssid, password)

	found, err := hubwifi.Setup(os.Stdout, serial, hubwifi.Options{
		SSID:     ssid,
		Password: password,
		Persist:  true,
	})
	if err != nil {
		return err
	}

	robot.Remember(robot.Found{Addr: found.Address + ":5555", Model: "Control Hub"})
	_ = config.SetRelay(true)

	fmt.Printf("\n[OK] The robot is on %s at %s\n", ssid, found.Address)
	fmt.Println("     Deploys will go there now, without touching your Wi-Fi.")
	fmt.Println("     `pusher ip` shows what it is serving.")

	return nil
}

func runRelayForget(cmd *cobra.Command, args []string) error {
	serial, err := requireRobot()
	if err != nil {
		return err
	}

	if err := hubwifi.Undo(serial); err != nil {
		return err
	}

	_ = config.SetRelay(false)
	_ = config.ForgetSpots()

	fmt.Println("[OK] Undone. The robot goes back to its own access point on the next reboot.")
	return nil
}
