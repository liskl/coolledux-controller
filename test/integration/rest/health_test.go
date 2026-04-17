//go:build integration && rest

package rest

import (
	"net/http"
	"testing"
)

// TestHealth hits GET /health and asserts the documented shape. Does not
// require a BLE connection — this is the "service is alive" sanity check.
func TestHealth(t *testing.T) {
	r := rig(t)

	var body struct {
		Status        string `json:"status"`
		BLEConnected  bool   `json:"ble_connected"`
		MQTTConnected bool   `json:"mqtt_connected"`
		UptimeSeconds int64  `json:"uptime_seconds"`
	}
	if err := doJSON(r.client, http.MethodGet, r.baseURL+"/health", nil, &body); err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("status = %q, want %q", body.Status, "ok")
	}
	if body.UptimeSeconds < 0 {
		t.Errorf("uptime_seconds = %d, want >= 0", body.UptimeSeconds)
	}
}

// TestFonts verifies the /fonts endpoint returns a non-empty font list with
// the three names we know ship with the binary. This catches a stripped
// //go:embed at build time.
func TestFonts(t *testing.T) {
	r := rig(t)

	var body struct {
		Default string `json:"default"`
		Fonts   []struct {
			Name string `json:"name"`
		} `json:"fonts"`
	}
	if err := doJSON(r.client, http.MethodGet, r.baseURL+"/fonts", nil, &body); err != nil {
		t.Fatalf("GET /fonts: %v", err)
	}
	if body.Default == "" {
		t.Error("default font is empty")
	}

	names := make(map[string]bool, len(body.Fonts))
	for _, f := range body.Fonts {
		names[f.Name] = true
	}
	for _, want := range []string{"7x13", "7x14b", "8x16"} {
		if !names[want] {
			t.Errorf("font %q missing from /fonts response", want)
		}
	}
}

// TestDevices verifies the /devices endpoint returns our harness device in
// the list. Implicitly confirms registry wiring end-to-end.
func TestDevices(t *testing.T) {
	r := rig(t)

	var body struct {
		Success bool `json:"success"`
		Devices []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			MAC       string `json:"mac"`
			Connected bool   `json:"connected"`
		} `json:"devices"`
	}
	if err := doJSON(r.client, http.MethodGet, r.baseURL+"/devices", nil, &body); err != nil {
		t.Fatalf("GET /devices: %v", err)
	}
	if !body.Success {
		t.Fatal("/devices reported success=false")
	}

	var found bool
	for _, d := range body.Devices {
		if d.ID == r.deviceID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("harness device %q not in /devices list", r.deviceID)
	}
}

// TestUnknownDevice_404 asserts resolve() rejects an unregistered ID with a
// Fiber 404 rather than panicking or returning 500.
func TestUnknownDevice_404(t *testing.T) {
	r := rig(t)
	status, _, err := doRaw(r.client, http.MethodGet, r.baseURL+"/device/deadbeefcafe/info", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if status != http.StatusNotFound {
		t.Errorf("status = %d, want 404", status)
	}
}
