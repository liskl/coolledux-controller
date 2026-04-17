package mqtt

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/liskl/coolledux-controller/internal/ble"
	"github.com/liskl/coolledux-controller/internal/config"
	"github.com/liskl/coolledux-controller/internal/controller"
	"github.com/liskl/coolledux-controller/internal/protocol"
)

// connectedHandler returns a CommandHandler wired to a controller whose BLE
// client is "connected" via the test overrides. Writes are accepted silently;
// callers are responsible for injecting the expected response frames.
func connectedHandler() (*CommandHandler, *ble.Transport, *ble.Client) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bleClient := ble.NewClient(logger)
	transport := ble.NewTransport(bleClient, logger)
	cfg := &config.Config{
		BLE:     config.BLEConfig{DeviceMAC: "01:00:00:FB:A4:16"},
		Display: config.DisplayConfig{Columns: 96, Rows: 16},
	}
	ctrl := controller.New(bleClient, transport, cfg, logger)

	bleClient.OverrideConnectedForTest(true)
	bleClient.OverrideSendFuncForTest(func(_ context.Context, _ []byte) error {
		return nil
	})

	return NewCommandHandler(ctrl, logger), transport, bleClient
}

func fakeMQTTResponse(respType byte) []byte {
	return protocol.BuildStreamFrame([]byte{respType, protocol.STATUS_SUCCESS})
}

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

	payload := `{"color":{"r":100,"g":50,"b":25},"effect":"wipe_down"}`
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
	if s.Effect != "wipe_down" {
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

func TestHandleImageCommand_InvalidFit(t *testing.T) {
	h := NewCommandHandler(nil, slog.Default())

	imgB64 := base64.StdEncoding.EncodeToString([]byte("x"))
	payload := `{"image_base64":"` + imgB64 + `","mode":"static","fit":"bogus"}`
	err := h.HandleImageCommand([]byte(payload))
	if err == nil {
		t.Fatal("expected error for invalid fit")
	}
	if !strings.Contains(err.Error(), "parsing image fit") {
		t.Errorf("expected fit error, got: %v", err)
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

func TestHandleGIFCommand_InvalidFit(t *testing.T) {
	h := NewCommandHandler(nil, slog.Default())

	gifB64 := base64.StdEncoding.EncodeToString([]byte("x"))
	payload := `{"gif_base64":"` + gifB64 + `","frame_duration":50,"fit":"bogus"}`
	err := h.HandleGIFCommand([]byte(payload))
	if err == nil {
		t.Fatal("expected error for invalid fit")
	}
	if !strings.Contains(err.Error(), "parsing gif fit") {
		t.Errorf("expected fit error, got: %v", err)
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

func TestHandleLightCommand_PowerON_Connected(t *testing.T) {
	h, transport, client := connectedHandler()
	defer client.OverrideConnectedForTest(false)

	go func() { transport.InjectResponseForTest(fakeMQTTResponse(protocol.RESPONSE_TYPE_POWER)) }()

	if err := h.HandleLightCommand([]byte(`{"state":"ON"}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var s LightState
	if err := json.Unmarshal(h.GetCurrentState(), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.State != "ON" {
		t.Errorf("state should be ON after power-on, got %q", s.State)
	}
}

func TestHandleLightCommand_PowerOFF_Connected(t *testing.T) {
	h, transport, client := connectedHandler()
	defer client.OverrideConnectedForTest(false)

	go func() { transport.InjectResponseForTest(fakeMQTTResponse(protocol.RESPONSE_TYPE_POWER)) }()

	if err := h.HandleLightCommand([]byte(`{"state":"off"}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var s LightState
	if err := json.Unmarshal(h.GetCurrentState(), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.State != "OFF" {
		t.Errorf("state should be OFF after power-off, got %q", s.State)
	}
}

func TestHandleLightCommand_Brightness_Connected(t *testing.T) {
	h, transport, client := connectedHandler()
	defer client.OverrideConnectedForTest(false)

	go func() { transport.InjectResponseForTest(fakeMQTTResponse(protocol.RESPONSE_TYPE_BRIGHTNESS)) }()

	payload := []byte(`{"brightness":42}`)
	if err := h.HandleLightCommand(payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var s LightState
	if err := json.Unmarshal(h.GetCurrentState(), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.Brightness != 42 {
		t.Errorf("brightness should be 42, got %d", s.Brightness)
	}
}

func TestHandleTextCommand_Success_Connected(t *testing.T) {
	h, transport, client := connectedHandler()
	defer client.OverrideConnectedForTest(false)

	// DisplayText uploads a program: RESPONSE_TYPE_PROGRAM_START then one or
	// more RESPONSE_TYPE_PROGRAM_DATA. Inject extras; spare responses are
	// harmless (channel buffer holds up to 16).
	go func() {
		transport.InjectResponseForTest(fakeMQTTResponse(protocol.RESPONSE_TYPE_PROGRAM_START))
		for range 10 {
			transport.InjectResponseForTest(fakeMQTTResponse(protocol.RESPONSE_TYPE_PROGRAM_DATA))
		}
	}()

	payload := `{"text":"hi","mode":"static","speed":5,"color":"#FF0000","font_size":16}`
	if err := h.HandleTextCommand([]byte(payload)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleShowDeviceIDCommand_Connected(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{"on", "ON"},
		{"off", "OFF"},
		{"lowercase on", "on"},
		{"whitespace", "  OFF  "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, transport, client := connectedHandler()
			defer client.OverrideConnectedForTest(false)
			go func() { transport.InjectResponseForTest(fakeMQTTResponse(protocol.RESPONSE_TYPE_SET_DEVICE_INFO)) }()
			if err := h.HandleShowDeviceIDCommand([]byte(tt.payload)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestHandleRemoteCommand_Connected(t *testing.T) {
	h, transport, client := connectedHandler()
	defer client.OverrideConnectedForTest(false)
	go func() { transport.InjectResponseForTest(fakeMQTTResponse(protocol.RESPONSE_TYPE_SET_DEVICE_INFO)) }()
	if err := h.HandleRemoteCommand([]byte("ON")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleSwitchToggle_SetterError_Wrapped(t *testing.T) {
	// Disconnected controller: the setter returns a not-connected error.
	// handleSwitchToggle should wrap it with a "setting <label>" prefix.
	h := testHandler()
	err := h.HandleShowDeviceIDCommand([]byte("ON"))
	if err == nil {
		t.Fatal("expected error with disconnected controller, got nil")
	}
	if !strings.Contains(err.Error(), "show_id") {
		t.Errorf("error = %q, want label %q in message", err.Error(), "show_id")
	}
}

func TestHandleColorModeCommand_OffPropagatesSetColorError(t *testing.T) {
	// Disconnected; SetColor returns an error. The handler should wrap it.
	h := testHandler()
	err := h.HandleColorModeCommand([]byte("off"))
	if err == nil {
		t.Fatal("expected error on disconnected SetColor, got nil")
	}
	if !strings.Contains(err.Error(), "restoring") {
		t.Errorf("error = %q, want wrap mentioning 'restoring'", err.Error())
	}
}

func TestHandleColorSpeedCommand_PropagatesSetterError(t *testing.T) {
	// Disconnected controller surfaces a wrapped "setting color speed" error.
	h := testHandler()
	err := h.HandleColorSpeedCommand([]byte("5"))
	if err == nil {
		t.Fatal("expected error on disconnected SetColorSpeed, got nil")
	}
	if !strings.Contains(err.Error(), "setting color speed") {
		t.Errorf("error = %q, want wrap", err.Error())
	}
}

func TestHandleImageCommand_PropagatesDisplayError(t *testing.T) {
	// Valid JSON + valid base64 + valid mode/fit, but DisplayImage fails
	// because no controller is connected and the image bytes are garbage.
	h := testHandler()
	err := h.HandleImageCommand([]byte(`{
		"image_base64":"aGVsbG8=",
		"mode":"static",
		"fit":"letterbox"
	}`))
	if err == nil {
		t.Fatal("expected error on invalid image bytes, got nil")
	}
	if !strings.Contains(err.Error(), "displaying image") {
		t.Errorf("error = %q, want wrap mentioning 'displaying image'", err.Error())
	}
}

func TestHandleGIFCommand_PropagatesDisplayError(t *testing.T) {
	h := testHandler()
	err := h.HandleGIFCommand([]byte(`{
		"gif_base64":"aGVsbG8=",
		"fit":"letterbox"
	}`))
	if err == nil {
		t.Fatal("expected error on invalid GIF bytes, got nil")
	}
	if !strings.Contains(err.Error(), "displaying gif") {
		t.Errorf("error = %q, want wrap mentioning 'displaying gif'", err.Error())
	}
}

func TestHandleSwitchToggle_RejectsBadPayload(t *testing.T) {
	h := testHandler()
	for _, bad := range []string{"", "maybe", "TRUE", "1", "ON\nextra"} {
		if err := h.HandleShowDeviceIDCommand([]byte(bad)); err == nil {
			t.Errorf("payload %q: expected error, got nil", bad)
		}
	}
}

func TestHandleColorModeCommand_ValidMode(t *testing.T) {
	h, transport, client := connectedHandler()
	defer client.OverrideConnectedForTest(false)
	go func() { transport.InjectResponseForTest(fakeMQTTResponse(protocol.RESPONSE_TYPE_COLOR)) }()
	if err := h.HandleColorModeCommand([]byte("5")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleColorModeCommand_OffRestoresColor(t *testing.T) {
	h, transport, client := connectedHandler()
	defer client.OverrideConnectedForTest(false)
	// NewCommandHandler seeds the state with white (255,255,255), so
	// "off" should trigger a BuildColorCommand and expect RESPONSE_TYPE_COLOR.
	go func() { transport.InjectResponseForTest(fakeMQTTResponse(protocol.RESPONSE_TYPE_COLOR)) }()
	if err := h.HandleColorModeCommand([]byte("off")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleColorModeCommand_OffNoColor(t *testing.T) {
	// If state has no color, "off" is a no-op (no controller call at all).
	h := testHandler()
	h.mu.Lock()
	h.state.Color = nil
	h.mu.Unlock()
	if err := h.HandleColorModeCommand([]byte("off")); err != nil {
		t.Errorf("expected nil error for off with no color, got %v", err)
	}
}

func TestHandleColorModeCommand_InvalidInt(t *testing.T) {
	h := testHandler()
	if err := h.HandleColorModeCommand([]byte("not-a-number")); err == nil {
		t.Error("expected error for unparseable mode, got nil")
	}
}

func TestHandleColorModeCommand_InvalidModeID(t *testing.T) {
	// Mode 3 is explicitly rejected by the protocol builder (empty palette).
	h, transport, client := connectedHandler()
	defer client.OverrideConnectedForTest(false)
	_ = transport // no response expected — the error surfaces before the send
	if err := h.HandleColorModeCommand([]byte("3")); err == nil {
		t.Error("expected error for invalid mode ID 3, got nil")
	}
}

func TestHandleColorSpeedCommand_Valid(t *testing.T) {
	h, transport, client := connectedHandler()
	defer client.OverrideConnectedForTest(false)
	go func() { transport.InjectResponseForTest(fakeMQTTResponse(protocol.RESPONSE_TYPE_COLOR)) }()
	if err := h.HandleColorSpeedCommand([]byte("5")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleColorSpeedCommand_BelowMinClamps(t *testing.T) {
	h, transport, client := connectedHandler()
	defer client.OverrideConnectedForTest(false)
	go func() { transport.InjectResponseForTest(fakeMQTTResponse(protocol.RESPONSE_TYPE_COLOR)) }()
	// 0 should be clamped up to 1 rather than rejected.
	if err := h.HandleColorSpeedCommand([]byte("0")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleColorSpeedCommand_AboveMaxClamps(t *testing.T) {
	h, transport, client := connectedHandler()
	defer client.OverrideConnectedForTest(false)
	go func() { transport.InjectResponseForTest(fakeMQTTResponse(protocol.RESPONSE_TYPE_COLOR)) }()
	if err := h.HandleColorSpeedCommand([]byte("99")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleColorSpeedCommand_InvalidPayload(t *testing.T) {
	h := testHandler()
	if err := h.HandleColorSpeedCommand([]byte("fast")); err == nil {
		t.Error("expected error for unparseable speed, got nil")
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
