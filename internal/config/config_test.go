package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/spf13/viper"
)

func setupTest(t *testing.T) (cleanup func()) {

	tmpDir := t.TempDir()

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)

	viper.Reset()

	err := Initialize()
	if err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	return func() {
		os.Setenv("HOME", originalHome)
		viper.Reset()
	}
}

func TestInitialize(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	configPath := filepath.Join(os.Getenv("HOME"), ".config", "pusher")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("Config directory was not created: %s", configPath)
	}

	configFile := filepath.Join(configPath, "config.yaml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Errorf("Config file was not created: %s", configFile)
	}
}

func TestAddProfile(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	err := AddProfile("test-robot", "DIRECT-Robot", "password123")
	if err != nil {
		t.Fatalf("AddProfile() failed: %v", err)
	}

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

	if cfg.DefaultProfile != "test-robot" {
		t.Errorf("Expected default profile 'test-robot', got '%s'", cfg.DefaultProfile)
	}
}

func TestGetDefaultProfile(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	_, err := GetDefaultProfile()
	if err == nil {
		t.Error("Expected error when no default profile exists")
	}

	err = AddProfile("test-robot", "DIRECT-Robot", "password")
	if err != nil {
		t.Fatalf("AddProfile() failed: %v", err)
	}

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

	AddProfile("robot1", "SSID1", "pass1")
	AddProfile("robot2", "SSID2", "pass2")

	err := SetDefaultProfile("robot2")
	if err != nil {
		t.Fatalf("SetDefaultProfile() failed: %v", err)
	}

	profile, err := GetDefaultProfile()
	if err != nil {
		t.Fatalf("GetDefaultProfile() failed: %v", err)
	}

	if profile.Name != "robot2" {
		t.Errorf("Expected default profile 'robot2', got '%s'", profile.Name)
	}

	err = SetDefaultProfile("nonexistent")
	if err == nil {
		t.Error("Expected error when setting non-existent profile as default")
	}
}

func TestSaveLastWiFi(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	err := SaveLastWiFi("MyHomeNetwork")
	if err != nil {
		t.Fatalf("SaveLastWiFi() failed: %v", err)
	}

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

	hasProfiles, err := HasProfiles()
	if err != nil {
		t.Fatalf("HasProfiles() failed: %v", err)
	}

	if hasProfiles {
		t.Error("Expected false when no profiles exist")
	}

	err = AddProfile("test", "SSID", "pass")
	if err != nil {
		t.Fatalf("AddProfile() failed: %v", err)
	}

	hasProfiles, err = HasProfiles()
	if err != nil {
		t.Fatalf("HasProfiles() failed: %v", err)
	}

	if !hasProfiles {
		t.Error("Expected true when profiles exist")
	}
}

// Save writes a hand-maintained list of keys while Config is a struct, and the
// two drifted: dash_watch was added to the struct, given a getter and a setter,
// and never written. The setting reported itself as changed and came back off
// on the next read, because the file never held it.
//
// So this walks the struct rather than naming fields. A new setting that Save
// forgets fails here instead of in somebody's menu.
func TestEverySettingSurvivesASave(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	// Every value is moved off whatever the default is, so a key that never
	// reaches the file reads back as the default and fails to match.
	want := map[string]any{}
	fields := reflect.ValueOf(cfg).Elem()

	for i := 0; i < fields.NumField(); i++ {
		field := fields.Field(i)
		name := fields.Type().Field(i).Tag.Get("mapstructure")

		switch field.Kind() {
		case reflect.Bool:
			field.SetBool(!field.Bool())
			want[name] = field.Bool()
		case reflect.String:
			field.SetString("saved-" + name)
			want[name] = field.String()
		case reflect.Int:
			field.SetInt(field.Int() + 7)
			want[name] = field.Int()
		}
	}

	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	// Read the file back the way a later run would, rather than trusting what
	// is still sitting in memory from the write.
	viper.Reset()
	if err := Initialize(); err != nil {
		t.Fatal(err)
	}

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	reloaded := reflect.ValueOf(got).Elem()
	for i := 0; i < reloaded.NumField(); i++ {
		name := reloaded.Type().Field(i).Tag.Get("mapstructure")

		expected, checked := want[name]
		if !checked {
			continue
		}

		var actual any
		switch field := reloaded.Field(i); field.Kind() {
		case reflect.Bool:
			actual = field.Bool()
		case reflect.String:
			actual = field.String()
		case reflect.Int:
			actual = field.Int()
		}

		if actual != expected {
			t.Errorf("%s came back as %v, want %v: Save does not write it", name, actual, expected)
		}
	}
}
