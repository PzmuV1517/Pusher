package cmd

import (
	"fmt"
	"strconv"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/spf13/cobra"
)

var threadsCmd = &cobra.Command{
	Use:   "threads",
	Short: "Manage Gradle thread configuration",
	Long:  `View or configure the number of threads used for Gradle builds.`,
	RunE:  runThreads,
}

var threadsSetCmd = &cobra.Command{
	Use:   "set <count>",
	Short: "Set the number of Gradle threads",
	Long:  `Set the number of threads used for parallel Gradle builds.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runThreadsSet,
}

var threadsResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset threads to default (8)",
	Long:  `Reset the number of Gradle threads to the default value of 8.`,
	RunE:  runThreadsReset,
}

func init() {
	threadsCmd.AddCommand(threadsSetCmd)
	threadsCmd.AddCommand(threadsResetCmd)
}

func runThreads(cmd *cobra.Command, args []string) error {
	threads := config.GetThreads()
	fmt.Printf("Current thread count: %d\n", threads)
	fmt.Println("\nUsage:")
	fmt.Println("  pusher threads set <count>  - Set thread count")
	fmt.Println("  pusher threads reset        - Reset to default (8)")
	return nil
}

func runThreadsSet(cmd *cobra.Command, args []string) error {
	count, err := strconv.Atoi(args[0])
	if err != nil || count < 1 {
		return fmt.Errorf("invalid thread count: %s (must be a positive integer)", args[0])
	}

	if err := config.SetThreads(count); err != nil {
		return fmt.Errorf("failed to set threads: %w", err)
	}

	fmt.Printf("[OK] Thread count set to %d\n", count)
	return nil
}

func runThreadsReset(cmd *cobra.Command, args []string) error {
	if err := config.ResetThreads(); err != nil {
		return fmt.Errorf("failed to reset threads: %w", err)
	}

	fmt.Println("[OK] Thread count reset to default (8)")
	return nil
}
