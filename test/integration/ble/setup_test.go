//go:build integration && ble_hw

package ble

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	bleclient "github.com/liskl/coolledux-controller/internal/ble"
	"github.com/liskl/coolledux-controller/internal/config"
	"github.com/liskl/coolledux-controller/internal/controller"
	"github.com/liskl/coolledux-controller/internal/models"
)

// harness holds the long-lived BLE connection shared by every test in the
// suite. Connecting is slow (a few seconds each time), so we open one link
// in TestMain, run all tests against it, and close it on teardown.
type harness struct {
	cfg        *config.Config
	client     *bleclient.Client
	transport  *bleclient.Transport
	ctrl       *controller.Controller
	initialDev *models.DeviceInfo // Baseline state captured at suite start.
}

var (
	h       *harness
	hMu     sync.Mutex
	testMAC string
)

// TestMain is the single point where we connect to real hardware. If
// COOLLEDUX_TEST_MAC is unset we skip the entire suite cleanly so CI without
// hardware still goes green. Teardown disconnects and tries to restore the
// panel's initial brightness / flip so repeated runs don't permanently alter
// state.
func TestMain(m *testing.M) {
	testMAC = strings.TrimSpace(os.Getenv("COOLLEDUX_TEST_MAC"))
	if testMAC == "" {
		// No hardware available — emit a clear note and exit success so the
		// Makefile target stays green in environments without a panel.
		_, _ = io.WriteString(os.Stderr, "COOLLEDUX_TEST_MAC unset — BLE integration tests skipped\n")
		os.Exit(0)
	}

	cfg, err := config.Load("")
	if err != nil {
		_, _ = io.WriteString(os.Stderr, "config load: "+err.Error()+"\n")
		os.Exit(1)
	}
	cfg.BLE.DeviceMAC = testMAC

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := bleclient.NewClient(logger)
	transport := bleclient.NewTransport(client, logger)
	ctrl := controller.New(client, transport, cfg, logger)

	// Generous connect budget: the adapter reset and scan can eat several
	// seconds on a cold start.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := ctrl.Connect(ctx); err != nil {
		_, _ = io.WriteString(os.Stderr, "connect: "+err.Error()+"\n")
		os.Exit(1)
	}

	// Baseline snapshot so tests can restore whatever they mutate.
	infoCtx, infoCancel := context.WithTimeout(context.Background(), 5*time.Second)
	initial, err := ctrl.GetDeviceInfo(infoCtx)
	infoCancel()
	if err != nil {
		_, _ = io.WriteString(os.Stderr, "initial GetDeviceInfo: "+err.Error()+"\n")
		_ = ctrl.Disconnect(context.Background())
		os.Exit(1)
	}

	h = &harness{
		cfg:        cfg,
		client:     client,
		transport:  transport,
		ctrl:       ctrl,
		initialDev: initial,
	}

	code := m.Run()

	// Best-effort restore of the pre-suite state. Failures here only warn —
	// we've already got our test results.
	restoreCtx, restoreCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := h.ctrl.SetBrightness(restoreCtx, initial.Brightness); err != nil {
		_, _ = io.WriteString(os.Stderr, "restore brightness: "+err.Error()+"\n")
	}
	if err := h.ctrl.SetFlip(restoreCtx, initial.FlipMode); err != nil {
		_, _ = io.WriteString(os.Stderr, "restore flip: "+err.Error()+"\n")
	}
	if err := h.ctrl.SetPower(restoreCtx, initial.Power); err != nil {
		_, _ = io.WriteString(os.Stderr, "restore power: "+err.Error()+"\n")
	}
	restoreCancel()

	_ = h.ctrl.Disconnect(context.Background())
	os.Exit(code)
}

// ctrl returns the shared controller. Wrapped in a getter so tests that race
// on setup still see a helpful nil-panic message instead of a nil deref.
func ctrl(t *testing.T) *controller.Controller {
	t.Helper()
	hMu.Lock()
	defer hMu.Unlock()
	if h == nil || h.ctrl == nil {
		t.Fatal("harness not initialized — TestMain must have bailed")
	}
	return h.ctrl
}

// withTimeout returns a context that auto-cancels at test teardown, so a
// stuck BLE command doesn't block the whole suite past the per-test budget.
func withTimeout(t *testing.T, d time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx
}
