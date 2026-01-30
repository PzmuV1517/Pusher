package cmd

import (
	"fmt"
	"os"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "pusher",
	Short: "FTC Robot deployment tool",
	Long:  `Pusher automates connecting to FTC robots and deploying Android Studio projects.`,
	RunE:  pushCmd.RunE, // Default behavior is to push
}

// Execute runs the root command
func Execute() {
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
	// Add subcommands
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(disconnectCmd)
	rootCmd.AddCommand(exitCmd)
	rootCmd.AddCommand(profileCmd)
	rootCmd.AddCommand(helpCmd)
}
