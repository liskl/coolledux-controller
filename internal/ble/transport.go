package ble

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/liskl/coolledux-controller/internal/protocol"
)

// Transport provides a higher-level send/receive layer on top of the BLE
// Client. It routes incoming BLE notifications into a response channel so
// callers can do request-response style communication with the device.
type Transport struct {
	client     *Client
	responseCh chan []byte
	mu         sync.Mutex
	logger     *slog.Logger
}

// NewTransport creates a Transport wired to the given Client.
// It installs a notification handler that forwards received data to the
// internal response channel.
func NewTransport(client *Client, logger *slog.Logger) *Transport {
	t := &Transport{
		client:     client,
		responseCh: make(chan []byte, 16),
		logger:     logger,
	}
	client.SetNotificationHandler(func(data []byte) {
		t.logger.Debug("notification received", "len", len(data))
		select {
		case t.responseCh <- data:
		default:
			t.logger.Warn("response channel full, dropping notification")
		}
	})
	return t
}

// SendCommand sends pre-framed data to the device without waiting for a response.
func (t *Transport) SendCommand(ctx context.Context, data []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.client.Send(ctx, data)
}

// SendAndWait sends pre-framed data and then blocks until a response arrives
// or the timeout elapses. If timeout is zero, the default CommandTimeout is used.
func (t *Transport) SendAndWait(ctx context.Context, data []byte, timeout time.Duration) ([]byte, error) {
	if timeout == 0 {
		timeout = protocol.CommandTimeout
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.client.Send(ctx, data); err != nil {
		return nil, fmt.Errorf("sending command: %w", err)
	}

	return t.waitForResponseUnlocked(ctx, timeout)
}

// WaitForResponse blocks until a response arrives on the notification channel
// or the timeout expires. If timeout is zero, the default ResponseTimeout is used.
func (t *Transport) WaitForResponse(ctx context.Context, timeout time.Duration) ([]byte, error) {
	if timeout == 0 {
		timeout = protocol.ResponseTimeout
	}
	return t.waitForResponseUnlocked(ctx, timeout)
}

// InjectResponseForTest pushes data into the response channel as if a
// notification was received. Only for test use.
func (t *Transport) InjectResponseForTest(data []byte) {
	t.responseCh <- data
}

// waitForResponseUnlocked is the internal wait implementation. Caller must
// manage their own locking if needed.
func (t *Transport) waitForResponseUnlocked(ctx context.Context, timeout time.Duration) ([]byte, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case resp := <-t.responseCh:
		return resp, nil
	case <-timer.C:
		return nil, fmt.Errorf("response timeout after %v", timeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
