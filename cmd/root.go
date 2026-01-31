package cmd

import (
	"fmt"
	"os"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/spf13/cobra"
)

var (
	versionFlag bool
	appVersion  string
)

var rootCmd = &cobra.Command{
	Use:   "pusher",
	Short: "FTC Robot deployment tool",
	Long:  `Pusher automates connecting to FTC robots and deploying Android Studio projects.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if versionFlag {
			fmt.Printf("Pusher version %s\n", appVersion)
			return nil
		}
		return pushCmd.RunE(cmd, args)
	},
}

// Execute runs the root command
func Execute(version string) {
	appVersion = version

	// Initialize config
	if err := config.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize config: %v\n", err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Add flags
	rootCmd.Flags().BoolVarP(&versionFlag, "version", "v", false, "Show version information")

	// Add subcommands
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(disconnectCmd)
	rootCmd.AddCommand(exitCmd)
	rootCmd.AddCommand(profileCmd)
	rootCmd.AddCommand(helpCmd)
}
