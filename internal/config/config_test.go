package config

import (
	"os"
	"path/filepath"
	"testing"
	
	"github.com/spf13/viper"
)

func setupTest(t *testing.T) (cleanup func()) {
	// Create temp directory
	tmpDir := t.TempDir()
	
	// Save and override HOME
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	
	// Reset viper
	viper.Reset()
	
	// Initialize config
	err := Initialize()
	if err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}
	
	// Return cleanup function
	return func() {
		os.Setenv("HOME", originalHome)
		viper.Reset()
	}
}

func TestInitialize(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()
	
	// Check if config directory was created
	configPath := filepath.Join(os.Getenv("HOME"), ".config", "pusher")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("Config directory was not created: %s", configPath)
	}
	
	// Check if config file was created
	configFile := filepath.Join(configPath, "config.yaml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Errorf("Config file was not created: %s", configFile)
	}
}

func TestAddProfile(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()
	
	// Test adding a profile
	err := AddProfile("test-robot", "DIRECT-Robot", "password123")
	if err != nil {
		t.Fatalf("AddProfile() failed: %v", err)
	}
	
	// Verify profile was added
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	
	profile, exists := cfg.Profiles["test-robot"]
	if !exists {
		t.Error("Profile was not added")
	}
	
	if profile.SSID != "DIRECT-Robot" {
		t.Errorf("Expected SSID 'DIRECT-Robot', got '%s'", profile.SSID)
	}
	
	if profile.Password != "password123" {
		t.Errorf("Expected password 'password123', got '%s'", profile.Password)
	}
	
	// First profile should become default
	if cfg.DefaultProfile != "test-robot" {
		t.Errorf("Expected default profile 'test-robot', got '%s'", cfg.DefaultProfile)
	}
}

func TestGetDefaultProfile(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()
	
	// Test with no profiles - should get error
	_, err := GetDefaultProfile()
	if err == nil {
		t.Error("Expected error when no default profile exists")
	}

	// Add a profile - it should become default automatically
	err = AddProfile("test-robot", "DIRECT-Robot", "password")
	if err != nil {
		t.Fatalf("AddProfile() failed: %v", err)
	}

	// Get default profile - should be the one we just added
	profile, err := GetDefaultProfile()
	if err != nil {
		t.Fatalf("GetDefaultProfile() failed: %v", err)
	}

	if profile.Name != "test-robot" {
		t.Errorf("Expected profile name 'test-robot', got '%s'", profile.Name)
	}
}

func TestSetDefaultProfile(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()
	
	// Add two profiles
	AddProfile("robot1", "SSID1", "pass1")
	AddProfile("robot2", "SSID2", "pass2")
	
	// Set robot2 as default
	err := SetDefaultProfile("robot2")
	if err != nil {
		t.Fatalf("SetDefaultProfile() failed: %v", err)
	}
	
	// Verify
	profile, err := GetDefaultProfile()
	if err != nil {
		t.Fatalf("GetDefaultProfile() failed: %v", err)
	}
	
	if profile.Name != "robot2" {
		t.Errorf("Expected default profile 'robot2', got '%s'", profile.Name)
	}
	
	// Test setting non-existent profile
	err = SetDefaultProfile("nonexistent")
	if err == nil {
		t.Error("Expected error when setting non-existent profile as default")
	}
}

func TestSaveLastWiFi(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()
	
	// Save Wi-Fi
	err := SaveLastWiFi("MyHomeNetwork")
	if err != nil {
		t.Fatalf("SaveLastWiFi() failed: %v", err)
	}
	
	// Verify
	lastWiFi, err := GetLastWiFi()
	if err != nil {
		t.Fatalf("GetLastWiFi() failed: %v", err)
	}
	
	if lastWiFi != "MyHomeNetwork" {
		t.Errorf("Expected last WiFi 'MyHomeNetwork', got '%s'", lastWiFi)
	}
}

func TestHasProfiles(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()
	
	// Test with no profiles
	hasProfiles, err := HasProfiles()
	if err != nil {
		t.Fatalf("HasProfiles() failed: %v", err)
	}

	if hasProfiles {
		t.Error("Expected false when no profiles exist")
	}

	// Add a profile
	err = AddProfile("test", "SSID", "pass")
	if err != nil {
		t.Fatalf("AddProfile() failed: %v", err)
	}

	// Test with profiles
	hasProfiles, err = HasProfiles()
	if err != nil {
		t.Fatalf("HasProfiles() failed: %v", err)
	}
	
	if !hasProfiles {
		t.Error("Expected true when profiles exist")
	}
}
