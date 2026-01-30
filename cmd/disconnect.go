package cmd

import (
	"fmt"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/spf13/cobra"
)

var disconnectCmd = &cobra.Command{
	Use:     "disconnect",
	Aliases: []string{"dc"},
	Short:   "Disconnect ADB connection",
	Long:    `Disconnects the ADB connection to the robot without changing Wi-Fi.`,
	RunE:    runDisconnect,
}

func runDisconnect(cmd *cobra.Command, args []string) error {
	fmt.Println("[+] Disconnecting ADB...")

	if !adb.IsInstalled() {
		return fmt.Errorf("adb not found")
	}

	if err := adb.Disconnect(); err != nil {
		return fmt.Errorf("failed to disconnect: %w", err)
	}

	fmt.Println("[OK] ADB disconnected")
	return nil
}
