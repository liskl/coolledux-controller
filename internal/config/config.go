package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// otelEndpointEnvVars enumerates the OTLP endpoint env vars the SDK reads
// natively when no WithEndpoint(...) option is passed. Used by Validate()
// to satisfy the "endpoint is set" requirement without forcing it through
// our own config keys.
var otelEndpointEnvVars = []string{
	"OTEL_EXPORTER_OTLP_ENDPOINT",
	"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
	"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
	"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT",
}

// Config holds the complete application configuration.
type Config struct {
	BLE     BLEConfig     `mapstructure:"ble"`
	Display DisplayConfig `mapstructure:"display"`
	MQTT    MQTTConfig    `mapstructure:"mqtt"`
	API     APIConfig     `mapstructure:"api"`
	Log     LogConfig     `mapstructure:"log"`
	OTel    OTelConfig    `mapstructure:"otel"`
}

// BLEConfig holds Bluetooth Low Energy connection settings.
//
// Multi-device model: populate the Devices list to manage multiple panels.
// The legacy DeviceMAC field still works — if set and Devices is empty it's
// treated as a single-device configuration. Devices also wins if both are
// set (legacy field is ignored).
//
// ExcludeMACs is a block list that wins over everything: any MAC here is
// filtered out of both the static Devices list and scan discoveries before
// registration. Use it to keep the service from driving a neighbour's
// sign, or a panel you've temporarily pulled from the fleet.
type BLEConfig struct {
	DeviceName        string         `mapstructure:"device_name"`
	DeviceMAC         string         `mapstructure:"device_mac"`
	Devices           []DeviceConfig `mapstructure:"devices"`
	ExcludeMACs       []string       `mapstructure:"exclude_macs"`
	ScanOnStartup     *bool          `mapstructure:"scan_on_startup"`
	ServiceUUID       string         `mapstructure:"service_uuid"`
	CharUUID          string         `mapstructure:"char_uuid"`
	DeviceServiceUUID string         `mapstructure:"device_service_uuid"`
	ScanTimeout       time.Duration  `mapstructure:"scan_timeout"`
	ReconnectInterval time.Duration  `mapstructure:"reconnect_interval"`
}

// IsExcluded reports whether the given MAC appears in ExcludeMACs.
// Comparison is case-insensitive and ignores colon separators, so the
// same MAC written "01:00:00:FB:A4:16", "01-00-00-FB-A4-16", or
// "010000fba416" all match.
func (b *BLEConfig) IsExcluded(mac string) bool {
	if mac == "" {
		return false
	}
	target := NormalizeMAC(mac)
	for _, excl := range b.ExcludeMACs {
		if NormalizeMAC(excl) == target {
			return true
		}
	}
	return false
}

// DeviceConfig is one entry in BLEConfig.Devices.
type DeviceConfig struct {
	Name string `mapstructure:"name"` // Friendly label (optional).
	MAC  string `mapstructure:"mac"`
}

// ResolveDevices returns the effective device list: if the new Devices
// slice is non-empty, it's used as-is; otherwise the legacy DeviceMAC is
// wrapped into a single entry. Returns an empty slice if neither is set,
// which is a valid "scan only" configuration. Excluded MACs are dropped
// before returning so callers don't need to re-apply the filter.
func (b *BLEConfig) ResolveDevices() []DeviceConfig {
	var raw []DeviceConfig
	switch {
	case len(b.Devices) > 0:
		raw = b.Devices
	case b.DeviceMAC != "":
		raw = []DeviceConfig{{MAC: b.DeviceMAC}}
	default:
		return nil
	}
	out := make([]DeviceConfig, 0, len(raw))
	for _, d := range raw {
		if b.IsExcluded(d.MAC) {
			continue
		}
		out = append(out, d)
	}
	return out
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

// OTelConfig holds OpenTelemetry configuration. When Enabled is false,
// the service runs with no-op tracer/meter/logger providers and no
// network traffic — the feature is entirely opt-in.
//
// Endpoint has no default: you must set it explicitly when enabling
// OTel. Typical values:
//
//   - In-cluster DaemonSet (gRPC):  otel-daemonset-collector.observability.svc.cluster.local:4317
//   - In-cluster Gateway (gRPC):    otel-gateway-collector.observability.svc.cluster.local:4317
//   - Same via OTLP HTTP:           http://otel-gateway-collector.observability.svc.cluster.local:4318
//   - External (once exposed):      https://otel.home.liskl.com:4317
//
// Per-signal sub-blocks (Traces, Metrics, Logs) can override the
// top-level Endpoint/Protocol when a single collector doesn't fan out
// to every signal; leave them blank to inherit.
type OTelConfig struct {
	Enabled        bool              `mapstructure:"enabled"`
	ServiceName    string            `mapstructure:"service_name"`
	ServiceVersion string            `mapstructure:"service_version"`
	Endpoint       string            `mapstructure:"endpoint"`
	Protocol       string            `mapstructure:"protocol"` // "grpc" | "http"
	Insecure       bool              `mapstructure:"insecure"`
	Headers        map[string]string `mapstructure:"headers"`
	Timeout        time.Duration     `mapstructure:"timeout"`
	ResourceAttrs  map[string]string `mapstructure:"resource_attributes"`
	Traces         OTelSignalConfig  `mapstructure:"traces"`
	Metrics        OTelMetricsConfig `mapstructure:"metrics"`
	Logs           OTelSignalConfig  `mapstructure:"logs"`
}

// OTelSignalConfig is the per-signal knob shared by traces/logs and
// embedded by metrics. Endpoint/Protocol override the parent OTelConfig
// values when non-empty.
type OTelSignalConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Endpoint string `mapstructure:"endpoint"`
	Protocol string `mapstructure:"protocol"`
}

// OTelMetricsConfig adds metrics-specific settings on top of the shared
// signal config.
type OTelMetricsConfig struct {
	OTelSignalConfig `mapstructure:",squash"`
	// Interval is the metric export period. The SDK's default is 60s.
	Interval time.Duration `mapstructure:"interval"`
	// Runtime enables Go runtime metrics (heap, goroutines, GC, etc.)
	// via go.opentelemetry.io/contrib/instrumentation/runtime.
	Runtime bool `mapstructure:"runtime"`
}

// ResolveEndpoint returns the effective endpoint for a given signal: the
// per-signal override if set, else the top-level OTelConfig endpoint.
func (o *OTelConfig) ResolveEndpoint(signal OTelSignalConfig) string {
	if signal.Endpoint != "" {
		return signal.Endpoint
	}
	return o.Endpoint
}

// ResolveProtocol returns the effective protocol for a given signal: the
// per-signal override if set, else the top-level OTelConfig protocol.
func (o *OTelConfig) ResolveProtocol(signal OTelSignalConfig) string {
	if signal.Protocol != "" {
		return signal.Protocol
	}
	return o.Protocol
}

// endpointConfigured reports whether any of the OTel endpoint sources
// is set: YAML/Viper-derived top-level or per-signal endpoint, or one of
// the standard OTLP env vars the SDK reads natively.
func (o *OTelConfig) endpointConfigured() bool {
	if o.Endpoint != "" || o.Traces.Endpoint != "" || o.Metrics.Endpoint != "" || o.Logs.Endpoint != "" {
		return true
	}
	for _, name := range otelEndpointEnvVars {
		if os.Getenv(name) != "" {
			return true
		}
	}
	return false
}

// Validate checks invariants that would make OTel init fail late: an
// enabled provider with no endpoint is the biggest footgun, so surface
// it at config-load time instead of after the first span attempts a
// connect.
//
// Endpoint resolution honors the standard OTel SDK fallback: if no
// endpoint is set in YAML/COOLLEDUX env vars, the SDK reads
// OTEL_EXPORTER_OTLP_ENDPOINT (and per-signal variants) at exporter
// construction time. Validate accepts presence of any of those env vars
// as satisfying the "endpoint is configured" requirement.
//
// Precedence (highest wins): COOLLEDUX_OTEL_* > YAML > OTEL_EXPORTER_OTLP_*
// > SDK defaults (no endpoint, exporter ultimately fails to dial).
// Viper applies env-over-config because Load() registers AutomaticEnv
// before ReadInConfig.
func (o *OTelConfig) Validate() error {
	if !o.Enabled {
		return nil
	}
	if !o.endpointConfigured() {
		return fmt.Errorf("otel.enabled is true but no endpoint is configured: " +
			"set otel.endpoint (or a per-signal endpoint) in YAML, " +
			"or export OTEL_EXPORTER_OTLP_ENDPOINT (or per-signal " +
			"OTEL_EXPORTER_OTLP_{TRACES,METRICS,LOGS}_ENDPOINT). " +
			"See docs/operations/observability.md")
	}
	switch o.Protocol {
	case "", "grpc", "http":
		// OK (empty only allowed if every signal overrides).
	default:
		return fmt.Errorf("otel.protocol %q is invalid; must be \"grpc\" or \"http\"", o.Protocol)
	}
	for _, s := range []struct {
		name string
		sig  OTelSignalConfig
	}{
		{"traces", o.Traces},
		{"metrics", o.Metrics.OTelSignalConfig},
		{"logs", o.Logs},
	} {
		switch s.sig.Protocol {
		case "", "grpc", "http":
		default:
			return fmt.Errorf("otel.%s.protocol %q is invalid; must be \"grpc\" or \"http\"", s.name, s.sig.Protocol)
		}
	}
	return nil
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

	// OpenTelemetry: disabled by default. No endpoint default — must be set
	// explicitly when enabling. See docs/operations/observability.md.
	v.SetDefault("otel.enabled", false)
	v.SetDefault("otel.service_name", "coolledux-controller")
	v.SetDefault("otel.protocol", "grpc")
	v.SetDefault("otel.insecure", true)
	v.SetDefault("otel.timeout", "10s")
	v.SetDefault("otel.traces.enabled", true)
	v.SetDefault("otel.metrics.enabled", true)
	v.SetDefault("otel.metrics.interval", "60s")
	v.SetDefault("otel.metrics.runtime", true)
	v.SetDefault("otel.logs.enabled", true)

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

	if err := cfg.OTel.Validate(); err != nil {
		return nil, fmt.Errorf("otel config: %w", err)
	}

	return &cfg, nil
}
