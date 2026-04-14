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

// --- RenderMono ---

func TestRenderMonoHeaderAndBody(t *testing.T) {
	// "A" -> header[6] + [width, 0x00, width*2 bytes] body
	out := RenderMono("A", 96, 0, false)
	if len(out) < 6 {
		t.Fatalf("output too short: %d", len(out))
	}
	charCount := int(out[0])<<8 | int(out[1])
	totalCols := int(out[2])<<24 | int(out[3])<<16 | int(out[4])<<8 | int(out[5])
	if charCount != 1 {
		t.Errorf("charCount = %d, want 1", charCount)
	}
	wantCols := glyphColumns('A')
	if totalCols != wantCols {
		t.Errorf("totalCols = %d, want %d", totalCols, wantCols)
	}
	if int(out[6]) != wantCols {
		t.Errorf("per-char width byte = %d, want %d", out[6], wantCols)
	}
	if out[7] != 0x00 {
		t.Errorf("type byte = 0x%02X, want 0x00", out[7])
	}
	wantLen := 6 + 2 + wantCols*BytesPerColumn
	if len(out) != wantLen {
		t.Errorf("len = %d, want %d", len(out), wantLen)
	}
}

func TestRenderMonoEmptyString(t *testing.T) {
	out := RenderMono("", 96, 1, true)
	// Header-only output with zero char count / zero columns.
	if len(out) != 6 {
		t.Fatalf("len = %d, want 6 (header only)", len(out))
	}
	for _, b := range out {
		if b != 0 {
			t.Fatalf("empty RenderMono header not all zero: %v", out)
		}
	}
}

func TestRenderMonoCenteredPadsFirstChar(t *testing.T) {
	out := RenderMono("A", 96, 0, true)
	wantGlyphCols := glyphColumns('A')
	firstCharCols := int(out[6])
	if firstCharCols <= wantGlyphCols {
		t.Fatalf("centered first char cols = %d, want > %d (glyph width)", firstCharCols, wantGlyphCols)
	}
}

func TestRenderMonoSpacingBetweenChars(t *testing.T) {
	base := RenderMono("AB", 96, 0, false)
	spaced := RenderMono("AB", 96, 3, false)
	// 1 gap × 3 cols × 2 bytes/col = 6 bytes added to the first char payload.
	if len(spaced) != len(base)+3*BytesPerColumn {
		t.Fatalf("spacing=3 added %d bytes, want %d", len(spaced)-len(base), 3*BytesPerColumn)
	}
}

func TestRenderMonoNegativeSpacingClampedToZero(t *testing.T) {
	a := RenderMono("AB", 96, 0, false)
	b := RenderMono("AB", 96, -5, false)
	if !bytes.Equal(a, b) {
		t.Fatalf("negative spacing should clamp to zero")
	}
}

// --- Rasterize / RasterizeAt / RasterizeWithFace / RasterizeByName ---

func TestRasterizeOutputLength(t *testing.T) {
	const w, h = 96, 16
	out := Rasterize([]rune("hi"), 0xFFFFFF, w, h)
	if len(out) != w*h*2 {
		t.Fatalf("len = %d, want %d", len(out), w*h*2)
	}
}

func TestRasterizeEmptyStringAllBlack(t *testing.T) {
	const w, h = 32, 16
	out := Rasterize(nil, 0xFFFFFF, w, h)
	if len(out) != w*h*2 {
		t.Fatalf("len = %d, want %d", len(out), w*h*2)
	}
	if !bytes.Equal(out, make([]byte, len(out))) {
		t.Fatal("empty rasterize output must be all black")
	}
}

func TestRasterizeProducesLitPixels(t *testing.T) {
	// Rendering a visible character must turn on at least one pixel.
	out := Rasterize([]rune("A"), 0xFFFFFF, 32, 16)
	lit := false
	for i := 0; i+1 < len(out); i += 2 {
		if out[i] != 0 || out[i+1] != 0 {
			lit = true
			break
		}
	}
	if !lit {
		t.Fatal("'A' rasterized to all-black output")
	}
}

func TestRasterizeColorApplied(t *testing.T) {
	// Pure red: RGB444Transfer(0xFF)=15, others=0 -> byte pair (0x0F, 0x00).
	out := Rasterize([]rune("A"), 0xFF0000, 32, 16)
	foundRed := false
	for i := 0; i+1 < len(out); i += 2 {
		if out[i] == 0x0F && out[i+1] == 0x00 {
			foundRed = true
			break
		}
	}
	if !foundRed {
		t.Fatal("no pure-red pixel in red rasterize output")
	}
}

func TestRasterizeAtLeftAligned(t *testing.T) {
	// Left-aligned: a lit pixel must appear in the first few columns,
	// whereas centered rasterize of the same short string leaves the
	// left margin blank.
	const w, h = 96, 16
	left := RasterizeAt([]rune("A"), 0xFFFFFF, w, h)
	center := Rasterize([]rune("A"), 0xFFFFFF, w, h)

	firstColLitLeft := false
	for row := 0; row < h; row++ {
		idx := 0*h*2 + row*2
		if left[idx] != 0 || left[idx+1] != 0 {
			firstColLitLeft = true
			break
		}
	}
	if !firstColLitLeft {
		t.Fatal("left-align: expected lit pixel in column 0")
	}

	firstColLitCenter := false
	for row := 0; row < h; row++ {
		idx := 0*h*2 + row*2
		if center[idx] != 0 || center[idx+1] != 0 {
			firstColLitCenter = true
			break
		}
	}
	if firstColLitCenter {
		t.Fatal("centered short text should not paint column 0")
	}
}

func TestRasterizeWithFaceRightAlign(t *testing.T) {
	const w, h = 96, 16
	face, err := Face(DefaultFontName)
	if err != nil {
		t.Fatal(err)
	}
	out := RasterizeWithFace([]rune("A"), 0xFFFFFF, face, w, h, AlignRight)
	// Rightmost columns should contain lit pixels.
	litInRightHalf := false
	for col := w - 8; col < w; col++ {
		for row := 0; row < h; row++ {
			idx := col*h*2 + row*2
			if out[idx] != 0 || out[idx+1] != 0 {
				litInRightHalf = true
			}
		}
	}
	if !litInRightHalf {
		t.Fatal("right-align: expected lit pixels near right edge")
	}
	// Leftmost columns should be blank.
	for col := 0; col < 8; col++ {
		for row := 0; row < h; row++ {
			idx := col*h*2 + row*2
			if out[idx] != 0 || out[idx+1] != 0 {
				t.Fatalf("right-align: col %d row %d unexpectedly lit", col, row)
			}
		}
	}
}

func TestRasterizeByNameKnownFonts(t *testing.T) {
	for _, name := range []string{"7x13", "7x14b", "8x16", ""} {
		out, err := RasterizeByName([]rune("Hi"), 0xFFFFFF, name, 96, 16, AlignLeft)
		if err != nil {
			t.Fatalf("RasterizeByName(%q) err = %v", name, err)
		}
		if len(out) != 96*16*2 {
			t.Fatalf("RasterizeByName(%q) len = %d, want %d", name, len(out), 96*16*2)
		}
		// Must have at least one lit pixel.
		lit := false
		for i := 0; i+1 < len(out); i += 2 {
			if out[i] != 0 || out[i+1] != 0 {
				lit = true
				break
			}
		}
		if !lit {
			t.Fatalf("RasterizeByName(%q) produced all-black output", name)
		}
	}
}

func TestRasterizeByNameUnknownReturnsError(t *testing.T) {
	_, err := RasterizeByName([]rune("x"), 0xFFFFFF, "no-such", 32, 16, AlignLeft)
	if err == nil {
		t.Fatal("expected error for unknown font name")
	}
}

func TestRasterizeStringTooWideDoesNotOverflow(t *testing.T) {
	// String wider than canvas: code path skips alignment adjustment;
	// must still return correctly sized buffer.
	const w, h = 16, 16
	out := Rasterize([]rune("ABCDEFGHIJKL"), 0xFFFFFF, w, h)
	if len(out) != w*h*2 {
		t.Fatalf("len = %d, want %d", len(out), w*h*2)
	}
}
