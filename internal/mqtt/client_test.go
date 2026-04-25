package mqtt

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/liskl/coolledux-controller/internal/ble"
	"github.com/liskl/coolledux-controller/internal/config"
	"github.com/liskl/coolledux-controller/internal/controller"
	"github.com/liskl/coolledux-controller/internal/registry"
)

// testRegistry builds a minimal registry with one disconnected device so
// NewClient can loop over it. The controller is wired to a disconnected
// BLE client; tests that exercise command handlers expect the
// controller.* calls to return "not connected" errors.
func testRegistry(id, mac string) *registry.Registry {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bleClient := ble.NewClient(logger)
	transport := ble.NewTransport(bleClient, logger)
	cfg := &config.Config{
		BLE:     config.BLEConfig{DeviceMAC: mac},
		Display: config.DisplayConfig{Columns: 96, Rows: 16},
	}
	ctrl := controller.New(bleClient, transport, cfg, logger)

	r := registry.New()
	if err := r.Add(&registry.Entry{
		ID:         id,
		Name:       id,
		MAC:        mac,
		Client:     bleClient,
		Transport:  transport,
		Controller: ctrl,
	}); err != nil {
		panic(err)
	}
	return r
}

func TestNewClient(t *testing.T) {
	cfg := &config.MQTTConfig{
		Broker:            "tcp://localhost:1883",
		ClientID:          "test-client",
		TopicPrefix:       "coolledux",
		HADiscoveryPrefix: "homeassistant",
		Keepalive:         30 * time.Second,
	}
	reg := testRegistry("testdevice123", "01:00:00:FB:A4:16")
	c := NewClient(cfg, reg, slog.Default())

	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.IsConnected() {
		t.Error("new client should not be connected")
	}
	// One handler per registered device.
	if len(c.handlers) != 1 {
		t.Errorf("handlers len = %d, want 1", len(c.handlers))
	}
	if _, ok := c.handlers["testdevice123"]; !ok {
		t.Error("expected handler for testdevice123")
	}
}

func TestIsConnected_NewClient(t *testing.T) {
	cfg := &config.MQTTConfig{}
	reg := testRegistry("dev1", "01:00:00:FB:A4:16")
	c := NewClient(cfg, reg, slog.Default())
	if c.IsConnected() {
		t.Error("IsConnected should return false for a new client")
	}
}

func TestDeviceAvailabilityTopic(t *testing.T) {
	cfg := &config.MQTTConfig{TopicPrefix: "myprefix"}
	reg := testRegistry("device42", "01:00:00:FB:A4:16")
	c := NewClient(cfg, reg, slog.Default())

	got := c.deviceAvailabilityTopic("device42")
	want := "myprefix/device42/availability"
	if got != want {
		t.Errorf("deviceAvailabilityTopic: expected %q, got %q", want, got)
	}
}

func TestServiceAvailabilityTopic(t *testing.T) {
	cfg := &config.MQTTConfig{TopicPrefix: "coolledux"}
	reg := testRegistry("010000fba416", "01:00:00:FB:A4:16")
	c := NewClient(cfg, reg, slog.Default())

	got := c.serviceAvailabilityTopic()
	want := "coolledux/availability"
	if got != want {
		t.Errorf("serviceAvailabilityTopic: expected %q, got %q", want, got)
	}
}

func TestDisconnect_NotConnected(t *testing.T) {
	cfg := &config.MQTTConfig{}
	reg := testRegistry("dev1", "01:00:00:FB:A4:16")
	c := NewClient(cfg, reg, slog.Default())

	// Disconnect on a client that was never connected should not panic.
	c.Disconnect()
}

func TestConnect_BadBroker(t *testing.T) {
	cfg := &config.MQTTConfig{
		Broker:    "tcp://localhost:19999",
		ClientID:  "test-bad-broker",
		Keepalive: 1 * time.Second,
	}
	reg := testRegistry("dev1", "01:00:00:FB:A4:16")
	c := NewClient(cfg, reg, slog.Default())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := c.Connect(ctx)
	if err == nil {
		t.Fatal("expected error connecting to non-existent broker")
	}
	if !strings.Contains(err.Error(), "mqtt connect") && !strings.Contains(err.Error(), "context") {
		t.Errorf("expected mqtt connect or context error, got: %v", err)
	}
}

func TestConnect_ContextCancelled(t *testing.T) {
	cfg := &config.MQTTConfig{
		Broker:    "tcp://localhost:19999",
		ClientID:  "test-ctx-cancel",
		Keepalive: 1 * time.Second,
	}
	reg := testRegistry("dev1", "01:00:00:FB:A4:16")
	c := NewClient(cfg, reg, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := c.Connect(ctx)
	if err == nil {
		t.Fatal("expected error when context is already cancelled")
	}
}

func TestConnect_WithCredentials(t *testing.T) {
	cfg := &config.MQTTConfig{
		Broker:    "tcp://localhost:19999",
		ClientID:  "test-creds",
		Username:  "user",
		Password:  "pass",
		Keepalive: 1 * time.Second,
	}
	reg := testRegistry("dev1", "01:00:00:FB:A4:16")
	c := NewClient(cfg, reg, slog.Default())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := c.Connect(ctx)
	if err == nil {
		t.Fatal("expected error connecting to non-existent broker")
	}
	// Just verifying the credentials code path doesn't panic.
}

func TestNewClient_Fields(t *testing.T) {
	cfg := &config.MQTTConfig{
		Broker:            "tcp://test:1883",
		ClientID:          "my-client",
		TopicPrefix:       "prefix",
		HADiscoveryPrefix: "ha",
		Keepalive:         30 * time.Second,
	}
	reg := testRegistry("device123", "01:00:00:FB:A4:16")
	c := NewClient(cfg, reg, slog.Default())

	if c.cfg != cfg {
		t.Error("cfg mismatch")
	}
	if c.reg != reg {
		t.Error("registry mismatch")
	}
	if c.connected {
		t.Error("should not be connected initially")
	}
	if c.mqttClient != nil {
		t.Error("mqttClient should be nil before Connect")
	}
}

func TestNewClient_MultipleDevices(t *testing.T) {
	cfg := &config.MQTTConfig{TopicPrefix: "coolledux"}
	reg := testRegistry("a", "01:00:00:FB:A4:16")
	// Add a second device manually so we exercise the per-device handler map.
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bleClient := ble.NewClient(logger)
	transport := ble.NewTransport(bleClient, logger)
	ctrl := controller.New(bleClient, transport, &config.Config{
		BLE:     config.BLEConfig{DeviceMAC: "01:00:00:FB:A4:17"},
		Display: config.DisplayConfig{Columns: 96, Rows: 16},
	}, logger)
	if err := reg.Add(&registry.Entry{
		ID: "b", Name: "b", MAC: "01:00:00:FB:A4:17",
		Client: bleClient, Transport: transport, Controller: ctrl,
	}); err != nil {
		t.Fatalf("add b: %v", err)
	}

	c := NewClient(cfg, reg, logger)
	if len(c.handlers) != 2 {
		t.Fatalf("handlers len = %d, want 2", len(c.handlers))
	}
	if _, ok := c.handlers["a"]; !ok {
		t.Error("missing handler for a")
	}
	if _, ok := c.handlers["b"]; !ok {
		t.Error("missing handler for b")
	}
}

func TestSanitizeBroker(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantURL   string
		wantHost  string
		wantPort  string
	}{
		{"empty", "", "", "", ""},
		{"plain tcp host port", "tcp://host:1883",
			"tcp://host:1883", "host", "1883"},
		{"userinfo stripped", "tcp://user:pass@host:1883",
			"tcp://host:1883", "host", "1883"},
		{"only-user stripped", "tcp://user@host:1883",
			"tcp://host:1883", "host", "1883"},
		{"https with creds (HA-style add-on broker)", "wss://admin:secret@mqtt.example.com:8883/ws",
			"wss://mqtt.example.com:8883/ws", "mqtt.example.com", "8883"},
		{"no port", "tcp://host", "tcp://host", "host", ""},
		// Paho accepts some non-RFC URLs that url.Parse may not handle the
		// same way; verify the regex fallback at least strips userinfo.
		{"non-standard fallback strips creds",
			"mqtts+ssl://user:pass@host", "mqtts+ssl://host", "host", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, gotHost, gotPort := sanitizeBroker(tt.raw)
			if gotURL != tt.wantURL {
				t.Errorf("url = %q, want %q", gotURL, tt.wantURL)
			}
			if gotHost != tt.wantHost {
				t.Errorf("host = %q, want %q", gotHost, tt.wantHost)
			}
			if gotPort != tt.wantPort {
				t.Errorf("port = %q, want %q", gotPort, tt.wantPort)
			}
			// Hard contract: under no circumstances may the sanitized URL
			// contain "user:pass" or any colon-separated creds pattern.
			if strings.Contains(gotURL, "pass") || strings.Contains(gotURL, "secret") {
				t.Errorf("sanitized URL %q still contains credential-like substring", gotURL)
			}
		})
	}
}
