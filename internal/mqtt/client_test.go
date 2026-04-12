package mqtt

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/liskl/coolledux-controller/internal/config"
)

func TestNewClient(t *testing.T) {
	cfg := &config.MQTTConfig{
		Broker:            "tcp://localhost:1883",
		ClientID:          "test-client",
		TopicPrefix:       "coolledux",
		HADiscoveryPrefix: "homeassistant",
		Keepalive:         30 * time.Second,
	}
	handler := NewCommandHandler(nil, slog.Default())
	c := NewClient(cfg, "testdevice123", handler, slog.Default())

	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.IsConnected() {
		t.Error("new client should not be connected")
	}
}

func TestIsConnected_NewClient(t *testing.T) {
	cfg := &config.MQTTConfig{}
	c := NewClient(cfg, "dev1", nil, slog.Default())
	if c.IsConnected() {
		t.Error("IsConnected should return false for a new client")
	}
}

func TestAvailabilityTopic(t *testing.T) {
	cfg := &config.MQTTConfig{
		TopicPrefix: "myprefix",
	}
	c := NewClient(cfg, "device42", nil, slog.Default())

	got := c.availabilityTopic()
	want := "myprefix/device42/availability"
	if got != want {
		t.Errorf("availabilityTopic: expected %q, got %q", want, got)
	}
}

func TestAvailabilityTopic_DifferentPrefix(t *testing.T) {
	cfg := &config.MQTTConfig{
		TopicPrefix: "coolledux",
	}
	c := NewClient(cfg, "010000fba416", nil, slog.Default())

	got := c.availabilityTopic()
	want := "coolledux/010000fba416/availability"
	if got != want {
		t.Errorf("availabilityTopic: expected %q, got %q", want, got)
	}
}

func TestDisconnect_NotConnected(t *testing.T) {
	cfg := &config.MQTTConfig{}
	c := NewClient(cfg, "dev1", nil, slog.Default())

	// Disconnect on a client that was never connected should not panic.
	// mqttClient is nil, so Disconnect should return early.
	c.Disconnect()
}

func TestConnect_BadBroker(t *testing.T) {
	cfg := &config.MQTTConfig{
		Broker:    "tcp://localhost:19999",
		ClientID:  "test-bad-broker",
		Keepalive: 1 * time.Second,
	}
	c := NewClient(cfg, "dev1", nil, slog.Default())

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
	c := NewClient(cfg, "dev1", nil, slog.Default())

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
	c := NewClient(cfg, "dev1", nil, slog.Default())

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
	handler := NewCommandHandler(nil, slog.Default())
	c := NewClient(cfg, "device123", handler, slog.Default())

	if c.cfg != cfg {
		t.Error("cfg mismatch")
	}
	if c.deviceID != "device123" {
		t.Errorf("deviceID = %q, want %q", c.deviceID, "device123")
	}
	if c.handler != handler {
		t.Error("handler mismatch")
	}
	if c.connected {
		t.Error("should not be connected initially")
	}
	if c.mqttClient != nil {
		t.Error("mqttClient should be nil before Connect")
	}
}
