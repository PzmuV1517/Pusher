package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Profile represents a robot Wi-Fi profile
type Profile struct {
	Name     string `mapstructure:"name"`
	SSID     string `mapstructure:"ssid"`
	Password string `mapstructure:"password"`
}

// Config represents the application configuration
type Config struct {
	DefaultProfile string              `mapstructure:"default_profile"`
	Profiles       map[string]*Profile `mapstructure:"profiles"`
	LastWiFi       string              `mapstructure:"last_wifi"`
}

var (
	configDir  string
	configFile string
)

// Initialize sets up the config directory and file
func Initialize() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir = filepath.Join(home, ".config", "pusher")
	configFile = filepath.Join(configDir, "config.yaml")

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	viper.SetConfigFile(configFile)
	viper.SetConfigType("yaml")

	// Set defaults
	viper.SetDefault("default_profile", "")
	viper.SetDefault("profiles", map[string]*Profile{})
	viper.SetDefault("last_wifi", "")

	// Check if config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// Create new config file with defaults
		if err := viper.WriteConfigAs(configFile); err != nil {
			return fmt.Errorf("failed to create config file: %w", err)
		}
	} else {
		// Read existing config
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

	return nil
}

// Load returns the current configuration
func Load() (*Config, error) {
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return &cfg, nil
}

// Save writes the configuration to disk
func Save(cfg *Config) error {
	viper.Set("default_profile", cfg.DefaultProfile)
	viper.Set("profiles", cfg.Profiles)
	viper.Set("last_wifi", cfg.LastWiFi)

	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// AddProfile adds or updates a profile
func AddProfile(name, ssid, password string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}

	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]*Profile)
	}

	cfg.Profiles[name] = &Profile{
		Name:     name,
		SSID:     ssid,
		Password: password,
	}

	// If this is the first profile, make it default
	if cfg.DefaultProfile == "" {
		cfg.DefaultProfile = name
	}

	return Save(cfg)
}

// GetDefaultProfile returns the default profile
func GetDefaultProfile() (*Profile, error) {
	cfg, err := Load()
	if err != nil {
		return nil, err
	}

	if cfg.DefaultProfile == "" {
		return nil, fmt.Errorf("no default profile set")
	}

	profile, ok := cfg.Profiles[cfg.DefaultProfile]
	if !ok {
		return nil, fmt.Errorf("default profile '%s' not found", cfg.DefaultProfile)
	}

	return profile, nil
}

// SetDefaultProfile sets the default profile
func SetDefaultProfile(name string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}

	if _, ok := cfg.Profiles[name]; !ok {
		return fmt.Errorf("profile '%s' not found", name)
	}

	cfg.DefaultProfile = name
	return Save(cfg)
}

// SaveLastWiFi saves the last connected Wi-Fi SSID
func SaveLastWiFi(ssid string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}

	cfg.LastWiFi = ssid
	return Save(cfg)
}

// GetLastWiFi returns the last connected Wi-Fi SSID
func GetLastWiFi() (string, error) {
	cfg, err := Load()
	if err != nil {
		return "", err
	}
	return cfg.LastWiFi, nil
}

// ConfigExists checks if a config file exists
func ConfigExists() bool {
	_, err := os.Stat(configFile)
	return err == nil
}

// HasProfiles checks if any profiles are configured
func HasProfiles() (bool, error) {
	cfg, err := Load()
	if err != nil {
		return false, err
	}
	return len(cfg.Profiles) > 0, nil
}
