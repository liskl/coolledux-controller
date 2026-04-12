package ble

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/liskl/coolledux-controller/internal/protocol"
	"tinygo.org/x/bluetooth"
)

// Client manages a BLE connection to the CoolLEDUX LED matrix device.
// It handles scanning, connecting, service/characteristic discovery,
// notification subscription, and chunked writes.
type Client struct {
	adapter        *bluetooth.Adapter
	device         bluetooth.Device
	char           bluetooth.DeviceCharacteristic
	mu             sync.Mutex
	connected      bool
	maxPayload     int
	onData         func([]byte) // notification callback
	sendOverride   func(ctx context.Context, data []byte) error // test stub
	lastDisconnect time.Time
	logger         *slog.Logger
}

// NewClient creates a new BLE client with the default adapter.
func NewClient(logger *slog.Logger) *Client {
	return &Client{
		adapter:    bluetooth.DefaultAdapter,
		maxPayload: protocol.DefaultPayload,
		logger:     logger,
	}
}

// Connect scans for the device with the given MAC address, connects to it,
// discovers the CoolLEDUX service and characteristic, and enables notifications.
//
// If a previous disconnect happened less than ReconnectDelay ago, Connect
// waits for the cooldown to elapse before proceeding.
func (c *Client) Connect(ctx context.Context, deviceMAC string) error {
	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		return fmt.Errorf("already connected")
	}

	// Enforce reconnect cooldown.
	if !c.lastDisconnect.IsZero() {
		elapsed := time.Since(c.lastDisconnect)
		if elapsed < protocol.ReconnectDelay {
			wait := protocol.ReconnectDelay - elapsed
			c.mu.Unlock()
			c.logger.Info("waiting for reconnect cooldown", "remaining", wait)
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return ctx.Err()
			}
			c.mu.Lock()
		}
	}
	c.mu.Unlock()

	c.logger.Info("enabling BLE adapter")
	if err := c.adapter.Enable(); err != nil {
		return fmt.Errorf("enabling adapter: %w", err)
	}

	// Parse the target MAC address.
	targetAddr, err := bluetooth.ParseMAC(deviceMAC)
	if err != nil {
		return fmt.Errorf("parsing MAC %q: %w", deviceMAC, err)
	}

	// Scan for the target device. We run the scan for a fixed window, collect
	// the result, then explicitly stop and wait for BlueZ to quiesce before
	// attempting to connect. Calling StopScan from inside the callback and
	// immediately connecting causes "le-connection-abort-by-local" on Linux.
	c.logger.Info("scanning for device", "mac", deviceMAC)

	var foundAddr bluetooth.Address
	var found bool

	scanDone := make(chan struct{})
	scanErr := make(chan error, 1)

	go func() {
		err := c.adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
			if result.Address == (bluetooth.Address{MACAddress: bluetooth.MACAddress{MAC: targetAddr}}) {
				foundAddr = result.Address
				found = true
				_ = adapter.StopScan()
			}
		})
		if err != nil && !found {
			scanErr <- fmt.Errorf("scanning: %w", err)
		}
		close(scanDone)
	}()

	// Wait for the scan goroutine to fully return (meaning StopScan completed
	// and the BlueZ D-Bus discovery session is closed).
	select {
	case <-scanDone:
		if !found {
			select {
			case err := <-scanErr:
				return err
			default:
				return fmt.Errorf("device %s not found", deviceMAC)
			}
		}
	case <-ctx.Done():
		_ = c.adapter.StopScan()
		return ctx.Err()
	}

	// Extra pause for BlueZ to finish cleaning up the discovery session.
	c.logger.Debug("scan complete, waiting before connect")
	select {
	case <-time.After(2 * time.Second):
	case <-ctx.Done():
		return ctx.Err()
	}

	// Retry connection up to 3 times. BLE connections on Linux/BlueZ are
	// unreliable, especially right after scanning or a recent disconnect.
	var device bluetooth.Device
	var connectErr error
	for attempt := 1; attempt <= 3; attempt++ {
		c.logger.Info("connecting to device", "address", foundAddr, "attempt", attempt)
		device, connectErr = c.adapter.Connect(foundAddr, bluetooth.ConnectionParams{})
		if connectErr == nil {
			break
		}
		c.logger.Warn("connection attempt failed", "attempt", attempt, "error", connectErr)
		if attempt < 3 {
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	if connectErr != nil {
		return fmt.Errorf("connecting after 3 attempts: %w", connectErr)
	}
	c.device = device

	// Discover the CoolLEDUX service.
	serviceUUID, err := bluetooth.ParseUUID("0000fff0-0000-1000-8000-00805f9b34fb")
	if err != nil {
		_ = device.Disconnect()
		return fmt.Errorf("parsing service UUID: %w", err)
	}

	services, err := device.DiscoverServices([]bluetooth.UUID{serviceUUID})
	if err != nil {
		_ = device.Disconnect()
		return fmt.Errorf("discovering services: %w", err)
	}
	if len(services) == 0 {
		_ = device.Disconnect()
		return fmt.Errorf("service 0000fff0 not found")
	}

	// Discover the characteristic.
	charUUID, err := bluetooth.ParseUUID("0000fff1-0000-1000-8000-00805f9b34fb")
	if err != nil {
		_ = device.Disconnect()
		return fmt.Errorf("parsing char UUID: %w", err)
	}

	chars, err := services[0].DiscoverCharacteristics([]bluetooth.UUID{charUUID})
	if err != nil {
		_ = device.Disconnect()
		return fmt.Errorf("discovering characteristics: %w", err)
	}
	if len(chars) == 0 {
		_ = device.Disconnect()
		return fmt.Errorf("characteristic 0000fff1 not found")
	}
	c.char = chars[0]

	// tinygo bluetooth on Linux doesn't expose the negotiated MTU.
	// The device reports MTU=23 (default BLE), giving a 20-byte payload.
	// Using larger writes causes data loss. Stick with the safe default.
	c.maxPayload = protocol.DefaultPayload

	// Enable notifications with a brief startup delay.
	select {
	case <-time.After(protocol.NotificationStartDelay):
	case <-ctx.Done():
		_ = device.Disconnect()
		return ctx.Err()
	}

	var notifyErr error
	for attempt := 0; attempt < protocol.MaxNotificationRetries; attempt++ {
		notifyErr = c.char.EnableNotifications(func(buf []byte) {
			c.mu.Lock()
			handler := c.onData
			c.mu.Unlock()
			if handler != nil {
				data := make([]byte, len(buf))
				copy(data, buf)
				handler(data)
			}
		})
		if notifyErr == nil {
			break
		}
		c.logger.Warn("notification enable failed, retrying", "attempt", attempt+1, "error", notifyErr)
		select {
		case <-time.After(protocol.RetryDelay):
		case <-ctx.Done():
			_ = device.Disconnect()
			return ctx.Err()
		}
	}
	if notifyErr != nil {
		_ = device.Disconnect()
		return fmt.Errorf("enabling notifications after %d retries: %w", protocol.MaxNotificationRetries, notifyErr)
	}

	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	c.logger.Info("connected", "mac", deviceMAC, "maxPayload", c.maxPayload)
	return nil
}

// Disconnect closes the BLE connection and records the disconnect time.
func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	c.logger.Info("disconnecting")
	err := c.device.Disconnect()
	c.connected = false
	c.lastDisconnect = time.Now()
	return err
}

// Send writes data to the BLE characteristic, splitting it into chunks of
// maxPayload bytes. Each chunk is written sequentially with WriteWithoutResponse.
func (c *Client) Send(ctx context.Context, data []byte) error {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return fmt.Errorf("not connected")
	}
	override := c.sendOverride
	maxPayload := c.maxPayload
	c.mu.Unlock()

	if override != nil {
		return override(ctx, data)
	}

	numChunks := (len(data) + maxPayload - 1) / maxPayload
	for offset := 0; offset < len(data); offset += maxPayload {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		end := offset + maxPayload
		if end > len(data) {
			end = len(data)
		}
		chunk := data[offset:end]

		if _, err := c.char.WriteWithoutResponse(chunk); err != nil {
			return fmt.Errorf("writing chunk at offset %d: %w", offset, err)
		}

		// The device needs time between MTU-sized writes, especially for
		// large stream-framed packets (program data chunks). Without this
		// delay the device drops or garbles data. Skip delay on last chunk.
		if numChunks > 1 && offset+maxPayload < len(data) {
			time.Sleep(50 * time.Millisecond)
		}
	}

	return nil
}

// SetNotificationHandler sets the callback invoked when the device sends
// data via BLE notifications.
func (c *Client) SetNotificationHandler(handler func([]byte)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onData = handler
}

// IsConnected returns whether the client is currently connected.
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// OverrideConnectedForTest allows tests to set the connected flag without
// establishing a real BLE connection. This function exists solely for test
// use and should never be called in production.
func (c *Client) OverrideConnectedForTest(connected bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = connected
}

// OverrideSendFuncForTest replaces the real BLE send with a test stub. Pass
// nil to restore the default behavior. Only for test use.
func (c *Client) OverrideSendFuncForTest(fn func(ctx context.Context, data []byte) error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sendOverride = fn
}

// Reconnect disconnects (if connected) and then reconnects to the device,
// respecting the reconnect cooldown.
func (c *Client) Reconnect(ctx context.Context, deviceMAC string) error {
	c.logger.Info("reconnecting", "mac", deviceMAC)
	if err := c.Disconnect(); err != nil {
		c.logger.Warn("disconnect error during reconnect", "error", err)
	}
	return c.Connect(ctx, deviceMAC)
}
