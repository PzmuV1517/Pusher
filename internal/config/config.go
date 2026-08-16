package config

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/spf13/viper"
)

// Profile is one saved robot: a name, its Wi-Fi and the password.
type Profile struct {
	Name     string `mapstructure:"name"`
	SSID     string `mapstructure:"ssid"`
	Password string `mapstructure:"password"`
}

// Config is everything pusher remembers between runs.
type Config struct {
	DefaultProfile string              `mapstructure:"default_profile"`
	Profiles       map[string]*Profile `mapstructure:"profiles"`
	LastWiFi       string              `mapstructure:"last_wifi"`
	Threads        int                 `mapstructure:"threads"`

	HomeSSID string `mapstructure:"home_ssid"`

	SwitchBack bool `mapstructure:"switch_back"`

	PreferUSB bool `mapstructure:"prefer_usb"`

	AutoSlim bool `mapstructure:"auto_slim"`

	DeltaTransfer bool `mapstructure:"delta_transfer"`

	HubABI string `mapstructure:"hub_abi"`

	SkipUnchanged bool `mapstructure:"skip_unchanged"`

	StreamInstall bool `mapstructure:"stream_install"`

	StoreLibs bool `mapstructure:"store_libs"`

	SplitInstall bool `mapstructure:"split_install"`

	Extreme bool `mapstructure:"extreme"`

	DashWatch bool `mapstructure:"dash_watch"`
}

var (
	configDir  string
	configFile string
)

// Initialize locates the config file and loads it, creating one if needed.
func Initialize() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	if os.Geteuid() == 0 {
		if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
			if u, err := user.Lookup(sudoUser); err == nil && u != nil && u.HomeDir != "" {
				home = u.HomeDir
			}
		}
	}

	configDir = filepath.Join(home, ".config", "pusher")
	configFile = filepath.Join(configDir, "config.yaml")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	viper.SetConfigFile(configFile)
	viper.SetConfigType("yaml")

	viper.SetDefault("default_profile", "")
	viper.SetDefault("profiles", map[string]*Profile{})
	viper.SetDefault("last_wifi", "")
	viper.SetDefault("threads", 8)
	viper.SetDefault("home_ssid", "")
	viper.SetDefault("switch_back", true)
	viper.SetDefault("prefer_usb", true)
	viper.SetDefault("auto_slim", false)
	viper.SetDefault("delta_transfer", true)
	viper.SetDefault("hub_abi", "")
	viper.SetDefault("skip_unchanged", true)
	viper.SetDefault("stream_install", true)
	viper.SetDefault("store_libs", false)
	viper.SetDefault("split_install", false)
	viper.SetDefault("extreme", false)
	viper.SetDefault("telemetry", true)

	if _, err := os.Stat(configFile); os.IsNotExist(err) {

		if err := viper.WriteConfigAs(configFile); err != nil {
			return fmt.Errorf("failed to create config file: %w", err)
		}
	} else {

		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

	return nil
}

// Load reads the config.
func Load() (*Config, error) {
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return &cfg, nil
}

// Save writes the config back.
func Save(cfg *Config) error {
	viper.Set("default_profile", cfg.DefaultProfile)
	viper.Set("profiles", cfg.Profiles)
	viper.Set("last_wifi", cfg.LastWiFi)
	viper.Set("threads", cfg.Threads)
	viper.Set("home_ssid", cfg.HomeSSID)
	viper.Set("switch_back", cfg.SwitchBack)
	viper.Set("prefer_usb", cfg.PreferUSB)
	viper.Set("auto_slim", cfg.AutoSlim)
	viper.Set("delta_transfer", cfg.DeltaTransfer)
	viper.Set("hub_abi", cfg.HubABI)
	viper.Set("skip_unchanged", cfg.SkipUnchanged)
	viper.Set("stream_install", cfg.StreamInstall)
	viper.Set("store_libs", cfg.StoreLibs)
	viper.Set("split_install", cfg.SplitInstall)
	viper.Set("extreme", cfg.Extreme)

	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// AddProfile stores a robot profile, making it the default if it is the first.
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

	if cfg.DefaultProfile == "" {
		cfg.DefaultProfile = name
	}

	return Save(cfg)
}

// GetDefaultProfile returns the profile deploys use.
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

// SetDefaultProfile chooses which profile deploys use.
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

// SaveLastWiFi records the network pusher last saw.
func SaveLastWiFi(ssid string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}

	cfg.LastWiFi = ssid
	return Save(cfg)
}

// GetLastWiFi returns the network pusher last saw.
func GetLastWiFi() (string, error) {
	cfg, err := Load()
	if err != nil {
		return "", err
	}
	return cfg.LastWiFi, nil
}

// ConfigExists reports whether a config file has been written.
func ConfigExists() bool {
	_, err := os.Stat(configFile)
	return err == nil
}

// HasProfiles reports whether any robot has been set up.
func HasProfiles() (bool, error) {
	cfg, err := Load()
	if err != nil {
		return false, err
	}
	return len(cfg.Profiles) > 0, nil
}

// GetThreads is how many workers Gradle may use.
func GetThreads() int {
	threads := viper.GetInt("threads")
	if threads <= 0 {
		return 8
	}
	return threads
}

// SetThreads limits how many workers Gradle may use.
func SetThreads(count int) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.Threads = count
	return Save(cfg)
}

// ResetThreads puts the Gradle worker count back to the default.
func ResetThreads() error {
	return SetThreads(8)
}

// GetHomeSSID is the network to return to after deploying.
func GetHomeSSID() string {
	return viper.GetString("home_ssid")
}

// SetHomeSSID sets the network to return to after deploying.
func SetHomeSSID(ssid string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.HomeSSID = ssid
	return Save(cfg)
}

// GetSwitchBack reports whether pusher returns to your own network after a deploy.
func GetSwitchBack() bool {
	return viper.GetBool("switch_back")
}

// SetSwitchBack controls whether pusher returns to your own network.
func SetSwitchBack(enabled bool) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.SwitchBack = enabled
	return Save(cfg)
}

// GetPreferUSB reports whether an attached hub is used in preference to Wi-Fi.
func GetPreferUSB() bool {
	return viper.GetBool("prefer_usb")
}

// SetPreferUSB controls whether an attached hub is preferred over Wi-Fi.
func SetPreferUSB(enabled bool) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.PreferUSB = enabled
	return Save(cfg)
}

// GetAutoSlim reports whether every push slims the APK first.
func GetAutoSlim() bool {
	return viper.GetBool("auto_slim")
}

// SetAutoSlim controls whether every push slims the APK first.
func SetAutoSlim(enabled bool) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.AutoSlim = enabled
	return Save(cfg)
}

// GetDeltaTransfer reports whether only changed parts of the APK are sent.
func GetDeltaTransfer() bool {
	return viper.GetBool("delta_transfer")
}

// SetDeltaTransfer controls whether only changed parts of the APK are sent.
func SetDeltaTransfer(enabled bool) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.DeltaTransfer = enabled
	return Save(cfg)
}

// GetInstallKey returns this install's key, empty on a fresh device.
func GetInstallKey() string {
	return viper.GetString("install_key")
}

// SetInstallKey records this install's key.
func SetInstallKey(key string) error {
	viper.Set("install_key", key)
	return viper.WriteConfig()
}

// GetDeviceID returns this device's random identifier, empty until first use.
func GetDeviceID() string {
	return viper.GetString("device_id")
}

// SetDeviceID records this device's random identifier.
func SetDeviceID(id string) error {
	viper.Set("device_id", id)
	return viper.WriteConfig()
}

// GetTelemetry reports whether this device may be counted.
func GetTelemetry() bool {
	return viper.GetBool("telemetry")
}

// SetTelemetry turns the device count on or off.
func SetTelemetry(enabled bool) error {
	viper.Set("telemetry", enabled)
	return viper.WriteConfig()
}

// GetLastPing is when this device was last counted, as RFC 3339.
func GetLastPing() string {
	return viper.GetString("last_ping")
}

// SetLastPing records when this device was counted.
func SetLastPing(when string) error {
	viper.Set("last_ping", when)
	return viper.WriteConfig()
}

// GetHubABI is the CPU architecture the hub was last seen running.
func GetHubABI() string {
	return viper.GetString("hub_abi")
}

// SetHubABI records the CPU architecture the hub runs.
func SetHubABI(abi string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.HubABI = abi
	return Save(cfg)
}

// DeleteProfile removes a robot profile, picking a new default if needed.
func DeleteProfile(name string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}

	if _, ok := cfg.Profiles[name]; !ok {
		return fmt.Errorf("profile '%s' not found", name)
	}

	delete(cfg.Profiles, name)

	if cfg.DefaultProfile == name {
		cfg.DefaultProfile = ""
		for remaining := range cfg.Profiles {
			cfg.DefaultProfile = remaining
			break
		}
	}

	return Save(cfg)
}

// GetSkipUnchanged reports whether an install is skipped when the robot already has this build.
func GetSkipUnchanged() bool { return viper.GetBool("skip_unchanged") }

// SetSkipUnchanged controls whether an unchanged build is installed again.
func SetSkipUnchanged(enabled bool) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.SkipUnchanged = enabled
	return Save(cfg)
}

// GetStreamInstall reports whether the APK is streamed into an install session.
func GetStreamInstall() bool { return viper.GetBool("stream_install") }

// SetStreamInstall controls whether the APK is streamed into an install session.
func SetStreamInstall(enabled bool) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.StreamInstall = enabled
	return Save(cfg)
}

// GetStoreLibs reports whether slim also stores native libraries uncompressed.
func GetStoreLibs() bool { return viper.GetBool("store_libs") }

// SetStoreLibs controls whether slim stores native libraries uncompressed.
func SetStoreLibs(enabled bool) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.StoreLibs = enabled
	return Save(cfg)
}

// GetSplitInstall reports whether only changed split APKs are installed.
func GetSplitInstall() bool { return viper.GetBool("split_install") }

// SetSplitInstall controls whether only changed split APKs are installed.
func SetSplitInstall(enabled bool) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.SplitInstall = enabled
	return Save(cfg)
}

// GetExtreme reports whether a deploy reloads team code instead of installing
// an APK, when that is equivalent.
func GetExtreme() bool { return viper.GetBool("extreme") }

func SetExtreme(enabled bool) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.Extreme = enabled
	return Save(cfg)
}

// GetDashWatch reports whether a deploy reads the dashboard before and after,
// to say what tuning it threw away.
func GetDashWatch() bool { return viper.GetBool("dash_watch") }

// SetDashWatch controls the tuning check around a deploy.
func SetDashWatch(enabled bool) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.DashWatch = enabled
	return Save(cfg)
}

// Dir is where pusher keeps everything it remembers.
func Dir() string { return configDir }
