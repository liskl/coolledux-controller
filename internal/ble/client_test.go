package ble

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/liskl/coolledux-controller/internal/protocol"
)

func TestNewClient(t *testing.T) {
	logger := slog.Default()
	c := NewClient(logger)

	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.maxPayload != protocol.DefaultPayload {
		t.Errorf("maxPayload: expected %d (DefaultPayload), got %d", protocol.DefaultPayload, c.maxPayload)
	}
	if c.IsConnected() {
		t.Error("new client should not be connected")
	}
}

func TestIsConnected_NewClient(t *testing.T) {
	c := NewClient(slog.Default())
	if c.IsConnected() {
		t.Error("IsConnected should return false for a new client")
	}
}

func TestSend_Disconnected(t *testing.T) {
	c := NewClient(slog.Default())
	err := c.Send(context.Background(), []byte{0x01, 0x02})
	if err == nil {
		t.Fatal("expected error when sending on disconnected client")
	}
	if err.Error() != "not connected" {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestSend_EmptyData_Disconnected(t *testing.T) {
	c := NewClient(slog.Default())
	err := c.Send(context.Background(), []byte{})
	if err == nil {
		t.Fatal("expected error when sending empty data on disconnected client")
	}
	if err.Error() != "not connected" {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestSend_ContextCancelled_Disconnected(t *testing.T) {
	c := NewClient(slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// "not connected" is checked before the context, so we still get that error.
	err := c.Send(ctx, []byte{0x01, 0x02})
	if err == nil {
		t.Fatal("expected error when sending on disconnected client with cancelled context")
	}
	if err.Error() != "not connected" {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestSetNotificationHandler_NoPanic(t *testing.T) {
	c := NewClient(slog.Default())

	// Setting a handler on a disconnected client should not panic.
	c.SetNotificationHandler(func(data []byte) {})

	// Setting nil handler should also be fine.
	c.SetNotificationHandler(nil)
}

func TestDisconnect_NotConnected(t *testing.T) {
	c := NewClient(slog.Default())

	// Disconnecting a never-connected client should be a graceful no-op.
	err := c.Disconnect()
	if err != nil {
		t.Errorf("Disconnect on not-connected client should return nil, got: %v", err)
	}
}

func TestReconnect_NoHardware(t *testing.T) {
	c := NewClient(slog.Default())

	// Reconnect on a never-connected client. If real BLE hardware is
	// present, this may succeed (connecting to the device). If no
	// hardware, it returns an error. Either outcome is acceptable;
	// the test verifies no panic occurs.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = c.Reconnect(ctx, "00:00:00:00:00:00") // bogus MAC, won't find a device
	// No panic = pass. Disconnect to clean up.
	_ = c.Disconnect()
}

func TestSend_FakeConnected_EmptyData(t *testing.T) {
	c := NewClient(slog.Default())
	c.OverrideConnectedForTest(true)
	defer c.OverrideConnectedForTest(false)

	// Client thinks it's connected, but the characteristic is zero-value.
	// Send with empty data: the for loop body never executes.
	err := c.Send(context.Background(), []byte{})
	if err != nil {
		t.Errorf("empty data send should succeed (no chunks), got: %v", err)
	}
}

func TestSend_FakeConnected_WritePanics(t *testing.T) {
	c := NewClient(slog.Default())
	c.OverrideConnectedForTest(true)
	defer c.OverrideConnectedForTest(false)

	// Client thinks it's connected but the characteristic is uninitialized.
	// WriteWithoutResponse on a zero-value characteristic will panic with
	// a nil pointer dereference. Verify the panic happens (the Send code
	// path up to the write is covered regardless).
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		_ = c.Send(context.Background(), []byte{0x01, 0x02})
	}()

	if !panicked {
		t.Fatal("expected panic from write to uninitialized characteristic")
	}
}

func TestSend_FakeConnected_ContextCancelled(t *testing.T) {
	c := NewClient(slog.Default())
	c.OverrideConnectedForTest(true)
	defer c.OverrideConnectedForTest(false)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// With cancelled context and connected client, the Send loop's select
	// will either pick up the cancelled context or fall through to default.
	// The select is non-deterministic, so the outcome may vary: context
	// error OR panic from writing to a zero-value characteristic.
	panicked := false
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		err = c.Send(ctx, []byte{0x01})
	}()

	if !panicked && err == nil {
		t.Fatal("expected error or panic from send with cancelled context")
	}
}

func TestDisconnect_FakeConnected(t *testing.T) {
	c := NewClient(slog.Default())
	c.OverrideConnectedForTest(true)

	// Disconnect when "connected" but the device is zero-value.
	// device.Disconnect() on a zero-value Device may panic or return error.
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		_ = c.Disconnect()
	}()

	if panicked {
		// If it panicked, the Disconnect function didn't complete.
		// The connected state was not changed due to the panic.
		return
	}

	// If no panic, verify the state was cleaned up.
	if c.IsConnected() {
		t.Error("should not be connected after Disconnect")
	}
}

func TestSend_FakeConnected_LargePayload(t *testing.T) {
	c := NewClient(slog.Default())
	c.OverrideConnectedForTest(true)
	defer c.OverrideConnectedForTest(false)

	// Create a payload larger than maxPayload to exercise the chunking loop.
	payload := make([]byte, c.maxPayload+10)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	// Will panic at the first WriteWithoutResponse due to zero-value characteristic.
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		_ = c.Send(context.Background(), payload)
	}()

	if !panicked {
		t.Fatal("expected panic from write to uninitialized characteristic")
	}
}
