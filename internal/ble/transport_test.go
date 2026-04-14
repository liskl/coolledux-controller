package ble

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestNewTransport(t *testing.T) {
	c := NewClient(slog.Default())
	tr := NewTransport(c, slog.Default())

	if tr == nil {
		t.Fatal("NewTransport returned nil")
	}
	if tr.client != c {
		t.Error("transport client should match the provided client")
	}
	if tr.responseCh == nil {
		t.Error("responseCh should be initialized")
	}
}

func TestSendCommand_Disconnected(t *testing.T) {
	c := NewClient(slog.Default())
	tr := NewTransport(c, slog.Default())

	err := tr.SendCommand(context.Background(), []byte{0xAA, 0xBB})
	if err == nil {
		t.Fatal("expected error from SendCommand on disconnected transport")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' in error, got: %v", err)
	}
}

func TestSendAndWait_Disconnected(t *testing.T) {
	c := NewClient(slog.Default())
	tr := NewTransport(c, slog.Default())

	resp, err := tr.SendAndWait(context.Background(), []byte{0x01}, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected error from SendAndWait on disconnected transport")
	}
	if resp != nil {
		t.Error("response should be nil on error")
	}
	// The error should come from client.Send ("not connected") before the wait.
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' in error, got: %v", err)
	}
}

func TestSendAndWait_DefaultTimeout(t *testing.T) {
	c := NewClient(slog.Default())
	tr := NewTransport(c, slog.Default())

	// timeout=0 should use the default CommandTimeout. The client is disconnected
	// so it will fail on Send before even reaching the timeout.
	resp, err := tr.SendAndWait(context.Background(), []byte{0x01}, 0)
	if err == nil {
		t.Fatal("expected error from SendAndWait with default timeout on disconnected transport")
	}
	if resp != nil {
		t.Error("response should be nil on error")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' in error, got: %v", err)
	}
}

func TestWaitForResponse_Timeout(t *testing.T) {
	c := NewClient(slog.Default())
	tr := NewTransport(c, slog.Default())

	// Use a very short timeout so the test completes fast.
	resp, err := tr.WaitForResponse(context.Background(), 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error from WaitForResponse")
	}
	if resp != nil {
		t.Error("response should be nil on timeout")
	}
	if !strings.Contains(err.Error(), "response timeout") {
		t.Errorf("expected 'response timeout' in error, got: %v", err)
	}
}

func TestWaitForResponse_DefaultTimeout(t *testing.T) {
	c := NewClient(slog.Default())
	tr := NewTransport(c, slog.Default())

	// timeout=0 should use the default ResponseTimeout. Cancel the context
	// quickly so we don't wait for the full default timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	resp, err := tr.WaitForResponse(ctx, 0)
	if err == nil {
		t.Fatal("expected error from WaitForResponse with default timeout")
	}
	if resp != nil {
		t.Error("response should be nil on context cancellation")
	}
	// Could be either context deadline exceeded or context canceled.
	if !strings.Contains(err.Error(), "context") {
		t.Errorf("expected context-related error, got: %v", err)
	}
}

func TestWaitForResponse_ContextCancelled(t *testing.T) {
	c := NewClient(slog.Default())
	tr := NewTransport(c, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	resp, err := tr.WaitForResponse(ctx, 5*time.Second)
	if err == nil {
		t.Fatal("expected context error from WaitForResponse")
	}
	if resp != nil {
		t.Error("response should be nil on context cancellation")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected 'context canceled' in error, got: %v", err)
	}
}

func TestWaitForResponse_DataAvailable(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := NewClient(logger)
	tr := NewTransport(client, logger)

	expected := []byte{0x01, 0x02, 0x03}
	go func() {
		tr.responseCh <- expected
	}()

	resp, err := tr.WaitForResponse(context.Background(), 1*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(resp, expected) {
		t.Errorf("got %v, want %v", resp, expected)
	}
}

func TestInjectResponseForTest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tr := NewTransport(NewClient(logger), logger)

	expected := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	tr.InjectResponseForTest(expected)

	resp, err := tr.WaitForResponse(context.Background(), 200*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForResponse after inject: %v", err)
	}
	if !bytes.Equal(resp, expected) {
		t.Errorf("injected response mismatch: got %v, want %v", resp, expected)
	}
}

func TestNewTransport_NotificationHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := NewClient(logger)
	tr := NewTransport(client, logger)

	data := []byte{0xAA, 0xBB}
	client.onData(data)

	select {
	case received := <-tr.responseCh:
		if !bytes.Equal(received, data) {
			t.Errorf("got %v, want %v", received, data)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("notification not forwarded to response channel")
	}
}
