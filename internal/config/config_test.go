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
