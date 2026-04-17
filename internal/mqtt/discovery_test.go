package mqtt

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildLightConfig(t *testing.T) {
	tests := []struct {
		name        string
		deviceID    string
		topicPrefix string
		haPrefix    string
	}{
		{"standard", "abcdef123456", "coolledux", "homeassistant"},
		{"custom prefix", "010000fba416", "myprefix", "ha_custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topic, payload := BuildLightConfig(tt.deviceID, tt.topicPrefix, tt.haPrefix)

			// Verify topic format.
			expectedTopic := tt.haPrefix + "/light/coolledux_" + tt.deviceID + "/config"
			if topic != expectedTopic {
				t.Errorf("topic: expected %q, got %q", expectedTopic, topic)
			}

			// Unmarshal and verify fields.
			var cfg LightConfig
			if err := json.Unmarshal(payload, &cfg); err != nil {
				t.Fatalf("unmarshaling payload: %v", err)
			}

			if cfg.Name != "CoolLEDUX Sign" {
				t.Errorf("name: expected %q, got %q", "CoolLEDUX Sign", cfg.Name)
			}
			if cfg.UniqueID != "coolledux_"+tt.deviceID+"_light" {
				t.Errorf("unique_id: expected %q, got %q", "coolledux_"+tt.deviceID+"_light", cfg.UniqueID)
			}
			if cfg.Schema != "json" {
				t.Errorf("schema: expected %q, got %q", "json", cfg.Schema)
			}
			if !cfg.Brightness {
				t.Error("brightness should be true")
			}
			if cfg.BrightnessScale != 255 {
				t.Errorf("brightness_scale: expected 255, got %d", cfg.BrightnessScale)
			}
			if !cfg.ColorMode {
				t.Error("color_mode should be true")
			}
			if len(cfg.SupportedColorModes) != 1 || cfg.SupportedColorModes[0] != "rgb" {
				t.Errorf("supported_color_modes: expected [rgb], got %v", cfg.SupportedColorModes)
			}
			if !cfg.Effect {
				t.Error("effect should be true")
			}
			if len(cfg.EffectList) != 13 {
				t.Errorf("effect_list: expected 13 entries, got %d", len(cfg.EffectList))
			}
			if cfg.PayloadAvailable != "online" {
				t.Errorf("payload_available: expected %q, got %q", "online", cfg.PayloadAvailable)
			}
			if cfg.PayloadNotAvailable != "offline" {
				t.Errorf("payload_not_available: expected %q, got %q", "offline", cfg.PayloadNotAvailable)
			}

			// Verify command/state topics.
			expectedCmdTopic := tt.topicPrefix + "/" + tt.deviceID + "/set"
			if cfg.CommandTopic != expectedCmdTopic {
				t.Errorf("command_topic: expected %q, got %q", expectedCmdTopic, cfg.CommandTopic)
			}
			expectedStateTopic := tt.topicPrefix + "/" + tt.deviceID + "/state"
			if cfg.StateTopic != expectedStateTopic {
				t.Errorf("state_topic: expected %q, got %q", expectedStateTopic, cfg.StateTopic)
			}

			// Verify device info.
			if cfg.Device.Manufacturer != "E-CrossStu / CoolLEDUX" {
				t.Errorf("manufacturer: expected %q, got %q", "E-CrossStu / CoolLEDUX", cfg.Device.Manufacturer)
			}
			if cfg.Device.Model != "JT_HW358.02 16x96" {
				t.Errorf("model: expected %q, got %q", "JT_HW358.02 16x96", cfg.Device.Model)
			}
			if cfg.Device.Name != "CoolLEDUX Sign" {
				t.Errorf("device name: expected %q, got %q", "CoolLEDUX Sign", cfg.Device.Name)
			}
			if len(cfg.Device.Identifiers) != 1 || cfg.Device.Identifiers[0] != "coolledux_"+tt.deviceID {
				t.Errorf("device identifiers: expected [coolledux_%s], got %v", tt.deviceID, cfg.Device.Identifiers)
			}
		})
	}
}

func TestBuildConnectionSensorConfig(t *testing.T) {
	tests := []struct {
		name        string
		deviceID    string
		topicPrefix string
		haPrefix    string
	}{
		{"standard", "abcdef123456", "coolledux", "homeassistant"},
		{"custom", "010000fba416", "custom", "ha"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topic, payload := BuildConnectionSensorConfig(tt.deviceID, tt.topicPrefix, tt.haPrefix)

			expectedTopic := tt.haPrefix + "/binary_sensor/coolledux_" + tt.deviceID + "_connection/config"
			if topic != expectedTopic {
				t.Errorf("topic: expected %q, got %q", expectedTopic, topic)
			}

			var cfg BinarySensorConfig
			if err := json.Unmarshal(payload, &cfg); err != nil {
				t.Fatalf("unmarshaling: %v", err)
			}

			if cfg.PayloadOn != "online" {
				t.Errorf("payload_on: expected %q, got %q", "online", cfg.PayloadOn)
			}
			if cfg.PayloadOff != "offline" {
				t.Errorf("payload_off: expected %q, got %q", "offline", cfg.PayloadOff)
			}
			if cfg.DeviceClass != "connectivity" {
				t.Errorf("device_class: expected %q, got %q", "connectivity", cfg.DeviceClass)
			}
			if cfg.Name != "CoolLEDUX Connection" {
				t.Errorf("name: expected %q, got %q", "CoolLEDUX Connection", cfg.Name)
			}
			if cfg.UniqueID != "coolledux_"+tt.deviceID+"_connection" {
				t.Errorf("unique_id: expected %q, got %q", "coolledux_"+tt.deviceID+"_connection", cfg.UniqueID)
			}

			// Verify ref device (only identifiers, no name/manufacturer).
			if len(cfg.Device.Identifiers) != 1 {
				t.Errorf("expected 1 identifier, got %d", len(cfg.Device.Identifiers))
			}
			expectedAvailTopic := tt.topicPrefix + "/" + tt.deviceID + "/availability"
			if cfg.StateTopic != expectedAvailTopic {
				t.Errorf("state_topic: expected %q, got %q", expectedAvailTopic, cfg.StateTopic)
			}
		})
	}
}

func TestBuildBrightnessSensorConfig(t *testing.T) {
	tests := []struct {
		name        string
		deviceID    string
		topicPrefix string
		haPrefix    string
	}{
		{"standard", "abcdef123456", "coolledux", "homeassistant"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topic, payload := BuildBrightnessSensorConfig(tt.deviceID, tt.topicPrefix, tt.haPrefix)

			expectedTopic := tt.haPrefix + "/sensor/coolledux_" + tt.deviceID + "_brightness/config"
			if topic != expectedTopic {
				t.Errorf("topic: expected %q, got %q", expectedTopic, topic)
			}

			var cfg SensorConfig
			if err := json.Unmarshal(payload, &cfg); err != nil {
				t.Fatalf("unmarshaling: %v", err)
			}

			if cfg.Name != "CoolLEDUX Brightness" {
				t.Errorf("name: expected %q, got %q", "CoolLEDUX Brightness", cfg.Name)
			}
			if cfg.UniqueID != "coolledux_"+tt.deviceID+"_brightness" {
				t.Errorf("unique_id: expected %q, got %q", "coolledux_"+tt.deviceID+"_brightness", cfg.UniqueID)
			}
			if !strings.Contains(cfg.ValueTemplate, "brightness") {
				t.Errorf("value_template should contain 'brightness', got %q", cfg.ValueTemplate)
			}

			expectedStateTopic := tt.topicPrefix + "/" + tt.deviceID + "/state"
			if cfg.StateTopic != expectedStateTopic {
				t.Errorf("state_topic: expected %q, got %q", expectedStateTopic, cfg.StateTopic)
			}
		})
	}
}

func TestBuildColorModeSelectConfig(t *testing.T) {
	topic, payload := BuildColorModeSelectConfig("abcdef123456", "coolledux", "homeassistant")

	wantTopic := "homeassistant/select/coolledux_abcdef123456_color_mode/config"
	if topic != wantTopic {
		t.Errorf("topic: got %q, want %q", topic, wantTopic)
	}

	var cfg SelectConfig
	if err := json.Unmarshal(payload, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if cfg.Name != "CoolLEDUX Color Mode" {
		t.Errorf("name: got %q, want %q", cfg.Name, "CoolLEDUX Color Mode")
	}
	if cfg.UniqueID != "coolledux_abcdef123456_color_mode" {
		t.Errorf("unique_id: got %q", cfg.UniqueID)
	}
	if cfg.CommandTopic != "coolledux/abcdef123456/color/mode/set" {
		t.Errorf("command_topic: got %q", cfg.CommandTopic)
	}
	if cfg.StateTopic != "coolledux/abcdef123456/color/mode/state" {
		t.Errorf("state_topic: got %q", cfg.StateTopic)
	}
	// Options should start with "off" and include all valid mode IDs.
	if len(cfg.Options) < 2 {
		t.Fatalf("options: got %d entries, want >=2", len(cfg.Options))
	}
	if cfg.Options[0] != "off" {
		t.Errorf("options[0]: got %q, want %q", cfg.Options[0], "off")
	}
	if cfg.Icon != "mdi:palette" {
		t.Errorf("icon: got %q", cfg.Icon)
	}
	// refDevice: only identifiers populated.
	if len(cfg.Device.Identifiers) != 1 || cfg.Device.Identifiers[0] != "coolledux_abcdef123456" {
		t.Errorf("device identifiers: got %v", cfg.Device.Identifiers)
	}
	if cfg.Device.Name != "" || cfg.Device.Manufacturer != "" {
		t.Error("ref device should not have Name/Manufacturer set")
	}
}

func TestBuildColorSpeedNumberConfig(t *testing.T) {
	topic, payload := BuildColorSpeedNumberConfig("abcdef123456", "coolledux", "homeassistant")

	wantTopic := "homeassistant/number/coolledux_abcdef123456_color_speed/config"
	if topic != wantTopic {
		t.Errorf("topic: got %q, want %q", topic, wantTopic)
	}

	var cfg NumberConfig
	if err := json.Unmarshal(payload, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if cfg.Name != "CoolLEDUX Color Speed" {
		t.Errorf("name: got %q", cfg.Name)
	}
	if cfg.Min != 1 {
		t.Errorf("min: got %v, want 1", cfg.Min)
	}
	if cfg.Max != 10 {
		t.Errorf("max: got %v, want 10", cfg.Max)
	}
	if cfg.Step != 1 {
		t.Errorf("step: got %v, want 1", cfg.Step)
	}
	if cfg.Mode != "slider" {
		t.Errorf("mode: got %q, want slider", cfg.Mode)
	}
	if cfg.CommandTopic != "coolledux/abcdef123456/color/speed/set" {
		t.Errorf("command_topic: got %q", cfg.CommandTopic)
	}
}

func TestBuildShowDeviceIDSwitchConfig(t *testing.T) {
	topic, payload := BuildShowDeviceIDSwitchConfig("abcdef123456", "coolledux", "homeassistant")

	wantTopic := "homeassistant/switch/coolledux_abcdef123456_show_id/config"
	if topic != wantTopic {
		t.Errorf("topic: got %q, want %q", topic, wantTopic)
	}

	var cfg SwitchConfig
	if err := json.Unmarshal(payload, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Name != "CoolLEDUX Show Device ID" {
		t.Errorf("name: got %q", cfg.Name)
	}
	if cfg.UniqueID != "coolledux_abcdef123456_show_id" {
		t.Errorf("unique_id: got %q", cfg.UniqueID)
	}
	if cfg.CommandTopic != "coolledux/abcdef123456/show_id/set" {
		t.Errorf("command_topic: got %q", cfg.CommandTopic)
	}
	if cfg.PayloadOn != "ON" || cfg.PayloadOff != "OFF" {
		t.Errorf("payloads: got on=%q off=%q", cfg.PayloadOn, cfg.PayloadOff)
	}
	if cfg.Icon != "mdi:identifier" {
		t.Errorf("icon: got %q", cfg.Icon)
	}
}

func TestBuildRemoteEnableSwitchConfig(t *testing.T) {
	topic, payload := BuildRemoteEnableSwitchConfig("abcdef123456", "coolledux", "homeassistant")

	wantTopic := "homeassistant/switch/coolledux_abcdef123456_remote/config"
	if topic != wantTopic {
		t.Errorf("topic: got %q, want %q", topic, wantTopic)
	}

	var cfg SwitchConfig
	if err := json.Unmarshal(payload, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Name != "CoolLEDUX Remote" {
		t.Errorf("name: got %q", cfg.Name)
	}
	if cfg.CommandTopic != "coolledux/abcdef123456/remote/set" {
		t.Errorf("command_topic: got %q", cfg.CommandTopic)
	}
	if cfg.Icon != "mdi:remote" {
		t.Errorf("icon: got %q", cfg.Icon)
	}
}

func TestEffectListContents(t *testing.T) {
	expectedEffects := []string{
		"static", "scroll_left", "scroll_right", "scroll_up", "scroll_down",
		"wipe_down", "expand_from_center", "blink", "zoom_in", "zoom_out", "wipe_left", "wipe_right", "collapse_to_center",
	}

	_, payload := BuildLightConfig("test", "test", "ha")
	var cfg LightConfig
	if err := json.Unmarshal(payload, &cfg); err != nil {
		t.Fatalf("unmarshaling: %v", err)
	}

	if len(cfg.EffectList) != len(expectedEffects) {
		t.Fatalf("expected %d effects, got %d", len(expectedEffects), len(cfg.EffectList))
	}

	for i, expected := range expectedEffects {
		if cfg.EffectList[i] != expected {
			t.Errorf("effect[%d]: expected %q, got %q", i, expected, cfg.EffectList[i])
		}
	}
}
