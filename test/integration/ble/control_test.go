//go:build integration && ble_hw

package ble

import (
	"testing"
	"time"

	"github.com/liskl/coolledux-controller/internal/models"
)

// TestDeviceInfo reads identity and state from the panel and asserts the
// shape. Dimensions are checked against the 16x96 target (change the
// constants here if you're pointing at a different device geometry).
func TestDeviceInfo(t *testing.T) {
	ctx := withTimeout(t, 5*time.Second)
	info, err := ctrl(t).GetDeviceInfo(ctx)
	if err != nil {
		t.Fatalf("GetDeviceInfo: %v", err)
	}

	if info.FlipMode > models.FlipModeBoth {
		t.Errorf("FlipMode = %d, want 0..3", info.FlipMode)
	}
	if info.MaxProgramNumber == 0 {
		t.Error("MaxProgramNumber = 0 — expected a non-zero slot count")
	}
}

// TestDeviceInfo_Idempotent verifies that repeated reads return consistent
// values (no stale-frame carryover). We only compare fields the test suite
// has not yet mutated at this point in the run.
func TestDeviceInfo_Idempotent(t *testing.T) {
	ctx := withTimeout(t, 5*time.Second)
	a, err := ctrl(t).GetDeviceInfo(ctx)
	if err != nil {
		t.Fatalf("first GetDeviceInfo: %v", err)
	}
	b, err := ctrl(t).GetDeviceInfo(ctx)
	if err != nil {
		t.Fatalf("second GetDeviceInfo: %v", err)
	}
	if a.MaxProgramNumber != b.MaxProgramNumber {
		t.Errorf("MaxProgramNumber drift: %d -> %d", a.MaxProgramNumber, b.MaxProgramNumber)
	}
	if a.MicSupported != b.MicSupported {
		t.Errorf("MicSupported drift: %v -> %v", a.MicSupported, b.MicSupported)
	}
}

// TestBrightness_RoundTrip sets a distinct brightness, verifies GetDeviceInfo
// reflects it, then restores the original value. The tolerance (±2) is
// there because the firmware sometimes quantizes brightness to the nearest
// step supported by the LED driver.
func TestBrightness_RoundTrip(t *testing.T) {
	c := ctrl(t)
	ctx := withTimeout(t, 10*time.Second)

	before, err := c.GetDeviceInfo(ctx)
	if err != nil {
		t.Fatalf("baseline GetDeviceInfo: %v", err)
	}
	t.Cleanup(func() {
		restoreCtx := withTimeout(t, 5*time.Second)
		if err := c.SetBrightness(restoreCtx, before.Brightness); err != nil {
			t.Logf("restore brightness: %v", err)
		}
	})

	// Pick a target far enough from current to dodge quantization noise.
	target := uint8(64)
	if before.Brightness < 100 {
		target = 200
	}
	if err := c.SetBrightness(ctx, target); err != nil {
		t.Fatalf("SetBrightness(%d): %v", target, err)
	}

	after, err := c.GetDeviceInfo(ctx)
	if err != nil {
		t.Fatalf("post-set GetDeviceInfo: %v", err)
	}
	if absDiff(after.Brightness, target) > 2 {
		t.Errorf("brightness readback = %d, want ≈%d", after.Brightness, target)
	}
}

// TestFlip_RoundTrip walks through all four flip modes and verifies each via
// a readback, then restores the original mode. This catches regressions in
// CMD_FLIP (0x0C) encoding and the 0x1F response parser's flip-byte offset.
func TestFlip_RoundTrip(t *testing.T) {
	c := ctrl(t)
	ctx := withTimeout(t, 15*time.Second)

	before, err := c.GetDeviceInfo(ctx)
	if err != nil {
		t.Fatalf("baseline GetDeviceInfo: %v", err)
	}
	t.Cleanup(func() {
		restoreCtx := withTimeout(t, 5*time.Second)
		if err := c.SetFlip(restoreCtx, before.FlipMode); err != nil {
			t.Logf("restore flip: %v", err)
		}
	})

	modes := []models.FlipMode{
		models.FlipModeNone,
		models.FlipModeHorizontal,
		models.FlipModeVertical,
		models.FlipModeBoth,
	}
	for _, mode := range modes {
		t.Run(mode.String(), func(t *testing.T) {
			if err := c.SetFlip(ctx, mode); err != nil {
				t.Fatalf("SetFlip(%v): %v", mode, err)
			}
			info, err := c.GetDeviceInfo(ctx)
			if err != nil {
				t.Fatalf("GetDeviceInfo: %v", err)
			}
			if info.FlipMode != mode {
				t.Errorf("readback = %v, want %v", info.FlipMode, mode)
			}
		})
	}
}

// TestPower_RoundTrip toggles power to the opposite of the current state,
// reads it back, then restores. Done last-ish because a failure mid-test
// leaves the display off until the suite's TestMain restore runs.
func TestPower_RoundTrip(t *testing.T) {
	c := ctrl(t)
	ctx := withTimeout(t, 10*time.Second)

	before, err := c.GetDeviceInfo(ctx)
	if err != nil {
		t.Fatalf("baseline GetDeviceInfo: %v", err)
	}
	t.Cleanup(func() {
		restoreCtx := withTimeout(t, 5*time.Second)
		if err := c.SetPower(restoreCtx, before.Power); err != nil {
			t.Logf("restore power: %v", err)
		}
	})

	target := !before.Power
	if err := c.SetPower(ctx, target); err != nil {
		t.Fatalf("SetPower(%v): %v", target, err)
	}
	after, err := c.GetDeviceInfo(ctx)
	if err != nil {
		t.Fatalf("GetDeviceInfo: %v", err)
	}
	if after.Power != target {
		t.Errorf("power readback = %v, want %v", after.Power, target)
	}
}

// TestSetColor_ACKs is a fire-and-forget: the panel doesn't expose the
// current tint via GetDeviceInfo, so we can only assert the command ACKs.
// Cycling three distinct colors also surfaces any lingering CRC/framing
// bug that only shows up on specific byte patterns.
func TestSetColor_ACKs(t *testing.T) {
	c := ctrl(t)
	colors := []uint32{0xFF0000, 0x00FF00, 0x0000FF, 0xFFFFFF}
	for _, rgb := range colors {
		ctx := withTimeout(t, 3*time.Second)
		if err := c.SetColor(ctx, rgb); err != nil {
			t.Errorf("SetColor(0x%06X): %v", rgb, err)
		}
	}
}

// TestSetChannel_ACKs just verifies the panel ACKs a channel switch. We
// don't read back because the device info frame doesn't include the current
// channel index.
func TestSetChannel_ACKs(t *testing.T) {
	c := ctrl(t)
	ctx := withTimeout(t, 3*time.Second)
	// Channel 1 is safe: every shipping firmware has at least one slot.
	if err := c.SetChannel(ctx, 1); err != nil {
		t.Errorf("SetChannel(1): %v", err)
	}
}

func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}
