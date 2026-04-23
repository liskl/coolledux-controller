package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/liskl/coolledux-controller/internal/ble"
	"github.com/liskl/coolledux-controller/internal/config"
	"github.com/liskl/coolledux-controller/internal/controller"
	"github.com/liskl/coolledux-controller/internal/protocol"
	"github.com/liskl/coolledux-controller/internal/registry"
	"github.com/liskl/coolledux-controller/internal/text"
)

// testRig bundles the Server with the BLE hooks tests need to fake device I/O.
// Use this instead of testServer(t) when a test needs to override sends or
// inject responses.
type testRig struct {
	srv       *Server
	bleClient *ble.Client
	transport *ble.Transport
	ctrl      *controller.Controller
	cfg       *config.Config
}

// newTestRig builds a Server with a stubbed BLE client. The controller is
// marked connected so handlers that gate on IsConnected proceed; all outbound
// BLE writes are silently accepted unless overridden. The registry is
// populated with the primary device so /device/:id/... routes resolve.
func newTestRig(t *testing.T) *testRig {
	t.Helper()
	cfg := testConfig()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bleClient := ble.NewClient(logger)
	transport := ble.NewTransport(bleClient, logger)
	ctrl := controller.New(bleClient, transport, cfg, logger)

	bleClient.OverrideConnectedForTest(true)
	bleClient.OverrideSendFuncForTest(func(ctx context.Context, data []byte) error {
		return nil
	})
	ctrl.OverrideStateForTest(controller.StateConnected)

	reg := registry.New()
	if err := reg.Add(&registry.Entry{
		ID:         testPrimaryID,
		Name:       "primary",
		MAC:        cfg.BLE.DeviceMAC,
		Client:     bleClient,
		Transport:  transport,
		Controller: ctrl,
	}); err != nil {
		t.Fatalf("registry add: %v", err)
	}

	srv := NewServer(ctrl, cfg, logger, reg, nil)
	return &testRig{
		srv:       srv,
		bleClient: bleClient,
		transport: transport,
		ctrl:      ctrl,
		cfg:       cfg,
	}
}

// doJSONRequest is a small convenience for sending a JSON POST and parsing
// the SuccessResponse body.
func doJSONRequest(t *testing.T, srv *Server, method, path, body string) (*http.Response, []byte) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, path, r)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := srv.app.Test(req, -1)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	buf, _ := io.ReadAll(resp.Body)
	return resp, buf
}

// fakeOK builds a stream-framed success response for the given response type.
func fakeOK(respType byte) []byte {
	return protocol.BuildStreamFrame([]byte{respType, protocol.STATUS_SUCCESS})
}

// ---------- ListFonts ----------

func TestListFonts(t *testing.T) {
	srv := testServer(t)
	resp, body := doJSONRequest(t, srv, http.MethodGet, "/fonts", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var out FontsResponse
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Default != text.DefaultFontName {
		t.Errorf("default = %q, want %q", out.Default, text.DefaultFontName)
	}
	if len(out.Fonts) < 3 {
		t.Fatalf("expected at least 3 fonts, got %d", len(out.Fonts))
	}
	names := map[string]bool{}
	for _, f := range out.Fonts {
		names[f.Name] = true
	}
	for _, want := range []string{"7x13", "7x14b", "8x16"} {
		if !names[want] {
			t.Errorf("missing font %q in response: %v", want, names)
		}
	}
}

// ---------- SetChannel ----------

func TestSetChannel_InvalidBody(t *testing.T) {
	srv := testServer(t)
	resp, _ := doJSONRequest(t, srv, http.MethodPost, "/device/010000fba416/channel", `garbage`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSetChannel_Disconnected(t *testing.T) {
	srv := testServer(t)
	// With the default (disconnected) controller, the request parses but the
	// BLE send times out/fails, yielding 500.
	resp, _ := doJSONRequest(t, srv, http.MethodPost, "/device/010000fba416/channel", `{"channel":3}`)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

func TestSetChannel_Connected(t *testing.T) {
	rig := newTestRig(t)
	go rig.transport.InjectResponseForTest(fakeOK(protocol.RESPONSE_TYPE_CHANNEL))
	resp, body := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/channel", `{"channel":2}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	var sr SuccessResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !sr.Success {
		t.Errorf("expected success, got error %q", sr.Error)
	}
}

// ---------- GetTimers ----------

func TestGetTimers_Connected(t *testing.T) {
	rig := newTestRig(t)
	// GetTimers just returns the raw response frame; a minimal framed payload
	// is fine.
	go rig.transport.InjectResponseForTest(fakeOK(protocol.RESPONSE_TYPE_GET_TIMER))
	resp, body := doJSONRequest(t, rig.srv, http.MethodGet, "/device/010000fba416/timer", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["success"] != true {
		t.Errorf("expected success=true, got %v", out["success"])
	}
	if out["data"] == nil {
		t.Error("expected data field to be present")
	}
}

func TestGetTimers_Disconnected(t *testing.T) {
	// With no response injected and BLE disconnected, the SendAndWait fails
	// almost immediately. Use a short request to keep the test quick.
	srv := testServer(t)
	resp, _ := doJSONRequest(t, srv, http.MethodGet, "/device/010000fba416/timer", "")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

// ---------- CountdownProbeHandler ----------

func TestCountdownProbe_InvalidBody(t *testing.T) {
	srv := testServer(t)
	resp, _ := doJSONRequest(t, srv, http.MethodPost, "/device/010000fba416/debug/timecount", `nope`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCountdownProbe_WrongLengthHex(t *testing.T) {
	srv := testServer(t)
	// 4 hex chars, should be 78.
	resp, _ := doJSONRequest(t, srv, http.MethodPost, "/device/010000fba416/debug/timecount",
		`{"probe_hex":"abcd","color":"#FF0000"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCountdownProbe_InvalidHexChars(t *testing.T) {
	srv := testServer(t)
	// 78 chars but with non-hex "zz" near the end.
	hex := strings.Repeat("ab", 38) + "zz"
	resp, _ := doJSONRequest(t, srv, http.MethodPost, "/device/010000fba416/debug/timecount",
		`{"probe_hex":"`+hex+`","color":""}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCountdownProbe_InvalidColor(t *testing.T) {
	srv := testServer(t)
	hex := strings.Repeat("00", 39)
	resp, _ := doJSONRequest(t, srv, http.MethodPost, "/device/010000fba416/debug/timecount",
		`{"probe_hex":"`+hex+`","color":"not-a-color"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCountdownProbe_Disconnected(t *testing.T) {
	// Disconnected controller: CountdownProbe returns "device not connected".
	srv := testServer(t)
	hex := strings.Repeat("00", 39)
	resp, _ := doJSONRequest(t, srv, http.MethodPost, "/device/010000fba416/debug/timecount",
		`{"probe_hex":"`+hex+`","color":"#00FF00"}`)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

func TestCountdownProbe_Connected(t *testing.T) {
	rig := newTestRig(t)

	// Capture outbound bytes so we can assert the probe bitmap was sent.
	var mu sync.Mutex
	var writes [][]byte
	rig.bleClient.OverrideSendFuncForTest(func(ctx context.Context, data []byte) error {
		mu.Lock()
		writes = append(writes, append([]byte(nil), data...))
		mu.Unlock()
		return nil
	})

	// CountdownProbe goes through sendProgram: one PROGRAM_START ack + N data
	// acks. Inject generously; WaitForResponse on data chunks tolerates extras.
	go func() {
		rig.transport.InjectResponseForTest(fakeOK(protocol.RESPONSE_TYPE_PROGRAM_START))
		for i := 0; i < 40; i++ {
			rig.transport.InjectResponseForTest(fakeOK(protocol.RESPONSE_TYPE_PROGRAM_DATA))
		}
	}()

	// Probe bitmap: pattern we can search for in the chunked output. Use
	// 0xAA repeated for easy identification.
	hex := strings.Repeat("aa", 39)
	resp, body := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/debug/timecount",
		`{"probe_hex":"`+hex+`","color":"#00FF00"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	var sr SuccessResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !sr.Success {
		t.Errorf("expected success, got error %q", sr.Error)
	}
	mu.Lock()
	n := len(writes)
	mu.Unlock()
	if n == 0 {
		t.Error("expected at least one outbound BLE write")
	}
}

// ---------- Countdown ----------

func TestCountdown_InvalidBody(t *testing.T) {
	srv := testServer(t)
	resp, _ := doJSONRequest(t, srv, http.MethodPost, "/device/010000fba416/countdown", `not json`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCountdown_UnknownAction(t *testing.T) {
	srv := testServer(t)
	resp, body := doJSONRequest(t, srv, http.MethodPost, "/device/010000fba416/countdown", `{"action":"nope"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.StatusCode, body)
	}
	var sr SuccessResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !strings.Contains(sr.Error, "action must be") {
		t.Errorf("error should describe valid actions, got %q", sr.Error)
	}
}

func TestCountdown_FireAndForget(t *testing.T) {
	// Cover the set/start/stop/status paths. Each uses sendControl and
	// fire-and-forgets, so no response injection is required.
	actions := []string{"status", "set", "start", "stop"}
	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			rig := newTestRig(t)
			body := `{"action":"` + action + `","hour":1,"minute":30,"second":0}`
			resp, respBody := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/countdown", body)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", resp.StatusCode, respBody)
			}
			var sr SuccessResponse
			if err := json.Unmarshal(respBody, &sr); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if !sr.Success {
				t.Errorf("expected success, got error %q", sr.Error)
			}
		})
	}
}

func TestCountdown_FireAndForget_Disconnected(t *testing.T) {
	// Disconnected controller: sendControl returns "device not connected".
	srv := testServer(t)
	resp, _ := doJSONRequest(t, srv, http.MethodPost, "/device/010000fba416/countdown",
		`{"action":"start"}`)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

func TestCountdown_ShowInvalidColor(t *testing.T) {
	srv := testServer(t)
	resp, _ := doJSONRequest(t, srv, http.MethodPost, "/device/010000fba416/countdown",
		`{"action":"show","hour":0,"minute":1,"second":30,"color":"bogus"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCountdown_Show_Connected(t *testing.T) {
	rig := newTestRig(t)

	// CountdownDisplay does: sendProgram (start + chunk acks) + set + start.
	// The first two (set, start) are fire-and-forget.
	go func() {
		rig.transport.InjectResponseForTest(fakeOK(protocol.RESPONSE_TYPE_PROGRAM_START))
		for i := 0; i < 40; i++ {
			rig.transport.InjectResponseForTest(fakeOK(protocol.RESPONSE_TYPE_PROGRAM_DATA))
		}
	}()

	resp, body := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/countdown",
		`{"action":"show","hour":0,"minute":1,"second":0,"color":"#FF00FF"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
}

func TestCountdown_Show_DefaultColor_Connected(t *testing.T) {
	// Same as above but with no color field to cover the default-white branch.
	rig := newTestRig(t)
	go func() {
		rig.transport.InjectResponseForTest(fakeOK(protocol.RESPONSE_TYPE_PROGRAM_START))
		for i := 0; i < 40; i++ {
			rig.transport.InjectResponseForTest(fakeOK(protocol.RESPONSE_TYPE_PROGRAM_DATA))
		}
	}()
	resp, body := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/countdown",
		`{"action":"show","hour":0,"minute":0,"second":5}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
}

// ---------- Stopwatch ----------

func TestStopwatch_InvalidBody(t *testing.T) {
	srv := testServer(t)
	resp, _ := doJSONRequest(t, srv, http.MethodPost, "/device/010000fba416/stopwatch", `not json`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestStopwatch_UnknownAction(t *testing.T) {
	srv := testServer(t)
	resp, body := doJSONRequest(t, srv, http.MethodPost, "/device/010000fba416/stopwatch", `{"action":"flail"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.StatusCode, body)
	}
}

func TestStopwatch_FireAndForget(t *testing.T) {
	for _, action := range []string{"status", "reset", "start", "stop"} {
		t.Run(action, func(t *testing.T) {
			rig := newTestRig(t)
			resp, body := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/stopwatch",
				`{"action":"`+action+`"}`)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
			}
		})
	}
}

func TestStopwatch_Disconnected(t *testing.T) {
	srv := testServer(t)
	resp, _ := doJSONRequest(t, srv, http.MethodPost, "/device/010000fba416/stopwatch", `{"action":"start"}`)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

func TestStopwatch_ShowInvalidColor(t *testing.T) {
	srv := testServer(t)
	resp, _ := doJSONRequest(t, srv, http.MethodPost, "/device/010000fba416/stopwatch",
		`{"action":"show","color":"notacolor"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestStopwatch_Show_Connected(t *testing.T) {
	rig := newTestRig(t)
	go func() {
		rig.transport.InjectResponseForTest(fakeOK(protocol.RESPONSE_TYPE_PROGRAM_START))
		for i := 0; i < 40; i++ {
			rig.transport.InjectResponseForTest(fakeOK(protocol.RESPONSE_TYPE_PROGRAM_DATA))
		}
	}()

	resp, body := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/stopwatch",
		`{"action":"show","color":"#00FF00"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
}

func TestStopwatch_Show_DefaultColor_Connected(t *testing.T) {
	rig := newTestRig(t)
	go func() {
		rig.transport.InjectResponseForTest(fakeOK(protocol.RESPONSE_TYPE_PROGRAM_START))
		for i := 0; i < 40; i++ {
			rig.transport.InjectResponseForTest(fakeOK(protocol.RESPONSE_TYPE_PROGRAM_DATA))
		}
	}()
	resp, body := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/stopwatch",
		`{"action":"show"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
}

// ---------- Scoreboard ----------

func TestScoreboard_InvalidBody(t *testing.T) {
	srv := testServer(t)
	resp, _ := doJSONRequest(t, srv, http.MethodPost, "/device/010000fba416/scoreboard", `not json`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestScoreboard_UnknownAction(t *testing.T) {
	srv := testServer(t)
	resp, body := doJSONRequest(t, srv, http.MethodPost, "/device/010000fba416/scoreboard", `{"action":"wat"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.StatusCode, body)
	}
}

func TestScoreboard_FireAndForget(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"status", `{"action":"status"}`},
		{"set_scores", `{"action":"set_scores","score_a":5,"score_b":3}`},
		{"set_time", `{"action":"set_time","hour":2,"minute":15,"is_timer":true}`},
		{"start", `{"action":"start"}`},
		{"stop", `{"action":"stop"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rig := newTestRig(t)
			resp, body := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/scoreboard", tc.body)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
			}
		})
	}
}

func TestScoreboard_Disconnected(t *testing.T) {
	srv := testServer(t)
	resp, _ := doJSONRequest(t, srv, http.MethodPost, "/device/010000fba416/scoreboard",
		`{"action":"set_scores","score_a":1,"score_b":2}`)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

func TestScoreboard_ShowInvalidColor(t *testing.T) {
	srv := testServer(t)
	resp, _ := doJSONRequest(t, srv, http.MethodPost, "/device/010000fba416/scoreboard",
		`{"action":"show","color":"notacolor"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestScoreboard_Show_Connected(t *testing.T) {
	rig := newTestRig(t)
	go func() {
		rig.transport.InjectResponseForTest(fakeOK(protocol.RESPONSE_TYPE_PROGRAM_START))
		for i := 0; i < 40; i++ {
			rig.transport.InjectResponseForTest(fakeOK(protocol.RESPONSE_TYPE_PROGRAM_DATA))
		}
	}()

	resp, body := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/scoreboard",
		`{"action":"show","color":"#FF8800"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
}

func TestScoreboard_Show_DefaultColor_Connected(t *testing.T) {
	rig := newTestRig(t)
	go func() {
		rig.transport.InjectResponseForTest(fakeOK(protocol.RESPONSE_TYPE_PROGRAM_START))
		for i := 0; i < 40; i++ {
			rig.transport.InjectResponseForTest(fakeOK(protocol.RESPONSE_TYPE_PROGRAM_DATA))
		}
	}()
	resp, body := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/scoreboard",
		`{"action":"show"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
}

// ---------- Password endpoints ----------

func TestCheckPassword_Verified(t *testing.T) {
	rig := newTestRig(t)
	go rig.transport.InjectResponseForTest(
		protocol.BuildStreamFrame([]byte{protocol.RESPONSE_TYPE_PASSWORD_VERIFY, 0x00}),
	)
	resp, body := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/password/check",
		`{"password":"1234"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	var sr SuccessResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !sr.Success {
		t.Errorf("expected success=true, got error %q", sr.Error)
	}
}

func TestCheckPassword_Rejected_401(t *testing.T) {
	rig := newTestRig(t)
	go rig.transport.InjectResponseForTest(
		protocol.BuildStreamFrame([]byte{protocol.RESPONSE_TYPE_PASSWORD_VERIFY, 0x01}),
	)
	resp, _ := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/password/check",
		`{"password":"1234"}`)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestCheckPassword_InvalidInput_400(t *testing.T) {
	// Too-short password: the builder returns "password length ..." which the
	// handler maps to 400.
	rig := newTestRig(t)
	resp, _ := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/password/check",
		`{"password":"ab"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCheckPassword_InvalidBody(t *testing.T) {
	rig := newTestRig(t)
	resp, _ := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/password/check",
		`garbage`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSetPassword_Success(t *testing.T) {
	rig := newTestRig(t)
	go rig.transport.InjectResponseForTest(
		protocol.BuildStreamFrame([]byte{protocol.RESPONSE_TYPE_PASSWORD_SET, 0x00}),
	)
	resp, body := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/password/set",
		`{"password":"abcd"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
}

// ---------- SetColor / SetColorMode / SetColorSpeed ----------

func TestSetColor_Connected(t *testing.T) {
	rig := newTestRig(t)
	go rig.transport.InjectResponseForTest(fakeOK(protocol.RESPONSE_TYPE_COLOR))
	resp, body := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/color",
		`{"color":"#FF8800"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	var sr SuccessResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !sr.Success {
		t.Errorf("expected success, got error %q", sr.Error)
	}
}

func TestSetColor_InvalidHex(t *testing.T) {
	rig := newTestRig(t)
	resp, _ := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/color",
		`{"color":"notacolor"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSetColorMode_Connected(t *testing.T) {
	rig := newTestRig(t)
	go rig.transport.InjectResponseForTest(fakeOK(protocol.RESPONSE_TYPE_COLOR))
	resp, body := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/color/mode",
		`{"mode":5}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	var sr SuccessResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !sr.Success {
		t.Errorf("expected success, got error %q", sr.Error)
	}
}

func TestSetColorMode_InvalidBody(t *testing.T) {
	rig := newTestRig(t)
	resp, _ := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/color/mode", `garbage`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSetColorMode_UnsupportedMode(t *testing.T) {
	// Mode 3 is rejected by the protocol layer (empty palette) — the handler
	// should surface that as a 400, not a 500.
	rig := newTestRig(t)
	resp, _ := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/color/mode",
		`{"mode":3}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSetColorSpeed_Connected(t *testing.T) {
	rig := newTestRig(t)
	go rig.transport.InjectResponseForTest(fakeOK(protocol.RESPONSE_TYPE_COLOR))
	resp, body := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/color/speed",
		`{"speed":5}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	var sr SuccessResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !sr.Success {
		t.Errorf("expected success, got error %q", sr.Error)
	}
}

func TestSetColorSpeed_InvalidBody(t *testing.T) {
	rig := newTestRig(t)
	resp, _ := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/color/speed", `garbage`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// ---------- scanPrefix / scanTimeout ----------

func TestScanPrefix_Constant(t *testing.T) {
	h := NewHandlers(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if got := h.scanPrefix(); got != "CoolLEDUX" {
		t.Errorf("scanPrefix() = %q, want %q", got, "CoolLEDUX")
	}
}

func TestScanTimeout_DefaultsWhenUnset(t *testing.T) {
	h := NewHandlers(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	// scanTO zero-value should fall back to the 5s default.
	if got := h.scanTimeout(); got != 5*time.Second {
		t.Errorf("scanTimeout() = %v, want 5s", got)
	}
}

func TestScanTimeout_UsesConfiguredValue(t *testing.T) {
	h := NewHandlers(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h.scanTO = 15 * time.Second
	if got := h.scanTimeout(); got != 15*time.Second {
		t.Errorf("scanTimeout() = %v, want 15s", got)
	}
}

func TestScanDevices_NoRegistry_503(t *testing.T) {
	// When the server is constructed without a registry, /scan should
	// return 503 rather than panicking.
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctrl := controller.New(nil, nil, testConfig(), logger)
	srv := NewServer(ctrl, testConfig(), logger, nil, nil)

	resp, _ := doJSONRequest(t, srv, http.MethodPost, "/scan", "")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
}

func TestListDevices_NoRegistry_503(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctrl := controller.New(nil, nil, testConfig(), logger)
	srv := NewServer(ctrl, testConfig(), logger, nil, nil)

	resp, _ := doJSONRequest(t, srv, http.MethodGet, "/devices", "")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
}

// ---------- resolve sad paths ----------

func TestResolve_UnknownDeviceID_404(t *testing.T) {
	rig := newTestRig(t)
	resp, _ := doJSONRequest(t, rig.srv, http.MethodPost,
		"/device/deadbeefcafe/channel", `{"channel":1}`)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for unknown device id, got %d", resp.StatusCode)
	}
}

// ---------- DisplayImage / DisplayGIF sad paths ----------

func TestDisplayImage_InvalidFitMode(t *testing.T) {
	rig := newTestRig(t)
	// Valid base64 of "hello" but an invalid fit mode rejects before decode.
	body := `{"image_base64":"aGVsbG8=","mode":"static","fit":"bogus"}`
	resp, _ := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/image", body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid fit, got %d", resp.StatusCode)
	}
}

func TestDisplayImage_NonImageBytes_500(t *testing.T) {
	// Valid base64, but the bytes are not a decodable image. The handler
	// gets past BodyParser and base64 decode, then the controller rejects
	// it via ledimage.DecodeImage — which should surface as 500.
	rig := newTestRig(t)
	body := `{"image_base64":"aGVsbG8=","mode":"static","fit":"letterbox"}`
	resp, _ := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/image", body)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 for undecodable image, got %d", resp.StatusCode)
	}
}

func TestDisplayGIF_InvalidFitMode(t *testing.T) {
	rig := newTestRig(t)
	body := `{"gif_base64":"aGVsbG8=","fit":"bogus"}`
	resp, _ := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/gif", body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid fit, got %d", resp.StatusCode)
	}
}

func TestDisplayGIF_NonGIFBytes_500(t *testing.T) {
	rig := newTestRig(t)
	body := `{"gif_base64":"aGVsbG8=","fit":"letterbox"}`
	resp, _ := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/gif", body)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 for undecodable gif, got %d", resp.StatusCode)
	}
}

func TestDisplayGIF_Raw_MissingMagic_500(t *testing.T) {
	// raw=true skips the library decode and goes straight to the 0x0C path,
	// which checks for GIF87a/GIF89a magic. "hello" fails that check.
	rig := newTestRig(t)
	body := `{"gif_base64":"aGVsbG8=","raw":true}`
	resp, _ := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/gif", body)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 for missing GIF magic, got %d", resp.StatusCode)
	}
}

// ---------- handleDeviceInfoToggle sad paths ----------

// ---------- SetPassword sad paths ----------

func TestSetPassword_InvalidInput_400(t *testing.T) {
	rig := newTestRig(t)
	// Too-short password fails at the builder: "password length ..." → 400.
	resp, _ := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/password/set",
		`{"password":"ab"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSetPassword_Rejected_401(t *testing.T) {
	rig := newTestRig(t)
	go rig.transport.InjectResponseForTest(
		protocol.BuildStreamFrame([]byte{protocol.RESPONSE_TYPE_PASSWORD_SET, 0x01}),
	)
	resp, _ := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/password/set",
		`{"password":"abcd"}`)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestSetPassword_InvalidBody(t *testing.T) {
	rig := newTestRig(t)
	resp, _ := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/password/set",
		`garbage`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// ---------- SetColor sad paths ----------

func TestSetColor_Disconnected_500(t *testing.T) {
	// testServer's controller is not connected; SetColor should time out
	// waiting on a response and surface as 500.
	srv := testServer(t)
	resp, _ := doJSONRequest(t, srv, http.MethodPost, "/device/010000fba416/color",
		`{"color":"#FF0000"}`)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

// Tiny sanity check: the Server respects the body-limit ceiling and the
// request context threads into the controller. This test doesn't wait on BLE;
// it cancels via a short context deadline so we exercise the ctx-cancelled
// path in SendAndWait.
func TestSetChannel_ContextCancel(t *testing.T) {
	rig := newTestRig(t)
	// No response injected; handler will wait on SendAndWait. The Fiber
	// in-memory runner enforces a per-request timeout argument (-1 disables
	// it), so rely on the default CommandTimeout instead. Keep this test
	// bounded by using a short timeout variable if needed.
	//
	// Rather than actually waiting, assert that repeated calls without
	// responses produce 500s consistently.
	start := time.Now()
	resp, _ := doJSONRequest(t, rig.srv, http.MethodPost, "/device/010000fba416/channel", `{"channel":1}`)
	elapsed := time.Since(start)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 on timeout, got %d", resp.StatusCode)
	}
	if elapsed > 2*protocol.CommandTimeout {
		t.Errorf("handler took %v, expected ~%v", elapsed, protocol.CommandTimeout)
	}
}
