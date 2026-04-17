//go:build integration && rest

package rest

import (
	"net/http"
	"testing"
)

// TestDeviceInfo_Shape queries the real panel's info endpoint and checks the
// fields we depend on for later round-trip tests. Everything here requires a
// connected BLE panel behind the service.
func TestDeviceInfo_Shape(t *testing.T) {
	r := rig(t)
	info, err := getDeviceInfo(r.client, r.baseURL, r.deviceID)
	if err != nil {
		t.Fatalf("GET /device/%s/info: %v", r.deviceID, err)
	}

	for _, key := range []string{"power", "brightness", "flip_mode"} {
		if _, ok := info[key]; !ok {
			t.Errorf("device info missing key %q", key)
		}
	}
	if flip := uint8FromJSON(info["flip_mode"]); flip > 3 {
		t.Errorf("flip_mode = %d, want 0..3", flip)
	}
}

// TestBrightness_RoundTrip sets a distinct brightness, GETs /info, and
// verifies the device reports it back (within the ±2 quantization tolerance
// the BLE suite established). The harness restores the baseline at teardown.
func TestBrightness_RoundTrip(t *testing.T) {
	r := rig(t)
	t.Cleanup(func() {
		_ = postJSON(r.client, r.baseURL, "/device/"+r.deviceID+"/brightness",
			map[string]any{"brightness": r.baseBrightness}, nil)
	})

	target := uint8(64)
	if r.baseBrightness < 100 {
		target = 200
	}
	if err := postJSON(r.client, r.baseURL, "/device/"+r.deviceID+"/brightness",
		map[string]any{"brightness": target}, nil); err != nil {
		t.Fatalf("POST /brightness: %v", err)
	}

	info, err := getDeviceInfo(r.client, r.baseURL, r.deviceID)
	if err != nil {
		t.Fatalf("GET /info: %v", err)
	}
	got := uint8FromJSON(info["brightness"])
	if diff := absDiff(got, target); diff > 2 {
		t.Errorf("brightness readback = %d, want ≈%d (diff %d)", got, target, diff)
	}
}

// TestFlip_RoundTrip walks all four flip modes via POST /flip and checks the
// /info endpoint reflects each. This end-to-end covers: schema routing, the
// controller command, BLE transport, and response parsing.
func TestFlip_RoundTrip(t *testing.T) {
	r := rig(t)
	t.Cleanup(func() {
		_ = postJSON(r.client, r.baseURL, "/device/"+r.deviceID+"/flip",
			map[string]any{"mode": r.baseFlipMode}, nil)
	})

	cases := []struct {
		name string
		want uint8
	}{
		{"none", 0},
		{"horizontal", 1},
		{"vertical", 2},
		{"both", 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := postJSON(r.client, r.baseURL, "/device/"+r.deviceID+"/flip",
				map[string]any{"mode": tc.name}, nil); err != nil {
				t.Fatalf("POST /flip %s: %v", tc.name, err)
			}
			info, err := getDeviceInfo(r.client, r.baseURL, r.deviceID)
			if err != nil {
				t.Fatalf("GET /info: %v", err)
			}
			if got := uint8FromJSON(info["flip_mode"]); got != tc.want {
				t.Errorf("flip readback = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestPower_RoundTrip toggles the panel to the opposite of its current
// state, verifies via /info, then restores. Placed after display-sensitive
// tests so the screen isn't off for the rest of the suite.
func TestPower_RoundTrip(t *testing.T) {
	r := rig(t)
	info, err := getDeviceInfo(r.client, r.baseURL, r.deviceID)
	if err != nil {
		t.Fatalf("baseline /info: %v", err)
	}
	before := boolFromJSON(info["power"])
	t.Cleanup(func() {
		state := "off"
		if before {
			state = "on"
		}
		_ = postJSON(r.client, r.baseURL, "/device/"+r.deviceID+"/power",
			map[string]any{"state": state}, nil)
	})

	want := "on"
	wantBool := true
	if before {
		want = "off"
		wantBool = false
	}
	if err := postJSON(r.client, r.baseURL, "/device/"+r.deviceID+"/power",
		map[string]any{"state": want}, nil); err != nil {
		t.Fatalf("POST /power %s: %v", want, err)
	}
	after, err := getDeviceInfo(r.client, r.baseURL, r.deviceID)
	if err != nil {
		t.Fatalf("post-set /info: %v", err)
	}
	if got := boolFromJSON(after["power"]); got != wantBool {
		t.Errorf("power readback = %v, want %v", got, wantBool)
	}
}

// TestSetColor_ACKs exercises /color with four distinct RGB values. The
// panel doesn't expose the current tint via /info, so we can only assert
// the handler returned 200 success for each.
func TestSetColor_ACKs(t *testing.T) {
	r := rig(t)
	for _, color := range []string{"#FF0000", "#00FF00", "#0000FF", "#FFFFFF"} {
		t.Run(color, func(t *testing.T) {
			if err := postJSON(r.client, r.baseURL, "/device/"+r.deviceID+"/color",
				map[string]any{"color": color}, nil); err != nil {
				t.Errorf("POST /color %s: %v", color, err)
			}
		})
	}
}

// TestSetChannel_ACKs hits /channel for slot 1. Every shipping firmware has
// at least one program slot, so this is safe regardless of what's been
// previously uploaded.
func TestSetChannel_ACKs(t *testing.T) {
	r := rig(t)
	if err := postJSON(r.client, r.baseURL, "/device/"+r.deviceID+"/channel",
		map[string]any{"channel": 1}, nil); err != nil {
		t.Errorf("POST /channel: %v", err)
	}
}

// TestBrightness_InvalidBody_400 sends garbage JSON and asserts the handler
// rejects it with a 400 rather than a 500. End-to-end check that BodyParser
// error handling is wired correctly at the Fiber layer.
func TestBrightness_InvalidBody_400(t *testing.T) {
	r := rig(t)
	status, _, err := doRaw(r.client, http.MethodPost,
		r.baseURL+"/device/"+r.deviceID+"/brightness", "not-json")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if status != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", status)
	}
}

// TestPower_InvalidState_400 asserts the server rejects power states other
// than "on"/"off" at the handler layer, not the BLE layer.
func TestPower_InvalidState_400(t *testing.T) {
	r := rig(t)
	status, _, err := doRaw(r.client, http.MethodPost,
		r.baseURL+"/device/"+r.deviceID+"/power",
		map[string]any{"state": "maybe"})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if status != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", status)
	}
}

func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}
