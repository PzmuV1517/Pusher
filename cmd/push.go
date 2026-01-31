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
	// Require that we're already on the robot Wi-Fi (192.168.43.x).
	// We no longer try to switch networks automatically.
	wifiMgr := wifi.NewManager()
	ip, err := wifiMgr.GetIPv4()
	if err != nil {
		return fmt.Errorf("failed to read Wi-Fi IP address: %w", err)
	}
	if ip == "" {
		return fmt.Errorf("robot Wi-Fi not detected (no IPv4 on en0). Please connect to the robot's Wi-Fi (192.168.43.x) and rerun 'pusher'")
	}

	onRobotNet, err := wifiMgr.IsOnRobotNetwork()
	if err != nil {
		return fmt.Errorf("failed to verify robot network: %w", err)
	}
	if !onRobotNet {
		return fmt.Errorf("robot Wi-Fi not detected: IPv4 on en0 is %s (expected 192.168.43.x). Please connect to the robot's Wi-Fi and rerun 'pusher'", ip)
	}

	fmt.Println("[OK] Robot Wi-Fi detected (192.168.43.x)")

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

	// Build (assembleDebug only - faster than including installDebug)
	fmt.Println("\n[#] Building...")
	fmt.Println("─────────────────────────────────────────")

	if err := gradle.Build(gradlePath, os.Stdout); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	fmt.Println("─────────────────────────────────────────")

	// Find the built APK
	wrapperDir := gradlePath[:len(gradlePath)-len("/gradlew")]
	if wrapperDir == "." {
		wrapperDir, _ = os.Getwd()
	}
	apkPath, err := gradle.FindApk(wrapperDir)
	if err != nil {
		return fmt.Errorf("failed to find APK: %w", err)
	}
	fmt.Printf("\n[*] Found APK: %s\n", apkPath)

	// Install via ADB (faster than Gradle's installDebug)
	fmt.Println("[*] Installing APK via ADB...")
	installStart := time.Now()
	if err := adb.Install(apkPath); err != nil {
		return fmt.Errorf("install failed: %w", err)
	}
	installDuration := time.Since(installStart)

	fmt.Printf("\n[OK] Deployment complete! (install took %.1fs)\n", installDuration.Seconds())
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
