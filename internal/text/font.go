// Package text renders strings into the per-character bitmap byte stream
// expected by the CoolLEDUX text content packet (content type 0x01).
//
// Glyphs come from the bitmap font extracted from the official Android app
// (see fonts/README.md). Each glyph is 16 columns × 16 rows, 1 bit per pixel,
// column-major, 2 bytes per column, MSB = top pixel.
package text

import (
	_ "embed"
)

//go:embed fonts/unicode_16_bold.bin
var font16Bold []byte

const (
	GlyphWidth     = 16
	GlyphHeight    = 16
	BytesPerGlyph  = 32
	BytesPerColumn = 2
	glyphCount     = 0x10000
	replacement    = 0xFFFD
)

// Glyph returns the raw 32-byte bitmap for a Unicode code point in the BMP.
// Runes outside the BMP fall back to U+FFFD.
func Glyph(r rune) []byte {
	code := int(r)
	if code < 0 || code >= glyphCount {
		code = replacement
	}
	off := code * BytesPerGlyph
	return font16Bold[off : off+BytesPerGlyph]
}

// TrimRight returns the glyph with its trailing all-zero columns removed.
// The result may be empty (e.g. for U+0020 space, whose glyph is fully blank).
func TrimRight(glyph []byte) []byte {
	end := len(glyph)
	for end >= BytesPerColumn && glyph[end-1] == 0 && glyph[end-2] == 0 {
		end -= BytesPerColumn
	}
	return glyph[:end]
}

// GlyphColumns returns the trimmed column count for a rune, i.e. the number
// of non-empty columns in its bitmap. Useful for building width metadata that
// matches the device's embedded font spacing.
func GlyphColumns(r rune) int {
	g := append([]byte{}, Glyph(r)...)
	return len(TrimRight(g)) / BytesPerColumn
}
