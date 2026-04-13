package text

import (
	"bytes"
	"testing"
)

func TestGlyphAIsNotBlank(t *testing.T) {
	g := Glyph('A')
	if len(g) != BytesPerGlyph {
		t.Fatalf("glyph len = %d, want %d", len(g), BytesPerGlyph)
	}
	trimmed := TrimRight(g)
	if len(trimmed) == 0 {
		t.Fatal("expected 'A' to have non-empty glyph")
	}
	if len(trimmed)%BytesPerColumn != 0 {
		t.Fatalf("trimmed len %d not a multiple of %d", len(trimmed), BytesPerColumn)
	}
}

func TestTrimRight(t *testing.T) {
	g := make([]byte, BytesPerGlyph)
	g[0], g[1] = 0xFF, 0xFF
	g[2], g[3] = 0x80, 0x01
	trimmed := TrimRight(g)
	if len(trimmed) != 4 {
		t.Fatalf("trimmed %d cols, want 2", len(trimmed)/BytesPerColumn)
	}
}

func TestTrimRightAllZero(t *testing.T) {
	g := make([]byte, BytesPerGlyph)
	if got := TrimRight(g); len(got) != 0 {
		t.Fatalf("all-zero glyph should trim to empty, got %d bytes", len(got))
	}
}

func TestRenderScrollSingleChar(t *testing.T) {
	data := Render("A", 0xFFFFFF, 96, 0, false)
	if len(data) < 3 {
		t.Fatalf("output too short: %d bytes", len(data))
	}
	if data[1] != 0x00 {
		t.Fatalf("char type = 0x%02X, want 0x00", data[1])
	}
	cols := int(data[0])
	if cols == 0 || cols > GlyphWidth {
		t.Fatalf("column count %d out of range", cols)
	}
	wantLen := 2 + cols*BytesPerRGB444Column
	if len(data) != wantLen {
		t.Fatalf("got %d bytes, want %d", len(data), wantLen)
	}
}

func TestRenderOnPixelHasColor(t *testing.T) {
	// Render 'A' in pure red. At least one pixel must encode as (R=F, G=0, B=0).
	out := Render("A", 0xFF0000, 96, 0, false)
	cols := int(out[0])
	pixels := out[2 : 2+cols*BytesPerRGB444Column]
	foundRed := false
	for i := 0; i+1 < len(pixels); i += 2 {
		if pixels[i] == 0x0F && pixels[i+1] == 0x00 {
			foundRed = true
			break
		}
	}
	if !foundRed {
		t.Fatal("rendered 'A' in red has no red pixel")
	}
}

func TestRenderOffPixelsAreBlack(t *testing.T) {
	out := Render("A", 0xFFFFFF, 96, 0, false)
	cols := int(out[0])
	pixels := out[2 : 2+cols*BytesPerRGB444Column]
	// First column has row 0 empty for 'A' (top pixel is always off in our font).
	if pixels[0] != 0x00 || pixels[1] != 0x00 {
		t.Fatalf("row 0 of 'A' col 0 should be black, got 0x%02X 0x%02X", pixels[0], pixels[1])
	}
}

func TestRenderScrollSpacingBetweenChars(t *testing.T) {
	noSpace := Render("AB", 0xFFFFFF, 96, 0, false)
	withSpace := Render("AB", 0xFFFFFF, 96, 2, false)
	// Two chars -> one inter-char gap. Extra bytes should be 2 cols × 32 bytes,
	// applied to the first char's payload (which grows by that amount).
	if len(withSpace) != len(noSpace)+2*BytesPerRGB444Column {
		t.Fatalf("spacing=2 should add %d bytes, got +%d", 2*BytesPerRGB444Column, len(withSpace)-len(noSpace))
	}
}

func TestRenderStaticCentersShortText(t *testing.T) {
	out := Render("A", 0xFFFFFF, 96, 0, true)
	cols := int(out[0])
	if cols <= GlyphWidth {
		t.Fatalf("centered char width %d did not include leading padding", cols)
	}
	leadCols := cols - glyphColumns('A')
	if leadCols <= 0 {
		t.Fatalf("expected positive lead padding, got %d", leadCols)
	}
	// Leading columns should be all zero pixels.
	lead := out[2 : 2+leadCols*BytesPerRGB444Column]
	if !bytes.Equal(lead, make([]byte, len(lead))) {
		t.Fatal("leading padding is not all zero")
	}
}

func TestRenderSpaceHasFixedWidth(t *testing.T) {
	out := Render(" A", 0xFFFFFF, 96, 0, false)
	if int(out[0]) != spaceColumns {
		t.Fatalf("space width = %d, want %d", out[0], spaceColumns)
	}
	if out[1] != 0x00 {
		t.Fatalf("space type = 0x%02X, want 0x00", out[1])
	}
	// Space pixels are all off.
	spaceEnd := 2 + spaceColumns*BytesPerRGB444Column
	if !bytes.Equal(out[2:spaceEnd], make([]byte, spaceColumns*BytesPerRGB444Column)) {
		t.Fatal("space glyph is not all zero pixels")
	}
}

func TestRenderEmptyString(t *testing.T) {
	if got := Render("", 0xFFFFFF, 96, 1, false); len(got) != 0 {
		t.Fatalf("empty string should render to zero bytes, got %d", len(got))
	}
}

func TestRenderReplacesRunesAboveBMP(t *testing.T) {
	out := Render(string(rune(0x1F600)), 0xFFFFFF, 96, 0, false)
	if len(out) == 0 {
		t.Fatal("supplementary-plane rune should render as U+FFFD fallback, got empty")
	}
}

func TestRenderPerRuneAlternatesColors(t *testing.T) {
	// Two chars with distinct colors; the first should contain red pixels,
	// the second should contain green pixels.
	runes := []rune{'A', 'B'}
	colors := []uint32{0xFF0000, 0x00FF00}
	out := RenderPerRune(runes, colors, 96, 0, false)

	off := 0
	// Character 1 payload
	cols1 := int(out[off])
	off += 2
	px1 := out[off : off+cols1*BytesPerRGB444Column]
	off += cols1 * BytesPerRGB444Column

	cols2 := int(out[off])
	off += 2
	px2 := out[off : off+cols2*BytesPerRGB444Column]

	hasRedOnly := false
	for i := 0; i+1 < len(px1); i += 2 {
		if px1[i] == 0x0F && px1[i+1] == 0x00 {
			hasRedOnly = true
			break
		}
	}
	if !hasRedOnly {
		t.Fatal("first char should have at least one pure-red pixel")
	}

	hasGreenOnly := false
	for i := 0; i+1 < len(px2); i += 2 {
		if px2[i] == 0x00 && px2[i+1] == 0xF0 {
			hasGreenOnly = true
			break
		}
	}
	if !hasGreenOnly {
		t.Fatal("second char should have at least one pure-green pixel")
	}
}

func glyphColumns(r rune) int {
	return len(TrimRight(append([]byte{}, Glyph(r)...))) / BytesPerColumn
}
