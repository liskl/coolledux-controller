package controller

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/liskl/coolledux-controller/internal/protocol"
)

func TestBuildScoreboardBackgroundContent(t *testing.T) {
	content, err := buildScoreboardBackgroundContent()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content) < 24 {
		t.Fatalf("content too short: %d bytes", len(content))
	}

	totalLen := binary.BigEndian.Uint32(content[0:4])
	if int(totalLen) != len(content) {
		t.Errorf("totalLen field = %d, actual len = %d", totalLen, len(content))
	}
	if content[4] != 0x03 {
		t.Errorf("content type = %#x, want 0x03 (animation)", content[4])
	}
	if content[12] != 0x01 {
		t.Errorf("layerType = %#x, want 0x01", content[12])
	}
}

func TestBuildScoreboardContent96x16_Header(t *testing.T) {
	content := buildScoreboardContent96x16(0xFFFFFF)

	// Total-length prefix.
	totalLen := binary.BigEndian.Uint32(content[0:4])
	if int(totalLen) != len(content) {
		t.Errorf("totalLen field = %d, actual len = %d", totalLen, len(content))
	}

	// Content-type for scoreboard is 0x0b (per APK smali CoolledUXUtils:12685+).
	if content[4] != 0x0b {
		t.Errorf("content type = %#x, want 0x0b (scoreboard)", content[4])
	}

	// Bytes 5..11 = 7 reserved zeros.
	for i := 5; i < 12; i++ {
		if content[i] != 0x00 {
			t.Errorf("padding byte %d = %#x, want 0x00", i, content[i])
		}
	}

	// Byte 12 = layerType 0x01.
	if content[12] != 0x01 {
		t.Errorf("layerType = %#x, want 0x01", content[12])
	}

	// Byte 13 = 0x00 filler.
	if content[13] != 0x00 {
		t.Errorf("byte[13] = %#x, want 0x00", content[13])
	}

	// scoreNumHeight/Width at bytes 14-17 should be 10 and 7 for 16x96.
	if h := binary.BigEndian.Uint16(content[14:16]); h != 10 {
		t.Errorf("scoreNumHeight = %d, want 10", h)
	}
	if w := binary.BigEndian.Uint16(content[16:18]); w != 7 {
		t.Errorf("scoreNumWidth = %d, want 7", w)
	}

	// Score-digit bitmap length at bytes 18-19 should be 140 and match
	// timeCountDigits96x16 verbatim.
	if bl := binary.BigEndian.Uint16(content[18:20]); bl != uint16(len(timeCountDigits96x16)) {
		t.Errorf("score-digit bitmap length = %d, want %d", bl, len(timeCountDigits96x16))
	}
	start := 20
	end := start + len(timeCountDigits96x16)
	for i := range timeCountDigits96x16 {
		if content[start+i] != timeCountDigits96x16[i] {
			t.Errorf("score-digit bitmap byte %d diverges: got %d, want %d",
				i, content[start+i], timeCountDigits96x16[i])
			break
		}
	}
	// Sanity: data continues past the first bitmap.
	if len(content) <= end {
		t.Fatalf("content truncated at bitmap end (%d bytes)", len(content))
	}
}

func TestBuildScoreboardContent96x16_ColorTint(t *testing.T) {
	// The 2-byte RGB444 scoreHostColor lands directly after the 140-byte
	// score-digit bitmap. That bitmap starts at offset 20 (4 length + 1 type
	// + 7 pad + 1 layer + 1 pad + 2 numH + 2 numW + 2 len), so the color is
	// at offset 20 + len(timeCountDigits96x16).
	content := buildScoreboardContent96x16(0xFF0000)
	off := 20 + len(timeCountDigits96x16)
	r, gb := content[off], content[off+1]
	if r != 0x0F || gb != 0x00 {
		t.Errorf("scoreHostColor (red) = (%#x, %#x) at offset %d, want (0x0f, 0x00)", r, gb, off)
	}
}

func TestBuildScoreboardProgram96x16_Composite(t *testing.T) {
	payload := buildScoreboardProgram96x16(0xFFFFFF)

	if len(payload) < 10 {
		t.Fatalf("payload too short: %d", len(payload))
	}
	for i := 0; i < 8; i++ {
		if payload[i] != 0x00 {
			t.Errorf("wrapper[%d] = %#x, want 0x00", i, payload[i])
		}
	}
	if payload[8] != 0x02 {
		t.Errorf("contentCount = %d, want 2 (bg + scoreboard)", payload[8])
	}

	// First block content-type at offset 14 is 0x03 (animation).
	if payload[14] != 0x03 {
		t.Errorf("first content type = %#x, want 0x03 (animation)", payload[14])
	}

	// Second block content-type is 0x0b (scoreboard).
	firstLen := binary.BigEndian.Uint32(payload[10:14])
	secondOff := 10 + int(firstLen)
	if secondOff+5 > len(payload) {
		t.Fatalf("truncated composite: secondOff=%d, len=%d", secondOff, len(payload))
	}
	if payload[secondOff+4] != 0x0b {
		t.Errorf("second content type = %#x, want 0x0b (scoreboard)", payload[secondOff+4])
	}
}

func TestScoreboardDisplay_NotConnected(t *testing.T) {
	ctrl := testController()
	if err := ctrl.ScoreboardDisplay(context.Background(), 0xFFFFFF); err == nil {
		t.Fatal("expected error when not connected")
	}
}

func TestScoreboardDisplay_Connected(t *testing.T) {
	ctrl, transport, client := connectedController()
	defer client.OverrideConnectedForTest(false)
	markConnected(ctrl)

	// ScoreboardDisplay: sendProgram + SetScores + SetTime + StartStop(true).
	go func() {
		transport.InjectResponseForTest(fakeResponse(protocol.RESPONSE_TYPE_PROGRAM_START))
		for i := 0; i < 40; i++ {
			transport.InjectResponseForTest(fakeResponse(protocol.RESPONSE_TYPE_PROGRAM_DATA))
		}
	}()

	if err := ctrl.ScoreboardDisplay(context.Background(), 0x00FF00); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
