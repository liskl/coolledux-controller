package protocol

import (
	"bytes"
	"testing"
)

// expectedPaletteLen captures the byte count we traced from the APK smali
// for each mode's palette. A regression here means the APK string constants
// at the top of colormode.go drifted away from CoolledUXUtils.java:34-63.
var expectedPaletteLen = map[int]int{
	1: 180, 2: 180, 5: 180, 6: 180,
	7: 24, 8: 24,
	9: 36, 10: 36, 11: 36, 12: 36, 15: 36, 16: 36,
	13: 12, 14: 4,
	17: 60, 18: 60,
	19: 16, 20: 16, 21: 16, 22: 16, 23: 16, 24: 16,
	25: 16, 26: 16, 27: 16, 28: 16,
	29: 96, 30: 96,
	31: 12,
}

// innerOf strips the stream framing and returns the raw [CMD ...] body.
// Uses ParseStreamFrame so any 0x01/0x02/0x03 escape sequences in the
// middle of the packet are unwound back to their original bytes.
func innerOf(t *testing.T, frame []byte) []byte {
	t.Helper()
	inner, err := ParseStreamFrame(frame)
	if err != nil {
		t.Fatalf("ParseStreamFrame: %v\nframe=%x", err, frame)
	}
	return inner
}

func TestColorModeTable_PaletteLengths(t *testing.T) {
	for mode, want := range expectedPaletteLen {
		spec, ok := colorModeTable[mode]
		if !ok {
			t.Errorf("mode %d missing from table", mode)
			continue
		}
		if got := len(spec.palette); got != want {
			t.Errorf("mode %d palette length = %d, want %d", mode, got, want)
		}
	}
}

func TestColorModeIDs_ExcludesThreeAndFour(t *testing.T) {
	for _, id := range ColorModeIDs() {
		if id == 3 || id == 4 {
			t.Errorf("mode %d should be excluded (APK no-op)", id)
		}
	}
	if len(ColorModeIDs()) != 29 {
		t.Errorf("len(ColorModeIDs) = %d, want 29", len(ColorModeIDs()))
	}
}

func TestColorModeIDs_Sorted(t *testing.T) {
	ids := ColorModeIDs()
	for i := 1; i < len(ids); i++ {
		if ids[i-1] >= ids[i] {
			t.Fatalf("ColorModeIDs not sorted: %v", ids)
		}
	}
}

func TestBuildColorSpeedCommand(t *testing.T) {
	inner := innerOf(t, BuildColorSpeedCommand(7))
	want := []byte{CMD_COLOR, COLOR_SUBTYPE_SPEED, 7}
	if !bytes.Equal(inner, want) {
		t.Errorf("got  %x\nwant %x", inner, want)
	}
}

func TestBuildColorModeCommand_Mode1(t *testing.T) {
	// Mode 1: i3=2, i4=0, i2=90, palette = colorMode1 (180 bytes).
	// Inner = [0x13, 0x03, 0x02, 0x00, 0x5A, <180 palette bytes>] = 185 bytes.
	frame, err := BuildColorModeCommand(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inner := innerOf(t, frame)

	if len(inner) != 185 {
		t.Fatalf("inner length = %d, want 185", len(inner))
	}
	if inner[0] != CMD_COLOR || inner[1] != COLOR_SUBTYPE_MODE {
		t.Errorf("header = %x %x, want %x %x", inner[0], inner[1], CMD_COLOR, COLOR_SUBTYPE_MODE)
	}
	if inner[2] != 2 || inner[3] != 0 || inner[4] != 0x5A {
		t.Errorf("i3,i4,i2 = %d,%d,%d, want 2,0,90", inner[2], inner[3], inner[4])
	}
	wantPrefix := []byte{0x0F, 0x00, 0x0F, 0x10, 0x0F, 0x20}
	if !bytes.Equal(inner[5:5+len(wantPrefix)], wantPrefix) {
		t.Errorf("palette prefix = %x, want %x", inner[5:5+len(wantPrefix)], wantPrefix)
	}
}

func TestBuildColorModeCommand_Mode13_OmitsI4(t *testing.T) {
	// Mode 13: i3=1, i4=-1 (omitted), i2=6, palette = colorMode13 (12 bytes).
	// Inner WITHOUT i4 = [0x13, 0x03, 0x01, 0x06, <12 palette bytes>] = 16 bytes.
	// This is the signature test that guards against a builder bug where
	// i4 is always emitted.
	frame, err := BuildColorModeCommand(13)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inner := innerOf(t, frame)

	want := []byte{
		CMD_COLOR, COLOR_SUBTYPE_MODE, 0x01, 0x06,
		0x0F, 0x00, 0x00, 0xF0, 0x00, 0x0F, 0x0F, 0xF0,
		0x00, 0xFF, 0x0F, 0x0F,
	}
	if !bytes.Equal(inner, want) {
		t.Errorf("mode 13 inner payload:\ngot  %x\nwant %x", inner, want)
	}
}

func TestBuildColorModeCommand_Mode14_SmallestPayload(t *testing.T) {
	// Mode 14: i3=1, i4=-1, i2=2, palette = "0F,00,00,0F" (4 bytes).
	// Smallest 0x13/0x03 packet: 8 inner bytes.
	frame, err := BuildColorModeCommand(14)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inner := innerOf(t, frame)

	want := []byte{
		CMD_COLOR, COLOR_SUBTYPE_MODE, 0x01, 0x02,
		0x0F, 0x00, 0x00, 0x0F,
	}
	if !bytes.Equal(inner, want) {
		t.Errorf("mode 14 inner payload:\ngot  %x\nwant %x", inner, want)
	}
}

func TestBuildColorModeCommand_Mode20_EvenIndexI4One(t *testing.T) {
	// Pairs with mode 19: same i2=8, same 16-byte palette shape, but i4=1
	// instead of 0. Guards the "even modes get i4=1" pattern from the
	// smali :goto_9f/:goto_95 split on modes 19..28.
	frame, err := BuildColorModeCommand(20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inner := innerOf(t, frame)

	if len(inner) != 5+16 {
		t.Fatalf("inner length = %d, want %d", len(inner), 5+16)
	}
	if inner[2] != 2 || inner[3] != 1 || inner[4] != 8 {
		t.Errorf("i3,i4,i2 = %d,%d,%d, want 2,1,8", inner[2], inner[3], inner[4])
	}
	wantPrefix := []byte{0x01, 0x00, 0x03, 0x00}
	if !bytes.Equal(inner[5:5+len(wantPrefix)], wantPrefix) {
		t.Errorf("palette prefix = %x, want %x", inner[5:5+len(wantPrefix)], wantPrefix)
	}
}

func TestBuildColorModeCommand_Mode31_UsesMode13Palette(t *testing.T) {
	// Mode 31 reuses colorMode13's 12-byte palette but with i3=4 and no i4.
	// Confirms the smali :goto_65 path (move v1, v12) where v12 was loaded
	// with the colorMode13 literal.
	frame31, err := BuildColorModeCommand(31)
	if err != nil {
		t.Fatalf("mode 31: %v", err)
	}
	frame13, err := BuildColorModeCommand(13)
	if err != nil {
		t.Fatalf("mode 13: %v", err)
	}
	inner31 := innerOf(t, frame31)
	inner13 := innerOf(t, frame13)

	// Same palette tail (last 12 bytes), different i3.
	tail31 := inner31[len(inner31)-12:]
	tail13 := inner13[len(inner13)-12:]
	if !bytes.Equal(tail31, tail13) {
		t.Errorf("palette differs: mode31=%x mode13=%x", tail31, tail13)
	}
	if inner31[2] != 4 {
		t.Errorf("mode 31 i3 = %d, want 4", inner31[2])
	}
	if inner13[2] != 1 {
		t.Errorf("mode 13 i3 = %d, want 1", inner13[2])
	}
}

func TestBuildColorModeCommand_RejectsInvalid(t *testing.T) {
	for _, mode := range []int{0, 3, 4, 32, 100, -1} {
		if _, err := BuildColorModeCommand(mode); err == nil {
			t.Errorf("mode %d: expected error, got nil", mode)
		}
	}
}

func TestParseHexList_DropsEmptyTokens(t *testing.T) {
	// The colorMode21 quirk: APK declaration has a trailing comma that
	// would otherwise produce a phantom byte.
	got, err := parseHexList("0F,00,0A,")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []byte{0x0F, 0x00, 0x0A}
	if !bytes.Equal(got, want) {
		t.Errorf("got %x, want %x", got, want)
	}
}

func TestParseHexList_TrimsWhitespace(t *testing.T) {
	got, err := parseHexList("0F,00,\n0F,10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []byte{0x0F, 0x00, 0x0F, 0x10}
	if !bytes.Equal(got, want) {
		t.Errorf("got %x, want %x", got, want)
	}
}

func TestParseHexList_RejectsBadToken(t *testing.T) {
	if _, err := parseHexList("0F,ZZ"); err == nil {
		t.Error("expected error on bad hex, got nil")
	}
	if _, err := parseHexList("0F,000"); err == nil {
		t.Error("expected error on 3-char token, got nil")
	}
}

func TestMustParseHexList_PanicsOnBadInput(t *testing.T) {
	// mustParseHexList is used to build the palette table at init time, so
	// any bad constant is a bug the program should crash on. Verify the
	// panic branch fires when it should — a bad token here would ship
	// unnoticed otherwise.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on malformed input, got none")
		}
	}()
	_ = mustParseHexList("ZZ")
}

func TestMustParseHexList_GoodInput(t *testing.T) {
	got := mustParseHexList("0F,FF,00")
	want := []byte{0x0F, 0xFF, 0x00}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("byte[%d] = 0x%02X, want 0x%02X", i, got[i], want[i])
		}
	}
}
