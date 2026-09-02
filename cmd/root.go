package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/feature"
	"github.com/andreibanu/pusher/internal/selfupdate"
	"github.com/andreibanu/pusher/internal/telemetry"
	"github.com/andreibanu/pusher/internal/updates"
	"github.com/spf13/cobra"
)

var (
	versionFlag bool
	appVersion  string

	// ignoreWarnings carries on past a check that would otherwise stop the
	// command. Persistent, so `pusher` and `pusher slim` both take it.
	ignoreWarnings bool
)

var rootCmd = &cobra.Command{
	Use:          "pusher",
	Short:        "FTC Robot deployment tool",
	Long:         `Pusher automates connecting to FTC robots and deploying Android Studio projects.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if versionFlag {
			fmt.Printf("Pusher version %s\n", appVersion)
			return nil
		}
		return pushCmd.RunE(cmd, args)
	},
}

// Execute runs the CLI.
func Execute(version string) {
	appVersion = version
	selfupdate.SetCurrent(version)

	if err := config.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize config: %v\n", err)
		os.Exit(1)
	}

	// A robot that was found somewhere other than its own access point is still
	// there next time, and every command needs to know before it asks adb
	// anything. Cheap: one string out of the config, no sockets.
	if config.GetRelay() {
		if addr := config.GetRobotAddress(); addr != "" {
			adb.UseAddress(addr)
		}
	}

	visualiseCmd.Hidden = !feature.Revealed()

	// Both started here so their requests overlap the command rather than
	// following it, and finished after it so a short command still gets one.
	// Neither can fail the run.
	counted := telemetry.Start(version)
	newer := updates.Watch()

	err := rootCmd.Execute()

	counted.Finish(pingWait)
	newer.Finish(pingWait)

	if err != nil {
		os.Exit(1)
	}
}

// pingWait is how long the device count may hold up an exit. Most commands
// outlast the request several times over, so this is usually not a wait at all.
const pingWait = 1500 * time.Millisecond

func init() {

	rootCmd.Flags().BoolVarP(&versionFlag, "version", "v", false, "Show version information")
	rootCmd.PersistentFlags().BoolVar(&ignoreWarnings, "ignore-warnings", false,
		"Carry on past a check that would otherwise stop the command")

	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(disconnectCmd)
	rootCmd.AddCommand(exitCmd)
	rootCmd.AddCommand(prepareCmd)
	rootCmd.AddCommand(settingsCmd)
	rootCmd.AddCommand(slimCmd)
	rootCmd.AddCommand(hwconfigCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(dashCmd)
	rootCmd.AddCommand(powerCmd)
	rootCmd.AddCommand(profileCmd)
	rootCmd.AddCommand(ipCmd)
	rootCmd.AddCommand(relayCmd)
	rootCmd.AddCommand(devCmd)
	rootCmd.AddCommand(visualiseCmd)
	rootCmd.AddCommand(helpCmd)
}
