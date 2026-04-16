package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds the complete application configuration.
type Config struct {
	BLE     BLEConfig     `mapstructure:"ble"`
	Display DisplayConfig `mapstructure:"display"`
	MQTT    MQTTConfig    `mapstructure:"mqtt"`
	API     APIConfig     `mapstructure:"api"`
	Log     LogConfig     `mapstructure:"log"`
}

// BLEConfig holds Bluetooth Low Energy connection settings.
//
// Multi-device model: populate the Devices list to manage multiple panels.
// The legacy DeviceMAC field still works — if set and Devices is empty it's
// treated as a single-device configuration. Devices also wins if both are
// set (legacy field is ignored).
type BLEConfig struct {
	DeviceName        string         `mapstructure:"device_name"`
	DeviceMAC         string         `mapstructure:"device_mac"`
	Devices           []DeviceConfig `mapstructure:"devices"`
	ScanOnStartup     *bool          `mapstructure:"scan_on_startup"`
	ServiceUUID       string         `mapstructure:"service_uuid"`
	CharUUID          string         `mapstructure:"char_uuid"`
	DeviceServiceUUID string         `mapstructure:"device_service_uuid"`
	ScanTimeout       time.Duration  `mapstructure:"scan_timeout"`
	ReconnectInterval time.Duration  `mapstructure:"reconnect_interval"`
}

// DeviceConfig is one entry in BLEConfig.Devices.
type DeviceConfig struct {
	Name string `mapstructure:"name"` // Friendly label (optional).
	MAC  string `mapstructure:"mac"`
}

// ResolveDevices returns the effective device list: if the new Devices
// slice is non-empty, it's used as-is; otherwise the legacy DeviceMAC is
// wrapped into a single entry. Returns an empty slice if neither is set,
// which is a valid "scan only" configuration.
func (b *BLEConfig) ResolveDevices() []DeviceConfig {
	if len(b.Devices) > 0 {
		return b.Devices
	}
	if b.DeviceMAC != "" {
		return []DeviceConfig{{MAC: b.DeviceMAC}}
	}
	return nil
}

// ScanOnStartupEnabled reports whether the service should run a BLE scan
// at startup and register discovered panels. Default: true when no
// devices are configured, false when they are — users who pinned MACs
// probably don't want surprise auto-registrations.
func (b *BLEConfig) ScanOnStartupEnabled() bool {
	if b.ScanOnStartup != nil {
		return *b.ScanOnStartup
	}
	return len(b.ResolveDevices()) == 0
}

// DisplayConfig holds LED matrix display defaults.
type DisplayConfig struct {
	Columns          int    `mapstructure:"columns"`
	Rows             int    `mapstructure:"rows"`
	DefaultBrightness int   `mapstructure:"default_brightness"`
	DefaultFontSize  int    `mapstructure:"default_font_size"`
	DefaultColor     string `mapstructure:"default_color"`
	DefaultSpeed     int    `mapstructure:"default_speed"`
}

// MQTTConfig holds MQTT broker connection settings.
type MQTTConfig struct {
	Broker             string        `mapstructure:"broker"`
	ClientID           string        `mapstructure:"client_id"`
	Username           string        `mapstructure:"username"`
	Password           string        `mapstructure:"password"`
	TopicPrefix        string        `mapstructure:"topic_prefix"`
	HADiscoveryPrefix  string        `mapstructure:"ha_discovery_prefix"`
	Keepalive          time.Duration `mapstructure:"keepalive"`
}

// APIConfig holds HTTP API server settings.
type APIConfig struct {
	Listen      string   `mapstructure:"listen"`
	CORSOrigins []string `mapstructure:"cors_origins"`
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// DeviceID derives a device identifier from the legacy BLE MAC address
// by stripping colons and lowercasing. For example, "01:00:00:FB:A4:16"
// becomes "010000fba416". Kept for the single-device path; multi-device
// code should use DeviceConfig.ID or NormalizeMAC directly.
func (c *Config) DeviceID() string {
	return NormalizeMAC(c.BLE.DeviceMAC)
}

// ID returns the normalized MAC (lowercase, no colons) for this device.
// Used as the stable identifier in REST paths, MQTT topics, and the
// in-memory device registry.
func (d *DeviceConfig) ID() string {
	return NormalizeMAC(d.MAC)
}

// NormalizeMAC lowercases and strips colons from a BLE MAC address, so
// "01:00:00:FB:A4:16" becomes "010000fba416".
func NormalizeMAC(mac string) string {
	return strings.ToLower(strings.ReplaceAll(mac, ":", ""))
}

// Load reads configuration from a YAML file (if it exists), environment
// variables with COOLLEDUX_ prefix, and built-in defaults. Environment
// variables override file values.
func Load(path string) (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("ble.device_name", "CoolLEDUX")
	v.SetDefault("ble.device_mac", "01:00:00:FB:A4:16")
	v.SetDefault("ble.service_uuid", "0000fff0-0000-1000-8000-00805f9b34fb")
	v.SetDefault("ble.char_uuid", "0000fff1-0000-1000-8000-00805f9b34fb")
	v.SetDefault("ble.device_service_uuid", "9056aa8d-24a1-427e-ae91-b70e0bf992cd")
	v.SetDefault("ble.scan_timeout", "10s")
	v.SetDefault("ble.reconnect_interval", "5s")

	v.SetDefault("display.columns", 96)
	v.SetDefault("display.rows", 16)
	v.SetDefault("display.default_brightness", 128)
	v.SetDefault("display.default_font_size", 16)
	v.SetDefault("display.default_color", "#FFFFFF")
	v.SetDefault("display.default_speed", 5)

	v.SetDefault("mqtt.broker", "tcp://localhost:1883")
	v.SetDefault("mqtt.client_id", "coolledux-controller")
	v.SetDefault("mqtt.username", "")
	v.SetDefault("mqtt.password", "")
	v.SetDefault("mqtt.topic_prefix", "coolledux")
	v.SetDefault("mqtt.ha_discovery_prefix", "homeassistant")
	v.SetDefault("mqtt.keepalive", "30s")

	v.SetDefault("api.listen", ":8080")
	v.SetDefault("api.cors_origins", []string{"*"})

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")

	// Environment variables: COOLLEDUX_BLE_DEVICE_NAME -> ble.device_name
	v.SetEnvPrefix("COOLLEDUX")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// YAML config file
	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			// Only fail if the file was explicitly provided and can't be read.
			// A "not found" for a default path is fine.
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, fmt.Errorf("reading config file: %w", err)
			}
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	return &cfg, nil
}
