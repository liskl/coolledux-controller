package controller

import (
	"bytes"
	"context"
	"encoding/binary"
	"sync"
	"testing"

	"github.com/liskl/coolledux-controller/internal/protocol"
)

// ---------- Pure byte-builder tests for timecount.go ----------

func TestRGB444PackBitShift(t *testing.T) {
	tests := []struct {
		name   string
		rgb    uint32
		wantR  byte
		wantGB byte
	}{
		{"black", 0x000000, 0x00, 0x00},
		{"white", 0xFFFFFF, 0x0F, 0xFF},
		{"pure red", 0xFF0000, 0x0F, 0x00},
		{"pure green", 0x00FF00, 0x00, 0xF0},
		{"pure blue", 0x0000FF, 0x00, 0x0F},
		{"midgrey", 0x808080, 0x08, 0x88},
		{"high-bits-only", 0xF0F0F0, 0x0F, 0xFF},
		{"low-bits-dropped", 0x0F0F0F, 0x00, 0x00},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, gb := rgb444PackBitShift(tc.rgb)
			if r != tc.wantR || gb != tc.wantGB {
				t.Errorf("rgb444PackBitShift(%#06x) = (%#x, %#x), want (%#x, %#x)",
					tc.rgb, r, gb, tc.wantR, tc.wantGB)
			}
		})
	}
}

func TestBuildCountdownBackgroundContent(t *testing.T) {
	content, err := buildCountdownBackgroundContent()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content) < 24 {
		t.Fatalf("content too short: %d bytes", len(content))
	}

	// First 4 bytes = total length BE.
	totalLen := binary.BigEndian.Uint32(content[0:4])
	if int(totalLen) != len(content) {
		t.Errorf("totalLen field = %d, actual len = %d", totalLen, len(content))
	}

	// Byte 4 is content type 0x03 (animation).
	if content[4] != 0x03 {
		t.Errorf("content type = %#x, want 0x03", content[4])
	}

	// Byte 12 is layerType (must be 1).
	if content[12] != 0x01 {
		t.Errorf("layerType = %#x, want 0x01", content[12])
	}

	// Frame count at offset 22..24 should be non-zero.
	frameCount := binary.BigEndian.Uint16(content[22:24])
	if frameCount == 0 {
		t.Error("frameCount is 0, want > 0")
	}
}

func TestBuildTimeCountContent96x16With_Structure(t *testing.T) {
	digits := make([]byte, 140)
	for i := range digits {
		digits[i] = byte(i)
	}
	const color uint32 = 0xFF8800

	content := buildTimeCountContent96x16With(color, digits, timeCountModeCountDown)

	// totalLen field must match actual length.
	totalLen := binary.BigEndian.Uint32(content[0:4])
	if int(totalLen) != len(content) {
		t.Errorf("totalLen field = %d, actual len = %d", totalLen, len(content))
	}

	// Byte 4 = content type 0x0a (time-count).
	if content[4] != 0x0a {
		t.Errorf("content type = %#x, want 0x0a", content[4])
	}

	// Bytes 5..11 (7 bytes) must be zero padding.
	for i := 5; i < 12; i++ {
		if content[i] != 0x00 {
			t.Errorf("padding byte %d = %#x, want 0x00", i, content[i])
		}
	}

	// Byte 12 = layerType (0x01).
	if content[12] != 0x01 {
		t.Errorf("layerType = %#x, want 0x01", content[12])
	}

	// Byte 13 = timeCountMode (0x00).
	if content[13] != 0x00 {
		t.Errorf("timeCountMode = %#x, want 0x00", content[13])
	}

	// Bytes 14..16 = numHeight (10), 16..18 = numWidth (7).
	if got := binary.BigEndian.Uint16(content[14:16]); got != 10 {
		t.Errorf("numHeight = %d, want 10", got)
	}
	if got := binary.BigEndian.Uint16(content[16:18]); got != 7 {
		t.Errorf("numWidth = %d, want 7", got)
	}

	// Bytes 18..20 = digit bitmap length (should be len(digits)).
	bitmapLen := binary.BigEndian.Uint16(content[18:20])
	if int(bitmapLen) != len(digits) {
		t.Errorf("bitmap length = %d, want %d", bitmapLen, len(digits))
	}

	// Bytes 20..(20+bitmapLen) must equal the digit bitmap.
	if !bytes.Equal(content[20:20+len(digits)], digits) {
		t.Error("digit bitmap not copied verbatim")
	}

	// After the bitmap: positions + separator blobs.
	off := 20 + len(digits)
	r4, gb := rgb444PackBitShift(color)

	readPosition := func(label string, wantCol, wantRow, wantW, wantH int) {
		if content[off] != r4 || content[off+1] != gb {
			t.Errorf("%s: color bytes = (%#x, %#x), want (%#x, %#x)", label, content[off], content[off+1], r4, gb)
		}
		off += 2
		if got := int(binary.BigEndian.Uint16(content[off : off+2])); got != wantCol {
			t.Errorf("%s: col = %d, want %d", label, got, wantCol)
		}
		off += 2
		if got := int(binary.BigEndian.Uint16(content[off : off+2])); got != wantRow {
			t.Errorf("%s: row = %d, want %d", label, got, wantRow)
		}
		off += 2
		if got := int(binary.BigEndian.Uint16(content[off : off+2])); got != wantW {
			t.Errorf("%s: width = %d, want %d", label, got, wantW)
		}
		off += 2
		if got := int(binary.BigEndian.Uint16(content[off : off+2])); got != wantH {
			t.Errorf("%s: height = %d, want %d", label, got, wantH)
		}
		off += 2
	}

	readSeparator := func() {
		sepLen := binary.BigEndian.Uint16(content[off : off+2])
		off += 2
		if int(sepLen) != len(timeCountSeparator96x16) {
			t.Errorf("separator length = %d, want %d", sepLen, len(timeCountSeparator96x16))
		}
		if !bytes.Equal(content[off:off+int(sepLen)], timeCountSeparator96x16) {
			t.Error("separator bytes mismatch")
		}
		off += int(sepLen)
	}

	readPosition("hour", 11, 3, 14, 10)
	readPosition("spaceHour", 26, 3, 2, 10)
	readSeparator()
	readPosition("minute", 30, 3, 14, 10)
	readPosition("spaceMinute", 45, 3, 2, 10)
	readSeparator()
	readPosition("seconds", 49, 3, 14, 10)

	if off != len(content) {
		t.Errorf("trailing bytes: off=%d, len=%d", off, len(content))
	}
}

func TestBuildTimeCountProgram96x16With_WrapperShape(t *testing.T) {
	// Pass an atypical digit bitmap length to exercise the length plumbing.
	digits := make([]byte, 390)
	for i := range digits {
		digits[i] = 0xAB
	}
	payload := buildTimeCountProgram96x16With(0x00FF00, digits)

	// Wrapper layout: [0x00 x 8][contentCount:1][0x00][content...].
	if len(payload) < 10 {
		t.Fatalf("payload too short: %d", len(payload))
	}
	for i := 0; i < 8; i++ {
		if payload[i] != 0x00 {
			t.Errorf("wrapper[%d] = %#x, want 0x00", i, payload[i])
		}
	}
	if payload[8] != 0x01 {
		t.Errorf("contentCount = %d, want 1", payload[8])
	}
	if payload[9] != 0x00 {
		t.Errorf("separator = %#x, want 0x00", payload[9])
	}

	content := payload[10:]
	if content[4] != 0x0a {
		t.Errorf("content type = %#x, want 0x0a", content[4])
	}
	// The bitmap length field should equal 390.
	if got := binary.BigEndian.Uint16(content[18:20]); got != 390 {
		t.Errorf("bitmap length = %d, want 390", got)
	}
}

func TestBuildTimeCountProgram96x16_Composite(t *testing.T) {
	// Default path: should attempt to build a composite program (bg + timecount).
	// Since the embedded GIF asset exists, this should succeed and produce a
	// 2-content program.
	payload := buildTimeCountProgram96x16(0xFFFFFF)

	if len(payload) < 10 {
		t.Fatalf("payload too short: %d", len(payload))
	}
	for i := 0; i < 8; i++ {
		if payload[i] != 0x00 {
			t.Errorf("wrapper[%d] = %#x, want 0x00", i, payload[i])
		}
	}
	// Composite = 2 content blocks.
	if payload[8] != 0x02 {
		t.Errorf("contentCount = %d, want 2 (bg + timecount)", payload[8])
	}

	// First block starts at offset 10, expect content type 0x03 (animation).
	if payload[10+4] != 0x03 {
		t.Errorf("first content type = %#x, want 0x03 (animation)", payload[10+4])
	}

	// Second block starts after first block's totalLen.
	firstLen := binary.BigEndian.Uint32(payload[10:14])
	secondOff := 10 + int(firstLen)
	if secondOff+5 > len(payload) {
		t.Fatalf("truncated composite: secondOff=%d, len=%d", secondOff, len(payload))
	}
	if payload[secondOff+4] != 0x0a {
		t.Errorf("second content type = %#x, want 0x0a (timecount)", payload[secondOff+4])
	}
}

// ---------- wrapCompositeProgram ----------

func TestWrapCompositeProgram_TwoBlocks(t *testing.T) {
	a := []byte{0x00, 0x00, 0x00, 0x05, 0x02, 0xAA}          // 6 bytes, len field lies on purpose (not parsed)
	b := []byte{0x00, 0x00, 0x00, 0x04, 0x03, 0xBB, 0xCC}    // 7 bytes

	got := wrapCompositeProgram(a, b)

	want := make([]byte, 0, 10+len(a)+len(b))
	want = append(want, 0, 0, 0, 0, 0, 0, 0, 0) // 8 zero bytes
	want = append(want, 0x02)                   // contentCount = 2
	want = append(want, 0x00)                   // separator
	want = append(want, a...)
	want = append(want, b...)

	if !bytes.Equal(got, want) {
		t.Errorf("wrapCompositeProgram mismatch\n got: %x\nwant: %x", got, want)
	}
}

func TestWrapCompositeProgram_Empty(t *testing.T) {
	got := wrapCompositeProgram()
	if len(got) != 10 {
		t.Fatalf("len = %d, want 10", len(got))
	}
	for i := 0; i < 8; i++ {
		if got[i] != 0x00 {
			t.Errorf("byte %d = %#x, want 0x00", i, got[i])
		}
	}
	if got[8] != 0x00 {
		t.Errorf("contentCount = %d, want 0", got[8])
	}
	if got[9] != 0x00 {
		t.Errorf("separator = %#x, want 0x00", got[9])
	}
}

func TestWrapCompositeProgram_SingleBlock(t *testing.T) {
	block := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	got := wrapCompositeProgram(block)
	if len(got) != 10+len(block) {
		t.Fatalf("len = %d, want %d", len(got), 10+len(block))
	}
	if got[8] != 0x01 {
		t.Errorf("contentCount = %d, want 1", got[8])
	}
	if !bytes.Equal(got[10:], block) {
		t.Error("block not copied verbatim")
	}
}

// ---------- buildCustomColorTextProgram ----------

func TestBuildCustomColorTextProgram_Structure(t *testing.T) {
	widths := []byte{7, 7, 9}
	// Per character: two color bytes [0R, GB], so 3 chars -> 6 bytes.
	colors := []byte{0x0F, 0xF0, 0x0F, 0x0F, 0x00, 0xFF}

	payload := buildCustomColorTextProgram(96, 16, 1 /* scroll left */, 5, 2, widths, colors)

	// Wrapper
	if payload[8] != 0x01 {
		t.Errorf("contentCount = %d, want 1", payload[8])
	}
	content := payload[10:]

	totalLen := binary.BigEndian.Uint32(content[0:4])
	if int(totalLen) != len(content) {
		t.Errorf("totalLen = %d, actual len = %d", totalLen, len(content))
	}
	if content[4] != 0x06 {
		t.Errorf("content type = %#x, want 0x06", content[4])
	}
	// moveSpace at content[10:12] = 0
	if binary.BigEndian.Uint16(content[10:12]) != 0 {
		t.Error("moveSpace should be 0")
	}
	// start col/row at 12..16 = 0, 0
	if binary.BigEndian.Uint16(content[12:14]) != 0 {
		t.Error("startCol should be 0")
	}
	if binary.BigEndian.Uint16(content[14:16]) != 0 {
		t.Error("startRow should be 0")
	}
	// show width / height
	if got := binary.BigEndian.Uint16(content[16:18]); got != 96 {
		t.Errorf("showWidth = %d, want 96", got)
	}
	if got := binary.BigEndian.Uint16(content[18:20]); got != 16 {
		t.Errorf("showHeight = %d, want 16", got)
	}
	// mode / speed / stayTime
	if content[20] != 1 {
		t.Errorf("mode = %d, want 1", content[20])
	}
	if content[21] != 5 {
		t.Errorf("speed = %d, want 5", content[21])
	}
	if content[22] != 2 {
		t.Errorf("stayTime = %d, want 2", content[22])
	}
	// content[23] reserved zero
	if content[23] != 0x00 {
		t.Errorf("reserved byte = %#x, want 0", content[23])
	}
	// textNumber at 24..26 = len(widths) = 3
	if got := binary.BigEndian.Uint16(content[24:26]); got != 3 {
		t.Errorf("textNumber = %d, want 3", got)
	}
	// allTextWidth at 26..28 = sum(widths) = 23
	if got := binary.BigEndian.Uint16(content[26:28]); got != 23 {
		t.Errorf("allTextWidth = %d, want 23", got)
	}
	// widths + colors follow
	if !bytes.Equal(content[28:28+len(widths)], widths) {
		t.Error("widths not copied")
	}
	if !bytes.Equal(content[28+len(widths):28+len(widths)+len(colors)], colors) {
		t.Error("colors not copied")
	}
}

func TestBuildCustomColorTextProgram_Empty(t *testing.T) {
	payload := buildCustomColorTextProgram(96, 16, 0, 0, 0, nil, nil)
	content := payload[10:]
	// totalLen should still be 28 (24 + 4 layout bytes for textNumber/allTextWidth).
	totalLen := binary.BigEndian.Uint32(content[0:4])
	if totalLen != 28 {
		t.Errorf("totalLen = %d, want 28", totalLen)
	}
	if binary.BigEndian.Uint16(content[24:26]) != 0 {
		t.Error("textNumber should be 0")
	}
	if binary.BigEndian.Uint16(content[26:28]) != 0 {
		t.Error("allTextWidth should be 0")
	}
}

// ---------- Connected fire-and-forget control tests (use OverrideSendFuncForTest) ----------

// captureSends returns a pointer to a slice that gets appended to by a stubbed
// send func. Use for tests that assert on what bytes were written to BLE.
// It also flips the controller's state to StateConnected so methods that gate
// on IsConnected() (sendControl, CountdownDisplay, etc.) proceed.
func captureSends(ctrl *Controller) *[][]byte {
	client := ctrl.bleClient
	var mu sync.Mutex
	sent := &[][]byte{}
	client.OverrideSendFuncForTest(func(_ context.Context, data []byte) error {
		mu.Lock()
		defer mu.Unlock()
		cp := make([]byte, len(data))
		copy(cp, data)
		*sent = append(*sent, cp)
		return nil
	})
	ctrl.mu.Lock()
	ctrl.state = StateConnected
	ctrl.mu.Unlock()
	return sent
}

// markConnected flips the controller's state flag to StateConnected without
// touching the send stub set up by connectedController().
func markConnected(ctrl *Controller) {
	ctrl.mu.Lock()
	ctrl.state = StateConnected
	ctrl.mu.Unlock()
}

func TestSetChannel_NotConnected(t *testing.T) {
	ctrl := testController()
	// Not connected: SendAndWait will time out / error at the BLE layer.
	err := ctrl.SetChannel(context.Background(), 2)
	if err == nil {
		t.Fatal("expected error from SetChannel when not connected")
	}
}

func TestSetChannel_Connected(t *testing.T) {
	ctrl, transport, client := connectedController()
	defer client.OverrideConnectedForTest(false)

	sent := captureSends(ctrl)

	go func() { transport.InjectResponseForTest(fakeResponse(protocol.RESPONSE_TYPE_CHANNEL)) }()

	if err := ctrl.SetChannel(context.Background(), 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*sent) != 1 {
		t.Fatalf("expected 1 send, got %d", len(*sent))
	}
	want := protocol.BuildChannelCommand(3)
	if !bytes.Equal((*sent)[0], want) {
		t.Errorf("sent bytes mismatch\n got: %x\nwant: %x", (*sent)[0], want)
	}
}

func TestCountdownStatus_NotConnected(t *testing.T) {
	ctrl := testController()
	if err := ctrl.CountdownStatus(context.Background()); err == nil {
		t.Fatal("expected error when not connected")
	}
}

func TestCountdownStatus_Connected(t *testing.T) {
	ctrl, _, client := connectedController()
	defer client.OverrideConnectedForTest(false)

	sent := captureSends(ctrl)

	if err := ctrl.CountdownStatus(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := protocol.BuildCountdownStatusCommand()
	if len(*sent) != 1 || !bytes.Equal((*sent)[0], want) {
		t.Errorf("sent mismatch: got %x, want %x", *sent, want)
	}
}

func TestCountdownSet_Connected(t *testing.T) {
	ctrl, _, client := connectedController()
	defer client.OverrideConnectedForTest(false)

	sent := captureSends(ctrl)

	if err := ctrl.CountdownSet(context.Background(), 1, 30, 15); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := protocol.BuildCountdownSetCommand(1, 30, 15)
	if len(*sent) != 1 || !bytes.Equal((*sent)[0], want) {
		t.Errorf("sent mismatch: got %x, want %x", *sent, want)
	}
}

func TestCountdownStartStop_Connected(t *testing.T) {
	for _, start := range []bool{true, false} {
		start := start
		t.Run("", func(t *testing.T) {
			ctrl, _, client := connectedController()
			defer client.OverrideConnectedForTest(false)

			sent := captureSends(ctrl)

			if err := ctrl.CountdownStartStop(context.Background(), start); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := protocol.BuildCountdownStartStopCommand(start)
			if len(*sent) != 1 || !bytes.Equal((*sent)[0], want) {
				t.Errorf("sent mismatch: got %x, want %x", *sent, want)
			}
		})
	}
}

func TestStopwatchStatus_Connected(t *testing.T) {
	ctrl, _, client := connectedController()
	defer client.OverrideConnectedForTest(false)

	sent := captureSends(ctrl)

	if err := ctrl.StopwatchStatus(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*sent) != 1 || !bytes.Equal((*sent)[0], protocol.BuildStopwatchStatusCommand()) {
		t.Errorf("unexpected send: %x", *sent)
	}
}

func TestStopwatchReset_Connected(t *testing.T) {
	ctrl, _, client := connectedController()
	defer client.OverrideConnectedForTest(false)

	sent := captureSends(ctrl)

	if err := ctrl.StopwatchReset(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*sent) != 1 || !bytes.Equal((*sent)[0], protocol.BuildStopwatchResetCommand()) {
		t.Errorf("unexpected send: %x", *sent)
	}
}

func TestStopwatchStartStop_Connected(t *testing.T) {
	ctrl, _, client := connectedController()
	defer client.OverrideConnectedForTest(false)

	sent := captureSends(ctrl)

	if err := ctrl.StopwatchStartStop(context.Background(), true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*sent) != 1 || !bytes.Equal((*sent)[0], protocol.BuildStopwatchStartStopCommand(true)) {
		t.Errorf("unexpected send: %x", *sent)
	}
}

func TestStopwatch_NotConnected(t *testing.T) {
	ctrl := testController()
	if err := ctrl.StopwatchStatus(context.Background()); err == nil {
		t.Error("expected error")
	}
	if err := ctrl.StopwatchReset(context.Background()); err == nil {
		t.Error("expected error")
	}
	if err := ctrl.StopwatchStartStop(context.Background(), true); err == nil {
		t.Error("expected error")
	}
}

func TestScoreboard_Connected(t *testing.T) {
	ctrl, _, client := connectedController()
	defer client.OverrideConnectedForTest(false)

	sent := captureSends(ctrl)
	ctx := context.Background()

	if err := ctrl.ScoreboardStatus(ctx); err != nil {
		t.Fatalf("status: %v", err)
	}
	if err := ctrl.ScoreboardSetScores(ctx, 5, 7, 1, 2); err != nil {
		t.Fatalf("setscores: %v", err)
	}
	if err := ctrl.ScoreboardSetTime(ctx, 12, 30, true); err != nil {
		t.Fatalf("settime: %v", err)
	}
	if err := ctrl.ScoreboardStartStop(ctx, false); err != nil {
		t.Fatalf("startstop: %v", err)
	}

	if len(*sent) != 4 {
		t.Fatalf("expected 4 sends, got %d", len(*sent))
	}
	checks := [][]byte{
		protocol.BuildScoreboardStatusCommand(),
		protocol.BuildScoreboardSetScoresCommand(5, 7, 1, 2),
		protocol.BuildScoreboardSetTimeCommand(12, 30, true),
		protocol.BuildScoreboardStartStopCommand(false),
	}
	for i, want := range checks {
		if !bytes.Equal((*sent)[i], want) {
			t.Errorf("send %d mismatch\n got: %x\nwant: %x", i, (*sent)[i], want)
		}
	}
}

func TestScoreboard_NotConnected(t *testing.T) {
	ctrl := testController()
	ctx := context.Background()
	if err := ctrl.ScoreboardStatus(ctx); err == nil {
		t.Error("ScoreboardStatus: expected error")
	}
	if err := ctrl.ScoreboardSetScores(ctx, 1, 2, 0, 0); err == nil {
		t.Error("ScoreboardSetScores: expected error")
	}
	if err := ctrl.ScoreboardSetTime(ctx, 1, 2, false); err == nil {
		t.Error("ScoreboardSetTime: expected error")
	}
	if err := ctrl.ScoreboardStartStop(ctx, true); err == nil {
		t.Error("ScoreboardStartStop: expected error")
	}
}

// ---------- CountdownDisplay & CountdownProbe (program upload path) ----------

func TestCountdownDisplay_NotConnected(t *testing.T) {
	ctrl := testController()
	if err := ctrl.CountdownDisplay(context.Background(), 0, 5, 0, 0xFFFFFF); err == nil {
		t.Fatal("expected error when not connected")
	}
}

func TestCountdownDisplay_Connected(t *testing.T) {
	ctrl, transport, client := connectedController()
	defer client.OverrideConnectedForTest(false)
	markConnected(ctrl)

	// CountdownDisplay: sendProgram + CountdownSet + CountdownStartStop.
	// Inject plenty of PROGRAM_DATA ACKs for the program upload chunks.
	go func() {
		transport.InjectResponseForTest(fakeResponse(protocol.RESPONSE_TYPE_PROGRAM_START))
		for i := 0; i < 40; i++ {
			transport.InjectResponseForTest(fakeResponse(protocol.RESPONSE_TYPE_PROGRAM_DATA))
		}
	}()

	if err := ctrl.CountdownDisplay(context.Background(), 0, 5, 30, 0x00FF00); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCountdownProbe_NotConnected(t *testing.T) {
	ctrl := testController()
	probe := make([]byte, 39)
	if err := ctrl.CountdownProbe(context.Background(), probe, 0xFFFFFF); err == nil {
		t.Fatal("expected error when not connected")
	}
}

func TestCountdownProbe_WrongSize(t *testing.T) {
	ctrl, _, client := connectedController()
	defer client.OverrideConnectedForTest(false)
	markConnected(ctrl)

	// Wrong probe size: should error before any upload.
	if err := ctrl.CountdownProbe(context.Background(), []byte{0x01, 0x02}, 0xFFFFFF); err == nil {
		t.Fatal("expected error for wrong-size probe")
	}
}

func TestCountdownProbe_Connected(t *testing.T) {
	ctrl, transport, client := connectedController()
	defer client.OverrideConnectedForTest(false)
	markConnected(ctrl)

	probe := make([]byte, 39)
	for i := range probe {
		probe[i] = 0x5A
	}

	go func() {
		transport.InjectResponseForTest(fakeResponse(protocol.RESPONSE_TYPE_PROGRAM_START))
		for i := 0; i < 40; i++ {
			transport.InjectResponseForTest(fakeResponse(protocol.RESPONSE_TYPE_PROGRAM_DATA))
		}
	}()

	if err := ctrl.CountdownProbe(context.Background(), probe, 0x112233); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------- Stopwatch (mirrors countdown) ----------

func TestBuildStopwatchBackgroundContent(t *testing.T) {
	content, err := buildStopwatchBackgroundContent()
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

	frameCount := binary.BigEndian.Uint16(content[22:24])
	if frameCount == 0 {
		t.Error("frameCount is 0, want > 0")
	}
}

func TestBuildStopwatchProgram96x16_Composite(t *testing.T) {
	payload := buildStopwatchProgram96x16(0xFFFFFF)

	if len(payload) < 10 {
		t.Fatalf("payload too short: %d", len(payload))
	}
	for i := 0; i < 8; i++ {
		if payload[i] != 0x00 {
			t.Errorf("wrapper[%d] = %#x, want 0x00", i, payload[i])
		}
	}
	if payload[8] != 0x02 {
		t.Errorf("contentCount = %d, want 2 (bg + timecount)", payload[8])
	}

	if payload[10+4] != 0x03 {
		t.Errorf("first content type = %#x, want 0x03 (animation)", payload[10+4])
	}

	firstLen := binary.BigEndian.Uint32(payload[10:14])
	secondOff := 10 + int(firstLen)
	if secondOff+5 > len(payload) {
		t.Fatalf("truncated composite: secondOff=%d, len=%d", secondOff, len(payload))
	}
	if payload[secondOff+4] != 0x0a {
		t.Errorf("second content type = %#x, want 0x0a (timecount)", payload[secondOff+4])
	}
}

func TestBuildTimeCountContent_ModeByteDistinguishesOverlays(t *testing.T) {
	// Byte offset 13 of the time-count content block is timeCountMode.
	// Countdown uses 0, stopwatch uses 1 (APK default). Getting this wrong
	// was the reason the stopwatch digits stayed frozen at 00:00:00 on real
	// hardware even though 0x10 03 01 was being delivered correctly.
	digits := timeCountDigits96x16
	const modeOffset = 13

	countdown := buildTimeCountContent96x16With(0xFFFFFF, digits, timeCountModeCountDown)
	if countdown[modeOffset] != 0 {
		t.Errorf("countdown mode byte = %#x, want 0", countdown[modeOffset])
	}

	stopwatch := buildTimeCountContent96x16With(0xFFFFFF, digits, timeCountModeCountUp)
	if stopwatch[modeOffset] != 1 {
		t.Errorf("stopwatch mode byte = %#x, want 1", stopwatch[modeOffset])
	}
}

func TestBuildStopwatchProgram96x16_SharesTimeCountContent(t *testing.T) {
	// The APK uses an identical time-count layout for both overlays on 16x96,
	// differing only in the timeCountMode byte (0=countdown, 1=stopwatch).
	// The timecount half of the stopwatch program must byte-equal the
	// standalone stopwatch time-count content for the same color.
	sw := buildStopwatchProgram96x16(0x00FF00)
	timeCount := buildTimeCountContent96x16With(0x00FF00, timeCountDigits96x16, timeCountModeCountUp)

	firstLen := binary.BigEndian.Uint32(sw[10:14])
	secondOff := 10 + int(firstLen)
	got := sw[secondOff:]
	if !bytes.Equal(got, timeCount) {
		t.Errorf("stopwatch timecount block diverges from expected count-up layout")
	}
}

func TestStopwatchDisplay_NotConnected(t *testing.T) {
	ctrl := testController()
	if err := ctrl.StopwatchDisplay(context.Background(), 0xFFFFFF); err == nil {
		t.Fatal("expected error when not connected")
	}
}

func TestStopwatchDisplay_Connected(t *testing.T) {
	ctrl, transport, client := connectedController()
	defer client.OverrideConnectedForTest(false)
	markConnected(ctrl)

	// StopwatchDisplay: sendProgram + StopwatchReset + StopwatchStartStop(true).
	go func() {
		transport.InjectResponseForTest(fakeResponse(protocol.RESPONSE_TYPE_PROGRAM_START))
		for i := 0; i < 40; i++ {
			transport.InjectResponseForTest(fakeResponse(protocol.RESPONSE_TYPE_PROGRAM_DATA))
		}
	}()

	if err := ctrl.StopwatchDisplay(context.Background(), 0x00FF00); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
