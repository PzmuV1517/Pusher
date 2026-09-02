package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/robot"
	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Join the robot's Wi-Fi and connect ADB",
	Long:  `Joins the robot's Wi-Fi network and establishes an ADB connection, without building or deploying.`,
	RunE:  runConnect,
}

func runConnect(cmd *cobra.Command, args []string) error {
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
