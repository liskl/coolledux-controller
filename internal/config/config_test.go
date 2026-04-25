package config

import (
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load(''): %v", err)
	}

	// BLE defaults
	bleTests := []struct {
		name string
		got  string
		want string
	}{
		{"DeviceName", cfg.BLE.DeviceName, "CoolLEDUX"},
		{"DeviceMAC", cfg.BLE.DeviceMAC, "01:00:00:FB:A4:16"},
		{"ServiceUUID", cfg.BLE.ServiceUUID, "0000fff0-0000-1000-8000-00805f9b34fb"},
		{"CharUUID", cfg.BLE.CharUUID, "0000fff1-0000-1000-8000-00805f9b34fb"},
		{"DeviceServiceUUID", cfg.BLE.DeviceServiceUUID, "9056aa8d-24a1-427e-ae91-b70e0bf992cd"},
	}
	for _, tt := range bleTests {
		t.Run("BLE/"+tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}

	// BLE duration defaults
	t.Run("BLE/ScanTimeout", func(t *testing.T) {
		if cfg.BLE.ScanTimeout != 10*time.Second {
			t.Errorf("ScanTimeout = %v, want 10s", cfg.BLE.ScanTimeout)
		}
	})
	t.Run("BLE/ReconnectInterval", func(t *testing.T) {
		if cfg.BLE.ReconnectInterval != 5*time.Second {
			t.Errorf("ReconnectInterval = %v, want 5s", cfg.BLE.ReconnectInterval)
		}
	})

	// Display defaults
	displayTests := []struct {
		name string
		got  int
		want int
	}{
		{"Columns", cfg.Display.Columns, 96},
		{"Rows", cfg.Display.Rows, 16},
		{"DefaultBrightness", cfg.Display.DefaultBrightness, 128},
		{"DefaultFontSize", cfg.Display.DefaultFontSize, 16},
		{"DefaultSpeed", cfg.Display.DefaultSpeed, 5},
	}
	for _, tt := range displayTests {
		t.Run("Display/"+tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.want)
			}
		})
	}
	t.Run("Display/DefaultColor", func(t *testing.T) {
		if cfg.Display.DefaultColor != "#FFFFFF" {
			t.Errorf("DefaultColor = %q, want %q", cfg.Display.DefaultColor, "#FFFFFF")
		}
	})

	// MQTT defaults
	mqttTests := []struct {
		name string
		got  string
		want string
	}{
		{"Broker", cfg.MQTT.Broker, "tcp://localhost:1883"},
		{"ClientID", cfg.MQTT.ClientID, "coolledux-controller"},
		{"Username", cfg.MQTT.Username, ""},
		{"Password", cfg.MQTT.Password, ""},
		{"TopicPrefix", cfg.MQTT.TopicPrefix, "coolledux"},
		{"HADiscoveryPrefix", cfg.MQTT.HADiscoveryPrefix, "homeassistant"},
	}
	for _, tt := range mqttTests {
		t.Run("MQTT/"+tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
	t.Run("MQTT/Keepalive", func(t *testing.T) {
		if cfg.MQTT.Keepalive != 30*time.Second {
			t.Errorf("Keepalive = %v, want 30s", cfg.MQTT.Keepalive)
		}
	})

	// API defaults
	t.Run("API/Listen", func(t *testing.T) {
		if cfg.API.Listen != ":8080" {
			t.Errorf("Listen = %q, want %q", cfg.API.Listen, ":8080")
		}
	})
	t.Run("API/CORSOrigins", func(t *testing.T) {
		if len(cfg.API.CORSOrigins) != 1 || cfg.API.CORSOrigins[0] != "*" {
			t.Errorf("CORSOrigins = %v, want [*]", cfg.API.CORSOrigins)
		}
	})

	// Log defaults
	t.Run("Log/Level", func(t *testing.T) {
		if cfg.Log.Level != "info" {
			t.Errorf("Level = %q, want %q", cfg.Log.Level, "info")
		}
	})
	t.Run("Log/Format", func(t *testing.T) {
		if cfg.Log.Format != "json" {
			t.Errorf("Format = %q, want %q", cfg.Log.Format, "json")
		}
	})
}

func TestDeviceID(t *testing.T) {
	tests := []struct {
		name string
		mac  string
		want string
	}{
		{
			name: "default_mac",
			mac:  "01:00:00:FB:A4:16",
			want: "010000fba416",
		},
		{
			name: "all_uppercase",
			mac:  "AA:BB:CC:DD:EE:FF",
			want: "aabbccddeeff",
		},
		{
			name: "no_colons",
			mac:  "112233445566",
			want: "112233445566",
		},
		{
			name: "empty",
			mac:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{BLE: BLEConfig{DeviceMAC: tt.mac}}
			if got := cfg.DeviceID(); got != tt.want {
				t.Errorf("DeviceID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	const envKey = "COOLLEDUX_BLE_DEVICE_MAC"
	const overrideMAC = "AA:BB:CC:DD:EE:FF"

	// Set the env var, then clean up after the test.
	t.Setenv(envKey, overrideMAC)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load(''): %v", err)
	}

	if cfg.BLE.DeviceMAC != overrideMAC {
		t.Errorf("BLE.DeviceMAC = %q, want %q (from env)", cfg.BLE.DeviceMAC, overrideMAC)
	}
}

func TestLoad_InvalidConfigFile(t *testing.T) {
	// Passing a path to a nonexistent file that was explicitly specified should fail.
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent config file, got nil")
	}
}

func TestLoad_DefaultDeviceID(t *testing.T) {
	// Verify the default config produces the expected device ID.
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load(''): %v", err)
	}

	want := "010000fba416"
	if got := cfg.DeviceID(); got != want {
		t.Errorf("default DeviceID() = %q, want %q", got, want)
	}
}

func TestResolveDevices_PrefersDevicesList(t *testing.T) {
	// When both the legacy DeviceMAC and the new Devices list are set,
	// Devices wins — this is the upgrade path for existing config files.
	b := BLEConfig{
		DeviceMAC: "LEGACY:MAC",
		Devices: []DeviceConfig{
			{Name: "a", MAC: "01:00:00:FB:A4:16"},
			{Name: "b", MAC: "01:00:00:FB:A4:17"},
		},
	}
	got := b.ResolveDevices()
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].MAC != "01:00:00:FB:A4:16" || got[1].MAC != "01:00:00:FB:A4:17" {
		t.Errorf("devices = %+v, want the Devices list not legacy", got)
	}
}

func TestResolveDevices_FallsBackToLegacyMAC(t *testing.T) {
	b := BLEConfig{DeviceMAC: "01:00:00:FB:A4:16"}
	got := b.ResolveDevices()
	if len(got) != 1 || got[0].MAC != "01:00:00:FB:A4:16" {
		t.Errorf("devices = %+v, want single legacy entry", got)
	}
}

func TestResolveDevices_EmptyWhenNeitherSet(t *testing.T) {
	b := BLEConfig{}
	if got := b.ResolveDevices(); len(got) != 0 {
		t.Errorf("devices = %+v, want empty", got)
	}
}

func TestScanOnStartupEnabled_Defaults(t *testing.T) {
	tests := []struct {
		name string
		cfg  BLEConfig
		want bool
	}{
		{"no devices, no override -> scan", BLEConfig{}, true},
		{"devices set, no override -> no scan", BLEConfig{Devices: []DeviceConfig{{MAC: "x"}}}, false},
		{"legacy MAC, no override -> no scan", BLEConfig{DeviceMAC: "x"}, false},
		{"override true regardless of devices", BLEConfig{Devices: []DeviceConfig{{MAC: "x"}}, ScanOnStartup: boolPtr(true)}, true},
		{"override false regardless of empty", BLEConfig{ScanOnStartup: boolPtr(false)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.ScanOnStartupEnabled(); got != tt.want {
				t.Errorf("ScanOnStartupEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeviceConfig_ID(t *testing.T) {
	d := DeviceConfig{MAC: "AB:CD:EF:12:34:56"}
	if got := d.ID(); got != "abcdef123456" {
		t.Errorf("ID() = %q, want abcdef123456", got)
	}
}

func TestNormalizeMAC(t *testing.T) {
	if got := NormalizeMAC("AB:CD:EF:12:34:56"); got != "abcdef123456" {
		t.Errorf("got %q, want abcdef123456", got)
	}
	if got := NormalizeMAC(""); got != "" {
		t.Errorf("empty: got %q, want empty", got)
	}
}

func boolPtr(b bool) *bool { return &b }

func TestIsExcluded(t *testing.T) {
	b := BLEConfig{
		ExcludeMACs: []string{
			"01:00:00:9E:3E:75",
			"aabbccddeeff",
		},
	}
	tests := []struct {
		mac  string
		want bool
	}{
		{"01:00:00:9E:3E:75", true},
		{"01:00:00:9e:3e:75", true}, // lowercase hex
		{"0100009e3e75", true},      // no colons
		{"01-00-00-9E-3E-75", true}, // dash separators survive the stripper? — colons only.
		{"AA:BB:CC:DD:EE:FF", true}, // uppercased version of the normalized entry
		{"01:00:00:FB:A4:16", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.mac, func(t *testing.T) {
			got := b.IsExcluded(tt.mac)
			// The dash-separator case is expected to NOT match because
			// NormalizeMAC only strips colons; dashes are not handled.
			// We assert false for that one to document the behavior.
			if tt.mac == "01-00-00-9E-3E-75" {
				if got {
					t.Errorf("dash-separated MAC matched; NormalizeMAC does not strip dashes")
				}
				return
			}
			if got != tt.want {
				t.Errorf("IsExcluded(%q) = %v, want %v", tt.mac, got, tt.want)
			}
		})
	}
}

func TestResolveDevices_AppliesExcludes(t *testing.T) {
	// Even when a MAC is explicitly in Devices, the exclude list wins.
	b := BLEConfig{
		Devices: []DeviceConfig{
			{Name: "kitchen", MAC: "01:00:00:FB:A4:16"},
			{Name: "garage", MAC: "01:00:00:9E:3E:75"},
		},
		ExcludeMACs: []string{"01:00:00:9E:3E:75"},
	}
	got := b.ResolveDevices()
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].MAC != "01:00:00:FB:A4:16" {
		t.Errorf("remaining device MAC = %q, want kitchen's", got[0].MAC)
	}
}

func TestResolveDevices_ExcludesLegacyMAC(t *testing.T) {
	// Even the legacy single-device MAC can be excluded. A degenerate
	// config but we document that ExcludeMACs wins universally.
	b := BLEConfig{
		DeviceMAC:   "01:00:00:FB:A4:16",
		ExcludeMACs: []string{"01:00:00:FB:A4:16"},
	}
	if got := b.ResolveDevices(); len(got) != 0 {
		t.Errorf("expected empty, got %+v", got)
	}
}

// --- OTel config ---

func TestLoad_OTelDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OTel.Enabled {
		t.Error("OTel should default to disabled")
	}
	if cfg.OTel.ServiceName != "coolledux-controller" {
		t.Errorf("ServiceName = %q, want coolledux-controller", cfg.OTel.ServiceName)
	}
	if cfg.OTel.Protocol != "grpc" {
		t.Errorf("Protocol = %q, want grpc", cfg.OTel.Protocol)
	}
	if !cfg.OTel.Insecure {
		t.Error("Insecure should default to true")
	}
	if cfg.OTel.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s", cfg.OTel.Timeout)
	}
	if cfg.OTel.Endpoint != "" {
		t.Errorf("Endpoint should have no default, got %q", cfg.OTel.Endpoint)
	}
	if !cfg.OTel.Traces.Enabled || !cfg.OTel.Metrics.Enabled || !cfg.OTel.Logs.Enabled {
		t.Errorf("per-signal defaults should all be enabled, got traces=%v metrics=%v logs=%v",
			cfg.OTel.Traces.Enabled, cfg.OTel.Metrics.Enabled, cfg.OTel.Logs.Enabled)
	}
	if cfg.OTel.Metrics.Interval != 60*time.Second {
		t.Errorf("metrics interval = %v, want 60s", cfg.OTel.Metrics.Interval)
	}
	if !cfg.OTel.Metrics.Runtime {
		t.Error("metrics.runtime should default to true")
	}
}

func TestOTelConfig_Validate_DisabledNoop(t *testing.T) {
	// Disabled config never errors regardless of missing endpoint.
	c := OTelConfig{Enabled: false}
	if err := c.Validate(); err != nil {
		t.Errorf("disabled config should validate: %v", err)
	}
}

func TestOTelConfig_Validate_MissingEndpoint(t *testing.T) {
	// Clear any inherited OTEL_* endpoint env so we test the no-source case.
	for _, name := range otelEndpointEnvVars {
		t.Setenv(name, "")
	}
	c := OTelConfig{Enabled: true, Protocol: "grpc"}
	err := c.Validate()
	if err == nil {
		t.Fatal("expected error for enabled without endpoint")
	}
}

func TestOTelConfig_Validate_OTELEnvVarSatisfies(t *testing.T) {
	// Clear and then set just one of the SDK-native env vars; Validate
	// should accept it as a sufficient endpoint source even though the
	// YAML/COOLLEDUX paths leave it blank.
	for _, name := range otelEndpointEnvVars {
		t.Setenv(name, "")
	}
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector.example:4318")

	c := OTelConfig{Enabled: true, Protocol: "http"}
	if err := c.Validate(); err != nil {
		t.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT should satisfy validation: %v", err)
	}
}

func TestOTelConfig_Validate_PerSignalEnvVarSatisfies(t *testing.T) {
	for _, name := range otelEndpointEnvVars {
		t.Setenv(name, "")
	}
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "tempo.example:4317")

	c := OTelConfig{Enabled: true, Protocol: "grpc"}
	if err := c.Validate(); err != nil {
		t.Errorf("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT should satisfy validation: %v", err)
	}
}

func TestOTelConfig_Validate_PerSignalEndpointSatisfies(t *testing.T) {
	c := OTelConfig{
		Enabled: true,
		Traces:  OTelSignalConfig{Endpoint: "tempo:4317"},
	}
	if err := c.Validate(); err != nil {
		t.Errorf("should accept per-signal endpoint, got %v", err)
	}
}

func TestOTelConfig_Validate_BadProtocol(t *testing.T) {
	c := OTelConfig{Enabled: true, Endpoint: "h:4317", Protocol: "smoke-signals"}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for invalid protocol")
	}
}

func TestOTelConfig_ResolveEndpoint_PerSignalOverride(t *testing.T) {
	c := OTelConfig{Endpoint: "top:4317"}
	signal := OTelSignalConfig{Endpoint: "override:4317"}
	if got := c.ResolveEndpoint(signal); got != "override:4317" {
		t.Errorf("ResolveEndpoint = %q, want override", got)
	}
}

func TestOTelConfig_ResolveEndpoint_InheritsTop(t *testing.T) {
	c := OTelConfig{Endpoint: "top:4317"}
	if got := c.ResolveEndpoint(OTelSignalConfig{}); got != "top:4317" {
		t.Errorf("ResolveEndpoint = %q, want top", got)
	}
}
