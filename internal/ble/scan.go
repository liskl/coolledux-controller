package ble

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

// ScanResult describes one CoolLEDUX advertiser observed during a scan.
type ScanResult struct {
	MAC  string // Canonical MAC in colon-separated form, e.g. "01:00:00:FB:A4:16".
	Name string // Local name from the advertisement, e.g. "CoolLEDUX".
	RSSI int16
}

// Scan runs a BLE discovery for the given timeout and returns every
// advertiser whose LocalName starts with namePrefix. Results are
// deduplicated by MAC (strongest RSSI wins). The adapter is Enable-d if
// needed but never disabled here — callers that share an adapter with
// Client should keep it up.
//
// This is a package-level function because it does not belong to any
// specific Client: the service may run a scan before it has decided
// which devices to connect to.
func Scan(ctx context.Context, namePrefix string, timeout time.Duration, logger *slog.Logger) ([]ScanResult, error) {
	adapter := bluetooth.DefaultAdapter
	if err := adapter.Enable(); err != nil {
		return nil, fmt.Errorf("enabling adapter: %w", err)
	}

	var (
		mu      sync.Mutex
		seen    = make(map[string]ScanResult)
		scanErr error
		done    = make(chan struct{})
	)

	go func() {
		defer close(done)
		err := adapter.Scan(func(_ *bluetooth.Adapter, result bluetooth.ScanResult) {
			name := result.LocalName()
			if namePrefix != "" && !strings.HasPrefix(name, namePrefix) {
				return
			}
			mac := result.Address.String()
			mu.Lock()
			prev, exists := seen[mac]
			// Keep the stronger signal's metadata on dedupe.
			if !exists || result.RSSI > prev.RSSI {
				seen[mac] = ScanResult{MAC: mac, Name: name, RSSI: result.RSSI}
			}
			mu.Unlock()
		})
		// adapter.Scan blocks until StopScan is called. err here is the
		// non-cancellation error path.
		if err != nil {
			scanErr = fmt.Errorf("ble scan: %w", err)
		}
	}()

	// Stop after either the timeout or context cancellation.
	select {
	case <-time.After(timeout):
	case <-ctx.Done():
	}
	_ = adapter.StopScan()

	// Wait for the goroutine to drain so we know seen is no longer written.
	<-done

	if scanErr != nil && len(seen) == 0 {
		return nil, scanErr
	}

	out := make([]ScanResult, 0, len(seen))
	for _, r := range seen {
		out = append(out, r)
	}
	if logger != nil {
		logger.Info("ble scan complete", "prefix", namePrefix, "found", len(out))
	}
	return out, nil
}
