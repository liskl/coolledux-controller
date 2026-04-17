//go:build integration && rest

package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// harness holds the shared HTTP client and the cached device ID picked on
// startup. One service connection is probed in TestMain; every test reuses
// the same base URL and client.
type harness struct {
	baseURL  string
	client   *http.Client
	deviceID string

	// Baseline panel state captured at suite start so TestMain can restore
	// brightness / flip / power on exit.
	baseBrightness uint8
	baseFlipMode   string
	basePower      bool
}

var (
	h   *harness
	hMu sync.Mutex
)

// TestMain probes the service, picks a device, snapshots its state, runs
// the suite, then restores the baseline. If the service is unreachable we
// skip cleanly so `make test-integration-rest` stays green on machines
// without a running coolledux-controller.
func TestMain(m *testing.M) {
	baseURL := strings.TrimRight(os.Getenv("COOLLEDUX_TEST_API"), "/")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	client := &http.Client{Timeout: 20 * time.Second}

	// Probe health before anything else — a connection-refused error here
	// should skip the suite, not fail it.
	if !probeReachable(client, baseURL) {
		fmt.Fprintf(os.Stderr, "service at %s is unreachable — REST integration tests skipped\n", baseURL)
		os.Exit(0)
	}

	deviceID, err := firstDeviceID(client, baseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "discovering device ID: %v\n", err)
		os.Exit(1)
	}
	if deviceID == "" {
		fmt.Fprintf(os.Stderr, "no devices registered at %s — REST integration tests skipped\n", baseURL)
		os.Exit(0)
	}

	info, err := getDeviceInfo(client, baseURL, deviceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "baseline device info: %v\n", err)
		os.Exit(1)
	}

	h = &harness{
		baseURL:        baseURL,
		client:         client,
		deviceID:       deviceID,
		baseBrightness: uint8FromJSON(info["brightness"]),
		baseFlipMode:   flipModeName(uint8FromJSON(info["flip_mode"])),
		basePower:      boolFromJSON(info["power"]),
	}

	code := m.Run()

	// Best-effort restore. Failures are logged, not fatal — we already have
	// the test results.
	if err := postJSON(client, baseURL, "/device/"+deviceID+"/brightness",
		map[string]any{"brightness": h.baseBrightness}, nil); err != nil {
		fmt.Fprintf(os.Stderr, "restore brightness: %v\n", err)
	}
	if err := postJSON(client, baseURL, "/device/"+deviceID+"/flip",
		map[string]any{"mode": h.baseFlipMode}, nil); err != nil {
		fmt.Fprintf(os.Stderr, "restore flip: %v\n", err)
	}
	powerState := "off"
	if h.basePower {
		powerState = "on"
	}
	if err := postJSON(client, baseURL, "/device/"+deviceID+"/power",
		map[string]any{"state": powerState}, nil); err != nil {
		fmt.Fprintf(os.Stderr, "restore power: %v\n", err)
	}

	os.Exit(code)
}

// rig returns the shared harness, failing the test cleanly if TestMain bailed.
func rig(t *testing.T) *harness {
	t.Helper()
	hMu.Lock()
	defer hMu.Unlock()
	if h == nil {
		t.Fatal("harness not initialized — TestMain must have bailed")
	}
	return h
}

// probeReachable returns true when GET /health answers with any 2xx response.
// A connection error (service not running) returns false and is the cue to
// skip the suite rather than fail.
func probeReachable(client *http.Client, baseURL string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// firstDeviceID returns the ID of the first device in /devices, or empty
// string if the list is empty. An HTTP error is propagated.
func firstDeviceID(client *http.Client, baseURL string) (string, error) {
	var body struct {
		Success bool `json:"success"`
		Devices []struct {
			ID string `json:"id"`
		} `json:"devices"`
	}
	if err := doJSON(client, http.MethodGet, baseURL+"/devices", nil, &body); err != nil {
		return "", err
	}
	if len(body.Devices) == 0 {
		return "", nil
	}
	return body.Devices[0].ID, nil
}

// getDeviceInfo hits /device/:id/info and returns the decoded JSON as a map.
// We use a map instead of a typed struct so the test doesn't have to track
// every field the API layer adds; individual tests pluck out what they need.
func getDeviceInfo(client *http.Client, baseURL, id string) (map[string]any, error) {
	var out map[string]any
	if err := doJSON(client, http.MethodGet, baseURL+"/device/"+id+"/info", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// doJSON executes a request with an optional JSON body and decodes a JSON
// response. A non-2xx status returns an error with the response body so test
// failures mention the actual server error.
func doJSON(client *http.Client, method, url string, body, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	buf, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %d %s", method, url, resp.StatusCode, strings.TrimSpace(string(buf)))
	}
	if out != nil && len(buf) > 0 {
		return json.Unmarshal(buf, out)
	}
	return nil
}

// postJSON is the mutation sibling of doJSON.
func postJSON(client *http.Client, baseURL, path string, body, out any) error {
	return doJSON(client, http.MethodPost, baseURL+path, body, out)
}

// doRaw is for tests that want to assert a status code (e.g. expected 400/404)
// without failing on non-2xx responses. Returns the status, body, error tuple.
func doRaw(client *http.Client, method, url string, body any) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(buf)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	buf, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, buf, nil
}

// uint8FromJSON coerces a JSON number (float64) into a uint8 with clamping.
func uint8FromJSON(v any) uint8 {
	f, ok := v.(float64)
	if !ok {
		return 0
	}
	if f < 0 {
		return 0
	}
	if f > 255 {
		return 255
	}
	return uint8(f)
}

// boolFromJSON pulls a bool out of a JSON value.
func boolFromJSON(v any) bool {
	b, _ := v.(bool)
	return b
}

// flipModeName converts the numeric flip_mode returned by /info into the
// string name that POST /flip expects.
func flipModeName(v uint8) string {
	switch v {
	case 0:
		return "none"
	case 1:
		return "horizontal"
	case 2:
		return "vertical"
	case 3:
		return "both"
	}
	return "none"
}
