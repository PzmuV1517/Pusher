package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/gradle"
	"github.com/andreibanu/pusher/internal/wifi"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Connect to robot, build and deploy",
	Long:  `Connects to the robot's Wi-Fi, establishes ADB connection, and builds/deploys the app.`,
	RunE:  runPush,
}

func runPush(cmd *cobra.Command, args []string) error {
	// Check for first-run setup
	hasProfiles, err := config.HasProfiles()
	if err != nil {
		return fmt.Errorf("failed to check profiles: %w", err)
	}

	if !hasProfiles {
		if err := firstRunSetup(); err != nil {
			return err
		}
	}

	// Get default profile
	profile, err := config.GetDefaultProfile()
	if err != nil {
		return fmt.Errorf("failed to get default profile: %w", err)
	}

	// Initialize Wi-Fi manager
	wifiMgr := wifi.NewManager()

	// Save current Wi-Fi
	fmt.Println("[*] Detecting current Wi-Fi...")
	currentSSID, err := wifiMgr.GetCurrentSSID()
	if err != nil {
		return fmt.Errorf("failed to get current Wi-Fi: %w", err)
	}

	if currentSSID != "" {
		fmt.Printf("[~] Saving current Wi-Fi: %s\n", currentSSID)
		if err := config.SaveLastWiFi(currentSSID); err != nil {
			fmt.Printf("[!] Warning: failed to save Wi-Fi state: %v\n", err)
		}
	}

	// New flow:
	// 1) Check current en0 IP. If already 192.168.43.x, skip Wi-Fi connect.
	// 2) Otherwise connect, wait, and re-check. If still not in subnet, exit.

	ip, err := wifiMgr.GetIPv4()
	if err != nil {
		return fmt.Errorf("failed to read Wi-Fi IP address: %w", err)
	}
	if ip != "" {
		fmt.Printf("[*] Current Wi-Fi IPv4 on en0: %s\n", ip)
	}

	onRobotNet, err := wifiMgr.IsOnRobotNetwork()
	if err != nil {
		return fmt.Errorf("failed to verify robot network: %w", err)
	}

	if !onRobotNet {
		// Not yet on robot subnet – perform a connect, then re-check.
		fmt.Printf("\n[>] Connecting to robot Wi-Fi: %s\n", profile.SSID)
		fmt.Println("[!] Note: If connection fails, you may need to run with 'sudo pusher'")
		if err := wifiMgr.ConnectWithRetry(profile.SSID, profile.Password, 3); err != nil {
			return fmt.Errorf("failed to connect to robot Wi-Fi: %w\n\n[!] Tip: Try running 'sudo pusher' if you're getting permission errors", err)
		}

		fmt.Println("[*] Waiting 10 seconds for network to stabilize...")
		time.Sleep(10 * time.Second)

		ip, err = wifiMgr.GetIPv4()
		if err != nil {
			return fmt.Errorf("failed to read Wi-Fi IP address after connect: %w", err)
		}
		fmt.Printf("[*] Wi-Fi IPv4 on en0 after connect: %s\n", ip)

		onRobotNet, err = wifiMgr.IsOnRobotNetwork()
		if err != nil {
			return fmt.Errorf("failed to verify robot network after connect: %w", err)
		}
		if !onRobotNet {
			return fmt.Errorf("Wi-Fi IP %s is not in robot subnet 192.168.43.x after connect - exiting", ip)
		}
	} else {
		fmt.Println("[>] Already on robot subnet (192.168.43.x), skipping Wi-Fi connect")
	}

	fmt.Println("[OK] Ready on robot Wi-Fi (192.168.43.x)")

	// Connect via ADB
	fmt.Println("\n[+] Connecting to robot via ADB...")
	if !adb.IsInstalled() {
		return fmt.Errorf("adb not found - please install Android SDK Platform-Tools")
	}

	if err := adb.Connect(); err != nil {
		return fmt.Errorf("failed to connect via ADB: %w", err)
	}
	fmt.Println("[OK] Connected via ADB")

	// Detect Gradle wrapper
	fmt.Println("\n[*] Detecting Gradle wrapper...")
	gradlePath, err := gradle.DetectWrapper()
	if err != nil {
		return fmt.Errorf("failed to detect Gradle wrapper: %w", err)
	}
	fmt.Printf("[OK] Found Gradle wrapper: %s\n", gradlePath)

	// Build and deploy
	fmt.Println("\n[#] Building and deploying...")
	fmt.Println("─────────────────────────────────────────")

	if err := gradle.Build(gradlePath, os.Stdout); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	fmt.Println("─────────────────────────────────────────")
	fmt.Println("\n[OK] Deployment complete!")
	fmt.Println("\nYour app has been successfully built and deployed to the robot.")

	return nil
}

func firstRunSetup() error {
	fmt.Println("Welcome to Pusher!")
	fmt.Println("\nNo robot profiles found. Let's set one up.")

	reader := bufio.NewReader(os.Stdin)

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

	// Save as default profile
	if err := config.AddProfile("default", ssid, password); err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}

	fmt.Println("\n[OK] Profile saved as 'default'")
	fmt.Println()

	return nil
}
