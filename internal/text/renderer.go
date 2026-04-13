package text

import (
	"image"
	"image/color"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	ledimage "github.com/liskl/coolledux-controller/internal/image"
)

// spaceColumns is the visual width, in columns, used for U+0020 since the
// font's blank space glyph trims to zero width.
const spaceColumns = 4

// MonoFace is the bitmap monospace face used for rasterizing text onto the
// matrix. Face7x13 is a Plan 9 font: 6-pixel advance, 13-pixel line height,
// pure 1-bit glyphs (no antialiasing), which is exactly what we want -- any
// grayscale AA pixels would be lit as dim-on pixels in RGB444 and make the
// output look blurry.
var MonoFace = basicfont.Face7x13

// BytesPerRGB444Column is the on-wire size of one rasterized column:
// 16 pixels tall × 2 bytes per pixel (RGB444) = 32 bytes.
const BytesPerRGB444Column = GlyphHeight * 2

// RenderMono encodes a string as 1-bit monochrome column-major bitmap data.
//
// Output layout, matching what the APK's getFontByteDataCoolleduxForEmoji emits:
//
//	[charCount:2 BE][totalColumns:4 BE]
//	per character: [width:1] [type=0x00:1] [width * 2 bytes: column-major 1-bit]
//
// Two bytes per column: byte 0 holds rows 0-7 (MSB=top), byte 1 holds 8-15.
func RenderMono(text string, showWidth, textSpacing int, centered bool) []byte {
	if textSpacing < 0 {
		textSpacing = 0
	}

	runes := []rune(text)
	glyphs := make([][]byte, len(runes))
	for i, r := range runes {
		glyphs[i] = trimmedGlyphForRune(r)
	}

	leadCols := 0
	if centered {
		total := 0
		for i, g := range glyphs {
			total += len(g) / BytesPerColumn
			if i < len(glyphs)-1 && len(g) > 0 {
				total += textSpacing
			}
		}
		if total < showWidth {
			leadCols = (showWidth - total) / 2
		}
	}

	var body []byte
	charCount := 0
	totalCols := 0
	for i, bm := range glyphs {
		if i == 0 && leadCols > 0 {
			bm = append(make([]byte, leadCols*BytesPerColumn), bm...)
		}
		if i < len(glyphs)-1 && textSpacing > 0 && len(bm) > 0 {
			bm = append(bm, make([]byte, textSpacing*BytesPerColumn)...)
		}
		cols := len(bm) / BytesPerColumn
		if cols == 0 {
			continue
		}
		body = append(body, byte(cols), 0x00)
		body = append(body, bm...)
		charCount++
		totalCols += cols
	}

	out := make([]byte, 0, 6+len(body))
	out = append(out,
		byte(charCount>>8), byte(charCount),
		byte(totalCols>>24), byte(totalCols>>16), byte(totalCols>>8), byte(totalCols),
	)
	out = append(out, body...)
	return out
}

// Render encodes a string as the font-data portion of a CoolLEDUX text
// content packet (content type 0x01).
//
// Each visible character is emitted as:
//
//	[1B: column width] [1B: 0x00 type=text] [width * 32 bytes: RGB444 pixel data]
//
// The pixel data is column-major, 16 pixels per column, 2 bytes per pixel in
// the standard [0x0R, 0xGB] RGB444 layout. Glyph "on" pixels are painted in
// colorRGB; "off" pixels are 0x00 0x00.
//
// Trailing empty columns are stripped from each glyph, and textSpacing empty
// columns are appended between characters. When centered is true (static
// display modes), the first character is prefixed with enough empty columns
// to center the full string inside showWidth; when false (scroll modes), no
// leading pad is added.
//
// Runes that render to zero visible columns (e.g. control characters) are
// skipped.
func Render(text string, colorRGB uint32, showWidth, textSpacing int, centered bool) []byte {
	runes := []rune(text)
	colors := make([]uint32, len(runes))
	for i := range colors {
		colors[i] = colorRGB
	}
	return RenderPerRune(runes, colors, showWidth, textSpacing, centered)
}

// RenderPerRune is like Render but takes a slice of per-rune colors. len(colors)
// must equal len(runes); if a shorter slice is passed, missing entries default
// to the last-provided color (or white if empty).
func RenderPerRune(runes []rune, colors []uint32, showWidth, textSpacing int, centered bool) []byte {
	if textSpacing < 0 {
		textSpacing = 0
	}

	glyphs := make([][]byte, len(runes))
	for i, r := range runes {
		glyphs[i] = trimmedGlyphForRune(r)
	}

	leadCols := 0
	if centered {
		total := 0
		for i, g := range glyphs {
			total += len(g) / BytesPerColumn
			if i < len(glyphs)-1 && len(g) > 0 {
				total += textSpacing
			}
		}
		if total < showWidth {
			leadCols = (showWidth - total) / 2
		}
	}

	var out []byte
	lastColor := uint32(0xFFFFFF)
	for i, g := range glyphs {
		color := lastColor
		if i < len(colors) {
			color = colors[i]
			lastColor = color
		}

		// Figure out the full column layout for this character: optional
		// leading lead-pad on the first char, the glyph itself, then
		// inter-character spacing.
		leading := 0
		if i == 0 {
			leading = leadCols
		}
		trailing := 0
		if i < len(glyphs)-1 && textSpacing > 0 && len(g) > 0 {
			trailing = textSpacing
		}
		glyphCols := len(g) / BytesPerColumn
		totalCols := leading + glyphCols + trailing
		if totalCols == 0 {
			continue
		}

		pixels := make([]byte, totalCols*BytesPerRGB444Column)
		for col := 0; col < glyphCols; col++ {
			b1 := g[col*BytesPerColumn]
			b2 := g[col*BytesPerColumn+1]
			srcCol := leading + col
			for row := 0; row < GlyphHeight; row++ {
				on := false
				if row < 8 {
					on = (b1 & (1 << (7 - row))) != 0
				} else {
					on = (b2 & (1 << (7 - (row - 8)))) != 0
				}
				if !on {
					continue
				}
				off := srcCol*BytesPerRGB444Column + row*2
				r := uint8((color >> 16) & 0xFF)
				gg := uint8((color >> 8) & 0xFF)
				b := uint8(color & 0xFF)
				pixels[off] = ledimage.RGB444Transfer(r)
				pixels[off+1] = (ledimage.RGB444Transfer(gg) << 4) | ledimage.RGB444Transfer(b)
			}
		}

		out = append(out, byte(totalCols), 0x00)
		out = append(out, pixels...)
	}
	return out
}

// Rasterize renders runes into an RGB444 column-major pixel buffer sized to
// fit a canvas of canvasWidth × canvasHeight. The string is centered
// horizontally; if it exceeds canvasWidth it is truncated. Each "on" pixel
// in the glyph bitmap is painted in colorRGB; "off" pixels are black.
// The output matches the format expected by the graffiti (content type 0x02)
// packet: 2 bytes per pixel, iterating columns outer / rows inner.
//
// Uses the default font. For a specific face use RasterizeWithFace.
func Rasterize(runes []rune, colorRGB uint32, canvasWidth, canvasHeight int) []byte {
	return RasterizeWithFace(runes, colorRGB, MonoFace, canvasWidth, canvasHeight, true)
}

// RasterizeAt is like Rasterize but renders the string starting at column 0
// (no centering). Useful for scroll modes where the device handles movement.
func RasterizeAt(runes []rune, colorRGB uint32, canvasWidth, canvasHeight int) []byte {
	return RasterizeWithFace(runes, colorRGB, MonoFace, canvasWidth, canvasHeight, false)
}

// RasterizeWithFace renders runes using the given font face. face may come
// from the Registry or be any font.Face. When centered is true, the string
// is horizontally centered within canvasWidth; otherwise it starts at x=0.
// Vertical centering uses the face's Ascent/Descent metrics, optionally
// adjusted by baselineOffset (positive shifts ink down).
func RasterizeWithFace(runes []rune, colorRGB uint32, face font.Face, canvasWidth, canvasHeight int, centered bool) []byte {
	return rasterizeWithFace(runes, colorRGB, face, canvasWidth, canvasHeight, centered, 0)
}

// RasterizeByName looks up a registered font by name and rasterizes runes
// using it. An empty name selects the default font. Returns an error if the
// name is not registered.
func RasterizeByName(runes []rune, colorRGB uint32, fontName string, canvasWidth, canvasHeight int, centered bool) ([]byte, error) {
	face, err := Face(fontName)
	if err != nil {
		return nil, err
	}
	return rasterizeWithFace(runes, colorRGB, face, canvasWidth, canvasHeight, centered, BaselineOffset(fontName)), nil
}

// rasterizeWithFace is the internal variant that also takes a per-font
// baseline offset.
func rasterizeWithFace(runes []rune, colorRGB uint32, face font.Face, canvasWidth, canvasHeight int, centered bool, baselineOffset int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, canvasWidth, canvasHeight))

	textColor := color.RGBA{
		R: uint8((colorRGB >> 16) & 0xFF),
		G: uint8((colorRGB >> 8) & 0xFF),
		B: uint8(colorRGB & 0xFF),
		A: 0xFF,
	}

	strPixels := font.MeasureString(face, string(runes)).Round()
	startX := 0
	if centered && strPixels < canvasWidth {
		startX = (canvasWidth - strPixels) / 2
	}

	m := face.Metrics()
	ascent := m.Ascent.Ceil()
	descent := m.Descent.Ceil()
	baseline := (canvasHeight-(ascent+descent))/2 + ascent + baselineOffset

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(textColor),
		Face: face,
		Dot:  fixed.P(startX, baseline),
	}
	d.DrawString(string(runes))

	pixels := ledimage.ImageToRGBA(img)
	return ledimage.EncodeImageColumnMajor(pixels, canvasWidth, canvasHeight)
}

// trimmedGlyphForRune returns the 1-bit glyph bitmap for a single rune, after
// applying the fixed-width substitution for space. Two bytes per column,
// MSB = top pixel of the column.
func trimmedGlyphForRune(r rune) []byte {
	if r == ' ' {
		return make([]byte, spaceColumns*BytesPerColumn)
	}
	glyph := make([]byte, BytesPerGlyph)
	copy(glyph, Glyph(r))
	return TrimRight(glyph)
}
