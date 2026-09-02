package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/robot"
	"github.com/spf13/cobra"
)

var connectFind bool

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Join the robot's Wi-Fi and connect ADB",
	Long: `Joins the robot's Wi-Fi network and establishes an ADB connection, without
building or deploying.

With ADB relay on, this looks for the robot on the network you are already on
first, and only takes over your Wi-Fi if it is not there. --find sweeps the
whole network for it, which is what to run the first time or after it moves.`,
	RunE: runConnect,
}

func init() {
	connectCmd.Flags().BoolVar(&connectFind, "find", false,
		"Sweep this network for the robot rather than only looking where it was")
}

func runConnect(cmd *cobra.Command, args []string) error {
	if connectFind {
		found, err := robot.Locate(os.Stdout, true)
		if err != nil {
			return err
		}

		robot.Remember(found)
		fmt.Printf("[OK] %s at %s\n", orRobot(found.Model), found.Addr)
		fmt.Println("[*] Remembered, so the next run finds it straight away.")

		if !config.GetRelay() {
			fmt.Println("[!] ADB relay is off, so deploys will still join the robot's own Wi-Fi.")
			fmt.Println("    Turn it on in `pusher settings` -> ADB relay.")
		}
		return nil
	}

	return connectAndSay()
}

func orRobot(model string) string {
	if model == "" {
		return "Robot"
	}
	return model
}

func connectAndSay() error {
	if err := connectRobot(); err != nil {
		return err
	}

	fmt.Println("[*] Run 'pusher' to build and deploy, or 'pusher exit' when you're done.")
	return nil
}

// connectRobot runs the connect flow, and sets up a profile first if there is
// no network on file and somebody here to be asked for one.
func connectRobot() error {
	err := robot.Connect(os.Stdout)
	if !errors.Is(err, robot.ErrNoProfile) {
		return err
	}

	// Nothing on file at all is a first run, and worth asking about. A config
	// that has profiles but no default one is a mistake in the config, which is
	// answered in settings rather than by asking the same questions again.
	if has, _ := config.HasProfiles(); has {
		return fmt.Errorf("%w\n\nRun 'pusher settings' to pick one", err)
	}

	if err := firstRunSetup(); err != nil {
		return err
	}

	return robot.Connect(os.Stdout)
}
