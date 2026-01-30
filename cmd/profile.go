package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage robot profiles",
	Long:  `Manage robot Wi-Fi profiles for connecting to different robots.`,
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles",
	RunE:  runProfileList,
}

var profileAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new profile",
	RunE:  runProfileAdd,
}

var profileEditCmd = &cobra.Command{
	Use:   "edit <name>",
	Short: "Edit an existing profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileEdit,
}

var profileUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set default profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileUse,
}

func init() {
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileAddCmd)
	profileCmd.AddCommand(profileEditCmd)
	profileCmd.AddCommand(profileUseCmd)
}

func runProfileList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Profiles) == 0 {
		fmt.Println("No profiles configured.")
		fmt.Println("\nRun 'pusher profile add' to create a profile.")
		return nil
	}

	fmt.Println("Robot Profiles:")
	fmt.Println()
	for name, profile := range cfg.Profiles {
		marker := " "
		if name == cfg.DefaultProfile {
			marker = "*"
		}
		fmt.Printf("  %s %s\n", marker, name)
		fmt.Printf("      SSID: %s\n", profile.SSID)
		fmt.Println()
	}

	if cfg.DefaultProfile != "" {
		fmt.Printf("Default profile: %s\n", cfg.DefaultProfile)
	}

	return nil
}

func runProfileAdd(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	// Get profile name
	fmt.Print("Profile name: ")
	name, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read profile name: %w", err)
	}
	name = strings.TrimSpace(name)

	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}

	// Get SSID
	fmt.Print("Robot Wi-Fi SSID: ")
	ssid, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read SSID: %w", err)
	}
	ssid = strings.TrimSpace(ssid)

	// Get password
	fmt.Print("Robot Wi-Fi Password: ")
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}
	password := string(passwordBytes)
	fmt.Println() // New line after password input

	// Save profile
	if err := config.AddProfile(name, ssid, password); err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}

	fmt.Printf("\n[OK] Profile '%s' saved\n", name)
	return nil
}

func runProfileEdit(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Check if profile exists
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if _, ok := cfg.Profiles[name]; !ok {
		return fmt.Errorf("profile '%s' not found", name)
	}

	reader := bufio.NewReader(os.Stdin)

	// Get new SSID
	fmt.Print("New Robot Wi-Fi SSID (press Enter to keep current): ")
	ssid, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read SSID: %w", err)
	}
	ssid = strings.TrimSpace(ssid)

	// Get new password
	fmt.Print("New Robot Wi-Fi Password (press Enter to keep current): ")
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}
	password := string(passwordBytes)
	fmt.Println() // New line after password input

	// Update profile
	profile := cfg.Profiles[name]
	if ssid != "" {
		profile.SSID = ssid
	}
	if password != "" {
		profile.Password = password
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("\n[OK] Profile '%s' updated\n", name)
	return nil
}

func runProfileUse(cmd *cobra.Command, args []string) error {
	name := args[0]

	if err := config.SetDefaultProfile(name); err != nil {
		return err
	}

	fmt.Printf("[OK] Default profile set to '%s'\n", name)
	return nil
}
