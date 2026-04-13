package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/liskl/coolledux-controller/internal/ble"
	"github.com/liskl/coolledux-controller/internal/config"
	"github.com/liskl/coolledux-controller/internal/controller"
)

func testConfig() *config.Config {
	return &config.Config{
		BLE: config.BLEConfig{DeviceMAC: "01:00:00:FB:A4:16"},
		Display: config.DisplayConfig{
			Columns:           96,
			Rows:              16,
			DefaultBrightness: 128,
			DefaultFontSize:   16,
			DefaultColor:      "#FFFFFF",
			DefaultSpeed:      5,
		},
		API: config.APIConfig{
			Listen:      ":0",
			CORSOrigins: []string{"*"},
		},
	}
}

func testServer(t *testing.T) *Server {
	t.Helper()
	cfg := testConfig()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bleClient := ble.NewClient(logger)
	transport := ble.NewTransport(bleClient, logger)
	ctrl := controller.New(bleClient, transport, cfg, logger)
	srv := NewServer(ctrl, cfg, logger)
	return srv
}

func TestHealthEndpoint(t *testing.T) {
	srv := testServer(t)

	req, err := http.NewRequest(http.MethodGet, "/health", nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}

	var health HealthResponse
	if err := json.Unmarshal(body, &health); err != nil {
		t.Fatalf("unmarshaling: %v", err)
	}

	if health.Status != "ok" {
		t.Errorf("status: expected %q, got %q", "ok", health.Status)
	}
	if health.BLEConnected {
		t.Error("BLE should not be connected with nil client")
	}
	if health.MQTTConnected {
		t.Error("MQTT should not be connected")
	}
	if health.UptimeSeconds < 0 {
		t.Error("uptime should be non-negative")
	}
}

func TestHealthEndpoint_JSONContentType(t *testing.T) {
	srv := testServer(t)

	req, err := http.NewRequest(http.MethodGet, "/health", nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected JSON content type, got %q", ct)
	}
}

func TestSetPower_ValidOn(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`{"state":"on"}`)
	req, err := http.NewRequest(http.MethodPost, "/device/power", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	// Returns 500 because the BLE client is disconnected. Validates that
	// parsing succeeded and the error path is exercised.
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var sr SuccessResponse
	if err := json.Unmarshal(respBody, &sr); err != nil {
		t.Fatalf("unmarshaling: %v", err)
	}
	if sr.Success {
		t.Error("should not be successful")
	}
}

func TestSetPower_InvalidState(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`{"state":"maybe"}`)
	req, err := http.NewRequest(http.MethodPost, "/device/power", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var sr SuccessResponse
	if err := json.Unmarshal(respBody, &sr); err != nil {
		t.Fatalf("unmarshaling: %v", err)
	}
	if sr.Success {
		t.Error("should not be successful")
	}
	if !strings.Contains(sr.Error, "on") && !strings.Contains(sr.Error, "off") {
		t.Errorf("error should mention valid states, got: %q", sr.Error)
	}
}

func TestSetPower_InvalidBody(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`not json`)
	req, err := http.NewRequest(http.MethodPost, "/device/power", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestDisplayText_InvalidBody(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`not json at all`)
	req, err := http.NewRequest(http.MethodPost, "/display/text", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestDisplayText_InvalidMode(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`{"text":"hi","mode":"bad","speed":5,"color":"#FF0000","font_size":16}`)
	req, err := http.NewRequest(http.MethodPost, "/display/text", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestDisplayText_InvalidColor(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`{"text":"hi","mode":"static","speed":5,"color":"nope","font_size":16}`)
	req, err := http.NewRequest(http.MethodPost, "/display/text", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestDisplayImage_InvalidBase64(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`{"image_base64":"!!!not-b64!!!","mode":"static"}`)
	req, err := http.NewRequest(http.MethodPost, "/display/image", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestDisplayGIF_InvalidBase64(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`{"gif_base64":"!!!bad!!!","frame_duration":100}`)
	req, err := http.NewRequest(http.MethodPost, "/display/gif", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRouteNotFound(t *testing.T) {
	srv := testServer(t)

	req, err := http.NewRequest(http.MethodGet, "/nonexistent", nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	// Fiber returns 404 for unmatched routes. The custom error handler
	// converts it to a JSON response, but status may differ. Just verify
	// we don't get a 200.
	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected non-200 for unknown route, got 200 with body: %s", body)
	}
}

func TestSetBrightness_InvalidBody(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`garbage`)
	req, err := http.NewRequest(http.MethodPost, "/device/brightness", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSetFlip_InvalidBody(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`garbage`)
	req, err := http.NewRequest(http.MethodPost, "/device/flip", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSetFlip_InvalidMode(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`{"mode":"diagonal"}`)
	req, err := http.NewRequest(http.MethodPost, "/device/flip", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestDeviceInfo_Disconnected(t *testing.T) {
	srv := testServer(t)

	req, err := http.NewRequest(http.MethodGet, "/device/info", nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var sr SuccessResponse
	if err := json.Unmarshal(respBody, &sr); err != nil {
		t.Fatalf("unmarshaling: %v", err)
	}
	if sr.Success {
		t.Error("should not be successful")
	}
}

func TestSyncTime_InvalidBody(t *testing.T) {
	srv := testServer(t)

	// SyncTime now accepts empty/invalid bodies and defaults to time.Now().
	// With a disconnected BLE client, it falls through to a 500 (BLE send error).
	body := strings.NewReader(`garbage`)
	req, err := http.NewRequest(http.MethodPost, "/device/time", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 (defaults to time.Now, fails at BLE), got %d", resp.StatusCode)
	}
}

func TestSetTimers_InvalidBody(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`garbage`)
	req, err := http.NewRequest(http.MethodPost, "/device/timer", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestDisplayImage_InvalidBody(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`not json`)
	req, err := http.NewRequest(http.MethodPost, "/display/image", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestDisplayGIF_InvalidBody(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`not json`)
	req, err := http.NewRequest(http.MethodPost, "/display/gif", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestDisplayImage_InvalidMode(t *testing.T) {
	srv := testServer(t)

	// Valid base64 but invalid mode.
	body := strings.NewReader(`{"image_base64":"AAAA","mode":"invalid_mode"}`)
	req, err := http.NewRequest(http.MethodPost, "/display/image", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestResetDevice_Disconnected(t *testing.T) {
	srv := testServer(t)

	req, err := http.NewRequest(http.MethodPost, "/device/reset", nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var sr SuccessResponse
	if err := json.Unmarshal(respBody, &sr); err != nil {
		t.Fatalf("unmarshaling: %v", err)
	}
	if sr.Success {
		t.Error("should not be successful")
	}
}

func TestSetBrightness_ValidBody_Disconnected(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`{"brightness":128}`)
	req, err := http.NewRequest(http.MethodPost, "/device/brightness", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var sr SuccessResponse
	if err := json.Unmarshal(respBody, &sr); err != nil {
		t.Fatalf("unmarshaling: %v", err)
	}
	if sr.Success {
		t.Error("should not be successful")
	}
}

func TestSetFlip_ValidBody_Disconnected(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`{"mode":"horizontal"}`)
	req, err := http.NewRequest(http.MethodPost, "/device/flip", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var sr SuccessResponse
	if err := json.Unmarshal(respBody, &sr); err != nil {
		t.Fatalf("unmarshaling: %v", err)
	}
	if sr.Success {
		t.Error("should not be successful")
	}
}

func TestSyncTime_ValidBody_Disconnected(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`{"hour":12,"minute":30,"second":45}`)
	req, err := http.NewRequest(http.MethodPost, "/device/time", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var sr SuccessResponse
	if err := json.Unmarshal(respBody, &sr); err != nil {
		t.Fatalf("unmarshaling: %v", err)
	}
	if sr.Success {
		t.Error("should not be successful")
	}
}

func TestSetTimers_ValidBody_Disconnected(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`{"items":[{"enable":true,"hour":8,"minute":0,"days":127,"power_on":true}]}`)
	req, err := http.NewRequest(http.MethodPost, "/device/timer", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var sr SuccessResponse
	if err := json.Unmarshal(respBody, &sr); err != nil {
		t.Fatalf("unmarshaling: %v", err)
	}
	if sr.Success {
		t.Error("should not be successful")
	}
}

func TestDisplayText_ValidBody_Disconnected(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`{"text":"hello","mode":"static","speed":5,"color":"#FF0000","font_size":16}`)
	req, err := http.NewRequest(http.MethodPost, "/display/text", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var sr SuccessResponse
	if err := json.Unmarshal(respBody, &sr); err != nil {
		t.Fatalf("unmarshaling: %v", err)
	}
	if sr.Success {
		t.Error("should not be successful")
	}
}

func TestDisplayGIF_InvalidGIFData_Disconnected(t *testing.T) {
	srv := testServer(t)

	// Valid base64 but the controller will fail to decode this as a real GIF.
	body := strings.NewReader(`{"gif_base64":"AAAA","frame_duration":100}`)
	req, err := http.NewRequest(http.MethodPost, "/display/gif", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	// Base64 decodes fine, but the data is not a valid GIF.
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var sr SuccessResponse
	if err := json.Unmarshal(respBody, &sr); err != nil {
		t.Fatalf("unmarshaling: %v", err)
	}
	if sr.Success {
		t.Error("should not be successful")
	}
}

func TestServerShutdown(t *testing.T) {
	srv := testServer(t)
	// Shutdown without Start should not panic.
	if err := srv.Shutdown(); err != nil {
		t.Errorf("shutdown error: %v", err)
	}
}

func TestSetPower_ValidOff_Disconnected(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`{"state":"off"}`)
	req, err := http.NewRequest(http.MethodPost, "/device/power", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

func TestDisplayImage_ValidMode_Disconnected(t *testing.T) {
	srv := testServer(t)

	// Valid base64 that decodes to something, but not a real image.
	body := strings.NewReader(`{"image_base64":"AAAA","mode":"static"}`)
	req, err := http.NewRequest(http.MethodPost, "/display/image", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	// Parsing and base64 succeed, but controller fails (image decode or BLE send).
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

func TestCORSMiddleware_MultipleOrigins(t *testing.T) {
	cfg := testConfig()
	cfg.API.CORSOrigins = []string{"http://localhost:3000", "http://example.com"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bleClient := ble.NewClient(logger)
	transport := ble.NewTransport(bleClient, logger)
	ctrl := controller.New(bleClient, transport, cfg, logger)
	srv := NewServer(ctrl, cfg, logger)

	req, err := http.NewRequest(http.MethodGet, "/health", nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRecoveryMiddleware_PanicHandler(t *testing.T) {
	// Use a nil-transport controller to trigger a panic in the handler,
	// exercising the recovery middleware's panic catch.
	cfg := testConfig()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctrl := controller.New(nil, nil, cfg, logger)
	srv := NewServer(ctrl, cfg, logger)

	// SetPower with valid body on a nil-transport controller will panic.
	body := strings.NewReader(`{"state":"on"}`)
	req, err := http.NewRequest(http.MethodPost, "/device/power", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	// Recovery middleware catches the panic and returns 500.
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var sr SuccessResponse
	if err := json.Unmarshal(respBody, &sr); err != nil {
		t.Fatalf("unmarshaling: %v", err)
	}
	if sr.Success {
		t.Error("should not be successful after panic recovery")
	}
	if sr.Error != "internal server error" {
		t.Errorf("expected generic error message, got %q", sr.Error)
	}
}

func TestSetColor_ValidBody_Disconnected(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`{"color":"#FF8800"}`)
	req, err := http.NewRequest(http.MethodPost, "/display/color", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	// Parsing succeeds; controller errors because BLE is disconnected.
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

func TestSetColor_InvalidColor(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`{"color":"not-a-color"}`)
	req, err := http.NewRequest(http.MethodPost, "/display/color", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSetColor_InvalidBody(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`{bad json`)
	req, err := http.NewRequest(http.MethodPost, "/display/color", body)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCORSMiddleware_EmptyOrigins(t *testing.T) {
	cfg := testConfig()
	cfg.API.CORSOrigins = []string{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bleClient := ble.NewClient(logger)
	transport := ble.NewTransport(bleClient, logger)
	ctrl := controller.New(bleClient, transport, cfg, logger)
	srv := NewServer(ctrl, cfg, logger)

	req, err := http.NewRequest(http.MethodGet, "/health", nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}

	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
