package mqtt

import (
	"encoding/json"
	"testing"
)

func TestLightState_Marshal(t *testing.T) {
	tests := []struct {
		name   string
		state  LightState
		checks map[string]interface{}
	}{
		{
			name: "full state with color",
			state: LightState{
				State:      "ON",
				Brightness: 200,
				Color:      &RGBColor{R: 255, G: 128, B: 0},
				Effect:     "scroll_left",
			},
			checks: map[string]interface{}{
				"state":      "ON",
				"brightness": float64(200),
				"effect":     "scroll_left",
			},
		},
		{
			name: "off state, no color or effect (omitempty)",
			state: LightState{
				State:      "OFF",
				Brightness: 0,
			},
			checks: map[string]interface{}{
				"state":      "OFF",
				"brightness": float64(0),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.state)
			if err != nil {
				t.Fatalf("marshal error: %v", err)
			}

			var m map[string]interface{}
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}

			for key, expected := range tt.checks {
				got, ok := m[key]
				if !ok {
					t.Errorf("missing key %q in JSON output", key)
					continue
				}
				if got != expected {
					t.Errorf("key %q: expected %v, got %v", key, expected, got)
				}
			}

			// Verify omitempty behavior.
			if tt.state.Color == nil {
				if _, ok := m["color"]; ok {
					t.Error("color should be omitted when nil")
				}
			} else {
				colorMap, ok := m["color"].(map[string]interface{})
				if !ok {
					t.Fatal("color should be an object")
				}
				if colorMap["r"] != float64(tt.state.Color.R) {
					t.Errorf("color.r: expected %d, got %v", tt.state.Color.R, colorMap["r"])
				}
			}

			if tt.state.Effect == "" {
				if _, ok := m["effect"]; ok {
					t.Error("effect should be omitted when empty")
				}
			}
		})
	}
}

func TestLightState_Unmarshal(t *testing.T) {
	input := `{"state":"ON","brightness":150,"color":{"r":10,"g":20,"b":30},"effect":"wipe_down"}`
	var s LightState
	if err := json.Unmarshal([]byte(input), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.State != "ON" {
		t.Errorf("state: expected ON, got %s", s.State)
	}
	if s.Brightness != 150 {
		t.Errorf("brightness: expected 150, got %d", s.Brightness)
	}
	if s.Color == nil {
		t.Fatal("color should not be nil")
	}
	if s.Color.R != 10 || s.Color.G != 20 || s.Color.B != 30 {
		t.Errorf("color: expected (10,20,30), got (%d,%d,%d)", s.Color.R, s.Color.G, s.Color.B)
	}
	if s.Effect != "wipe_down" {
		t.Errorf("effect: expected wipe_down, got %s", s.Effect)
	}
}

func TestLightCommand_OmitEmpty(t *testing.T) {
	tests := []struct {
		name        string
		cmd         LightCommand
		expectKeys  []string
		missingKeys []string
	}{
		{
			name:        "empty command",
			cmd:         LightCommand{},
			expectKeys:  nil,
			missingKeys: []string{"state", "brightness", "color", "effect"},
		},
		{
			name:        "state only",
			cmd:         LightCommand{State: "ON"},
			expectKeys:  []string{"state"},
			missingKeys: []string{"brightness", "color", "effect"},
		},
		{
			name: "brightness only",
			cmd: LightCommand{
				Brightness: ptrUint8(128),
			},
			expectKeys:  []string{"brightness"},
			missingKeys: []string{"state", "color", "effect"},
		},
		{
			name: "all fields",
			cmd: LightCommand{
				State:      "ON",
				Brightness: ptrUint8(200),
				Color:      &RGBColor{R: 1, G: 2, B: 3},
				Effect:     "wave",
			},
			expectKeys:  []string{"state", "brightness", "color", "effect"},
			missingKeys: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.cmd)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var m map[string]interface{}
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			for _, key := range tt.expectKeys {
				if _, ok := m[key]; !ok {
					t.Errorf("expected key %q to be present", key)
				}
			}
			for _, key := range tt.missingKeys {
				if _, ok := m[key]; ok {
					t.Errorf("expected key %q to be omitted", key)
				}
			}
		})
	}
}

func TestLightCommand_Unmarshal(t *testing.T) {
	input := `{"state":"OFF","brightness":50}`
	var cmd LightCommand
	if err := json.Unmarshal([]byte(input), &cmd); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cmd.State != "OFF" {
		t.Errorf("state: expected OFF, got %s", cmd.State)
	}
	if cmd.Brightness == nil || *cmd.Brightness != 50 {
		t.Errorf("brightness: expected 50, got %v", cmd.Brightness)
	}
	if cmd.Color != nil {
		t.Error("color should be nil")
	}
	if cmd.Effect != "" {
		t.Errorf("effect should be empty, got %q", cmd.Effect)
	}
}

func TestTextCommand_RoundTrip(t *testing.T) {
	original := TextCommand{
		Text:     "Hello World",
		Mode:     "scroll_left",
		Speed:    5,
		Color:    "#FF0000",
		FontSize: 16,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded TextCommand
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded != original {
		t.Errorf("round-trip mismatch:\n  original: %+v\n  decoded:  %+v", original, decoded)
	}
}

func TestImageCommand_RoundTrip(t *testing.T) {
	original := ImageCommand{
		ImageBase64: "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJ",
		Mode:        "static",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ImageCommand
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded != original {
		t.Errorf("round-trip mismatch:\n  original: %+v\n  decoded:  %+v", original, decoded)
	}
}

func TestGIFCommand_RoundTrip(t *testing.T) {
	original := GIFCommand{
		GIFBase64:     "R0lGODlhAQABAIAAAP///wAAACH5BAEAAAAALAAAAAABAAEAAAICRAEAOw==",
		FrameDuration: 100,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded GIFCommand
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded != original {
		t.Errorf("round-trip mismatch:\n  original: %+v\n  decoded:  %+v", original, decoded)
	}
}

func TestRGBColor_Marshal(t *testing.T) {
	c := RGBColor{R: 255, G: 0, B: 128}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if m["r"] != float64(255) {
		t.Errorf("r: expected 255, got %v", m["r"])
	}
	if m["g"] != float64(0) {
		t.Errorf("g: expected 0, got %v", m["g"])
	}
	if m["b"] != float64(128) {
		t.Errorf("b: expected 128, got %v", m["b"])
	}
}

func TestHADevice_Marshal(t *testing.T) {
	tests := []struct {
		name      string
		device    HADevice
		wantName  bool
		wantMfr   bool
		wantModel bool
	}{
		{
			name: "full device",
			device: HADevice{
				Identifiers:  []string{"id1"},
				Name:         "Test",
				Manufacturer: "Mfr",
				Model:        "Mdl",
				SWVersion:    "1.0",
			},
			wantName: true, wantMfr: true, wantModel: true,
		},
		{
			name: "ref device (identifiers only)",
			device: HADevice{
				Identifiers: []string{"id1"},
			},
			wantName: false, wantMfr: false, wantModel: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.device)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var m map[string]interface{}
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if _, ok := m["identifiers"]; !ok {
				t.Error("identifiers should always be present")
			}

			_, hasName := m["name"]
			if hasName != tt.wantName {
				t.Errorf("name presence: expected %v, got %v", tt.wantName, hasName)
			}
			_, hasMfr := m["manufacturer"]
			if hasMfr != tt.wantMfr {
				t.Errorf("manufacturer presence: expected %v, got %v", tt.wantMfr, hasMfr)
			}
			_, hasModel := m["model"]
			if hasModel != tt.wantModel {
				t.Errorf("model presence: expected %v, got %v", tt.wantModel, hasModel)
			}
		})
	}
}

func ptrUint8(v uint8) *uint8 {
	return &v
}
