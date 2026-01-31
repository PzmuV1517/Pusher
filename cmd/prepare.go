package cmd

import (
	"fmt"
	"os"

	"github.com/andreibanu/pusher/internal/gradle"
	"github.com/spf13/cobra"
)

var prepareCmd = &cobra.Command{
	Use:   "prepare",
	Short: "Prepare Gradle for offline pusher builds",
	Long:  `Runs the Gradle wrapper online to download and cache dependencies so that 'pusher' can build offline later.`,
	RunE:  runPrepare,
}

func runPrepare(cmd *cobra.Command, args []string) error {
	fmt.Println("[*] Detecting Gradle wrapper...")
	wrapper, err := gradle.DetectWrapper()
	if err != nil {
		return fmt.Errorf("failed to detect Gradle wrapper: %w", err)
	}
	fmt.Printf("[OK] Found Gradle wrapper: %s\n", wrapper)

	fmt.Println("\n[#] Preparing Gradle cache (online build)...")
	fmt.Println("─────────────────────────────────────────")

	if err := gradle.BuildOnline(wrapper, os.Stdout); err != nil {
		return fmt.Errorf("prepare failed: %w", err)
	}

	fmt.Println("─────────────────────────────────────────")
	fmt.Println("\n[OK] Gradle dependencies prepared. You can now use 'pusher' on the robot Wi-Fi.")

	return nil
}
