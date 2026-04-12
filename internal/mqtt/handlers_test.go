package mqtt

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/liskl/coolledux-controller/internal/ble"
	"github.com/liskl/coolledux-controller/internal/config"
	"github.com/liskl/coolledux-controller/internal/controller"
)

// testHandler returns a CommandHandler backed by a real (but disconnected)
// controller. Commands that hit the BLE transport will fail with "not connected"
// errors instead of nil-pointer panics.
func testHandler() *CommandHandler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bleClient := ble.NewClient(logger)
	transport := ble.NewTransport(bleClient, logger)
	cfg := &config.Config{
		BLE:     config.BLEConfig{DeviceMAC: "01:00:00:FB:A4:16"},
		Display: config.DisplayConfig{Columns: 96, Rows: 16},
	}
	ctrl := controller.New(bleClient, transport, cfg, logger)
	return NewCommandHandler(ctrl, logger)
}

func TestNewCommandHandler(t *testing.T) {
	h := NewCommandHandler(nil, slog.Default())
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.state == nil {
		t.Fatal("expected non-nil initial state")
	}
	if h.state.State != "ON" {
		t.Errorf("initial state: expected ON, got %s", h.state.State)
	}
	if h.state.Brightness != 128 {
		t.Errorf("initial brightness: expected 128, got %d", h.state.Brightness)
	}
	if h.state.Color == nil || h.state.Color.R != 255 || h.state.Color.G != 255 || h.state.Color.B != 255 {
		t.Error("initial color should be white (255,255,255)")
	}
}

func TestGetCurrentState(t *testing.T) {
	h := NewCommandHandler(nil, slog.Default())
	data := h.GetCurrentState()

	var state LightState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if state.State != "ON" {
		t.Errorf("state: expected ON, got %s", state.State)
	}
	if state.Brightness != 128 {
		t.Errorf("brightness: expected 128, got %d", state.Brightness)
	}
	if state.Color == nil {
		t.Fatal("color should not be nil")
	}
	if state.Color.R != 255 || state.Color.G != 255 || state.Color.B != 255 {
		t.Errorf("color: expected (255,255,255), got (%d,%d,%d)", state.Color.R, state.Color.G, state.Color.B)
	}
}

func TestGetCurrentState_ValidJSON(t *testing.T) {
	h := NewCommandHandler(nil, slog.Default())
	data := h.GetCurrentState()

	// Should be valid JSON with expected keys.
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}

	requiredKeys := []string{"state", "brightness", "color"}
	for _, key := range requiredKeys {
		if _, ok := m[key]; !ok {
			t.Errorf("missing key %q in state JSON", key)
		}
	}
}

func TestHandleLightCommand_InvalidJSON(t *testing.T) {
	h := NewCommandHandler(nil, slog.Default())

	tests := []struct {
		name    string
		payload string
	}{
		{"empty", ""},
		{"not json", "hello world"},
		{"malformed", `{"state": ON}`},
		{"truncated", `{"state": "ON`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.HandleLightCommand([]byte(tt.payload))
			if err == nil {
				t.Error("expected error for invalid JSON")
			}
			if !strings.Contains(err.Error(), "parsing light command") {
				t.Errorf("error should mention parsing, got: %v", err)
			}
		})
	}
}

func TestHandleLightCommand_StateON_Disconnected(t *testing.T) {
	h := testHandler()

	payload := `{"state": "ON"}`
	err := h.HandleLightCommand([]byte(payload))
	if err == nil {
		t.Fatal("expected error from disconnected controller")
	}
	if !strings.Contains(err.Error(), "setting power") {
		t.Errorf("expected error containing 'setting power', got: %v", err)
	}
}

func TestHandleLightCommand_Brightness_Disconnected(t *testing.T) {
	h := testHandler()

	payload := `{"brightness": 200}`
	err := h.HandleLightCommand([]byte(payload))
	if err == nil {
		t.Fatal("expected error from disconnected controller")
	}
	if !strings.Contains(err.Error(), "setting brightness") {
		t.Errorf("expected error containing 'setting brightness', got: %v", err)
	}
}

func TestHandleLightCommand_StateOFF_Disconnected(t *testing.T) {
	h := testHandler()

	payload := `{"state": "OFF"}`
	err := h.HandleLightCommand([]byte(payload))
	if err == nil {
		t.Fatal("expected error from disconnected controller")
	}
	if !strings.Contains(err.Error(), "setting power") {
		t.Errorf("expected error containing 'setting power', got: %v", err)
	}
}

func TestHandleLightCommand_StateAndColor_Disconnected(t *testing.T) {
	h := testHandler()

	payload := `{"state": "ON", "color": {"r":255,"g":0,"b":0}}`
	err := h.HandleLightCommand([]byte(payload))
	if err == nil {
		t.Fatal("expected error from disconnected controller")
	}
	if !strings.Contains(err.Error(), "setting power") {
		t.Errorf("expected error containing 'setting power', got: %v", err)
	}

	// Color should NOT have been updated because state processing failed first.
	data := h.GetCurrentState()
	var s LightState
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Initial color is white (255,255,255); it should still be white.
	if s.Color != nil && s.Color.R == 255 && s.Color.G == 0 && s.Color.B == 0 {
		t.Error("color was updated to red despite power command failing first")
	}
}

func TestHandleLightCommand_ColorAndEffect(t *testing.T) {
	h := testHandler()

	payload := `{"color":{"r":100,"g":50,"b":25},"effect":"blink"}`
	err := h.HandleLightCommand([]byte(payload))
	// No controller calls needed (color and effect are state-only), so no error.
	if err != nil {
		t.Errorf("unexpected error for color+effect command: %v", err)
	}

	data := h.GetCurrentState()
	var s LightState
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.Color == nil || s.Color.R != 100 || s.Color.G != 50 || s.Color.B != 25 {
		t.Errorf("color not updated: got %+v", s.Color)
	}
	if s.Effect != "blink" {
		t.Errorf("effect not updated: expected 'blink', got %q", s.Effect)
	}
}

func TestHandleLightCommand_ColorOnly(t *testing.T) {
	// A command with only color should not hit the controller (no state/brightness change).
	h := NewCommandHandler(nil, slog.Default())

	payload := `{"color": {"r": 100, "g": 50, "b": 25}}`
	err := h.HandleLightCommand([]byte(payload))
	// No controller calls needed for color-only, so no error.
	if err != nil {
		t.Errorf("unexpected error for color-only command: %v", err)
	}

	// Verify state was updated.
	state := h.GetCurrentState()
	var s LightState
	if err := json.Unmarshal(state, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.Color == nil || s.Color.R != 100 || s.Color.G != 50 || s.Color.B != 25 {
		t.Errorf("color not updated: got %+v", s.Color)
	}
}

func TestHandleLightCommand_EffectOnly(t *testing.T) {
	h := NewCommandHandler(nil, slog.Default())

	payload := `{"effect": "wave"}`
	err := h.HandleLightCommand([]byte(payload))
	if err != nil {
		t.Errorf("unexpected error for effect-only command: %v", err)
	}

	state := h.GetCurrentState()
	var s LightState
	if err := json.Unmarshal(state, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.Effect != "wave" {
		t.Errorf("effect not updated: expected 'wave', got %q", s.Effect)
	}
}

func TestHandleTextCommand_InvalidJSON(t *testing.T) {
	h := NewCommandHandler(nil, slog.Default())

	tests := []struct {
		name    string
		payload string
		errHint string
	}{
		{"not json", "nope", "parsing text command"},
		{"empty", "", "parsing text command"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.HandleTextCommand([]byte(tt.payload))
			if err == nil {
				t.Error("expected error")
			}
			if !strings.Contains(err.Error(), tt.errHint) {
				t.Errorf("expected error containing %q, got: %v", tt.errHint, err)
			}
		})
	}
}

func TestHandleTextCommand_InvalidMode(t *testing.T) {
	h := NewCommandHandler(nil, slog.Default())

	payload := `{"text":"hi","mode":"invalid_mode","speed":5,"color":"#FF0000","font_size":16}`
	err := h.HandleTextCommand([]byte(payload))
	if err == nil {
		t.Error("expected error for invalid mode")
	}
	if !strings.Contains(err.Error(), "parsing text mode") {
		t.Errorf("expected mode error, got: %v", err)
	}
}

func TestHandleTextCommand_InvalidColor(t *testing.T) {
	h := NewCommandHandler(nil, slog.Default())

	payload := `{"text":"hi","mode":"static","speed":5,"color":"notacolor","font_size":16}`
	err := h.HandleTextCommand([]byte(payload))
	if err == nil {
		t.Error("expected error for invalid color")
	}
	if !strings.Contains(err.Error(), "parsing color") {
		t.Errorf("expected color error, got: %v", err)
	}
}

func TestHandleTextCommand_ValidJSON_Disconnected(t *testing.T) {
	h := testHandler()

	payload := `{"text":"hello","mode":"static","speed":5,"color":"#FF0000","font_size":16}`
	err := h.HandleTextCommand([]byte(payload))
	if err == nil {
		t.Fatal("expected error from disconnected controller")
	}
	if !strings.Contains(err.Error(), "displaying text") {
		t.Errorf("expected error containing 'displaying text', got: %v", err)
	}
}

func TestHandleImageCommand_InvalidJSON(t *testing.T) {
	h := NewCommandHandler(nil, slog.Default())

	err := h.HandleImageCommand([]byte("not json"))
	if err == nil {
		t.Error("expected error")
	}
	if !strings.Contains(err.Error(), "parsing image command") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestHandleImageCommand_InvalidBase64(t *testing.T) {
	h := NewCommandHandler(nil, slog.Default())

	payload := `{"image_base64":"not-valid-base64!!!","mode":"static"}`
	err := h.HandleImageCommand([]byte(payload))
	if err == nil {
		t.Error("expected error for invalid base64")
	}
	if !strings.Contains(err.Error(), "decoding image base64") {
		t.Errorf("expected base64 error, got: %v", err)
	}
}

func TestHandleImageCommand_InvalidMode(t *testing.T) {
	h := NewCommandHandler(nil, slog.Default())

	imgB64 := base64.StdEncoding.EncodeToString([]byte("fake image data"))
	payload := `{"image_base64":"` + imgB64 + `","mode":"bad_mode"}`
	err := h.HandleImageCommand([]byte(payload))
	if err == nil {
		t.Error("expected error for invalid mode")
	}
	if !strings.Contains(err.Error(), "parsing image mode") {
		t.Errorf("expected mode error, got: %v", err)
	}
}

func TestHandleImageCommand_ValidBase64_NilController(t *testing.T) {
	h := NewCommandHandler(nil, slog.Default())

	imgB64 := base64.StdEncoding.EncodeToString([]byte("fake image data"))
	payload := `{"image_base64":"` + imgB64 + `","mode":"static"}`

	err := h.HandleImageCommand([]byte(payload))
	if err == nil {
		t.Error("expected error")
	}
	// Should not be a JSON parsing or base64 error. The fake image data
	// will fail to decode as an actual image inside the controller.
	if strings.Contains(err.Error(), "parsing image command") || strings.Contains(err.Error(), "decoding image base64") {
		t.Errorf("error should be past parse/base64 stages, got: %v", err)
	}
}

func TestHandleGIFCommand_InvalidJSON(t *testing.T) {
	h := NewCommandHandler(nil, slog.Default())

	err := h.HandleGIFCommand([]byte("}}}}"))
	if err == nil {
		t.Error("expected error")
	}
	if !strings.Contains(err.Error(), "parsing gif command") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestHandleGIFCommand_InvalidBase64(t *testing.T) {
	h := NewCommandHandler(nil, slog.Default())

	payload := `{"gif_base64":"!!!bad!!!","frame_duration":100}`
	err := h.HandleGIFCommand([]byte(payload))
	if err == nil {
		t.Error("expected error for invalid base64")
	}
	if !strings.Contains(err.Error(), "decoding gif base64") {
		t.Errorf("expected base64 error, got: %v", err)
	}
}

func TestHandleGIFCommand_ValidBase64_NilController(t *testing.T) {
	h := NewCommandHandler(nil, slog.Default())

	gifB64 := base64.StdEncoding.EncodeToString([]byte("fake gif data"))
	payload := `{"gif_base64":"` + gifB64 + `","frame_duration":50}`

	err := h.HandleGIFCommand([]byte(payload))
	if err == nil {
		t.Error("expected error")
	}
	// Should not be a JSON parsing or base64 error. The fake GIF data
	// will fail to decode as a valid GIF inside the controller.
	if strings.Contains(err.Error(), "parsing gif command") || strings.Contains(err.Error(), "decoding gif base64") {
		t.Errorf("error should be past parse/base64 stages, got: %v", err)
	}
}

func TestParseHexColor(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint32
		wantErr bool
	}{
		{"red with hash", "#FF0000", 0xFF0000, false},
		{"green without hash", "00FF00", 0x00FF00, false},
		{"blue with hash", "#0000FF", 0x0000FF, false},
		{"black", "#000000", 0x000000, false},
		{"white", "#FFFFFF", 0xFFFFFF, false},
		{"lowercase", "#ff8800", 0xFF8800, false},
		{"too short", "#FFF", 0, true},
		{"too long", "#FFFFFFF", 0, true},
		{"empty", "", 0, true},
		{"invalid hex", "#GGHHII", 0, true},
		{"no digits", "notcolor", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHexColor(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("expected 0x%06X, got 0x%06X", tt.want, got)
			}
		})
	}
}
