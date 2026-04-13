package text

import (
	_ "embed"
	"fmt"
	"sort"

	"github.com/zachomedia/go-bdf"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

//go:embed bdf/spleen-8x16.bdf
var bdfSpleen8x16 []byte

//go:embed bdf/7x14B.bdf
var bdf7x14B []byte

// FontInfo describes a registered font face available for text rendering.
type FontInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Width       int    `json:"advance_px"` // typical glyph advance in pixels
	Height      int    `json:"line_px"`    // ascent + descent in pixels
	Monospace   bool   `json:"monospace"`
}

// fontEntry pairs a face with its metadata.
type fontEntry struct {
	info           FontInfo
	face           font.Face
	baselineOffset int // add to computed baseline; positive shifts ink DOWN
}

// registry is the process-wide font catalog. Names are case-insensitive
// lookup keys. Extend by calling Register at init-time.
var registry = map[string]fontEntry{}

// DefaultFontName is the font used when no font is specified.
const DefaultFontName = "7x13"

func init() {
	Register(FontInfo{
		Name:        DefaultFontName,
		Description: "Plan 9 bitmap monospace, 7x13 cell, pure 1-bit",
		Width:       basicfont.Face7x13.Advance,
		Height:      basicfont.Face7x13.Ascent + basicfont.Face7x13.Descent,
		Monospace:   true,
	}, basicfont.Face7x13)

	registerBDF("7x14b",
		"X11 Misc Fixed Bold 7x14 (public domain)",
		bdf7x14B, 7, 14, true)

	// Spleen 8x16 declares descent=4 which fills the canvas, so metric-based
	// centering lands a row too high. Uppercase ink only uses ~10 rows; nudge
	// the baseline down by 1 so letters sit visually centered.
	registerBDF("8x16",
		"Spleen 8x16 by Frederic Cambus (BSD-2) — fills full matrix height",
		bdfSpleen8x16, 8, 16, true, 1)
}

// registerBDF parses an embedded BDF file and registers its face. It panics
// on parse failure since the files are compiled into the binary.
func registerBDF(name, description string, data []byte, advance, line int, mono bool, baselineOffset ...int) {
	f, err := bdf.Parse(data)
	if err != nil {
		panic(fmt.Sprintf("bdf parse %q: %v", name, err))
	}
	off := 0
	if len(baselineOffset) > 0 {
		off = baselineOffset[0]
	}
	RegisterWithOffset(FontInfo{
		Name:        name,
		Description: description,
		Width:       advance,
		Height:      line,
		Monospace:   mono,
	}, f.NewFace(), off)
}

// Register adds a font face to the registry under info.Name.
func Register(info FontInfo, face font.Face) {
	RegisterWithOffset(info, face, 0)
}

// RegisterWithOffset is like Register but lets a font adjust its vertical
// placement. The offset is added to the computed baseline (positive = ink
// shifts DOWN). Use this when a BDF's declared descent includes more empty
// space than the font actually uses, which makes the default metric-centered
// placement look too-high.
func RegisterWithOffset(info FontInfo, face font.Face, baselineOffset int) {
	registry[info.Name] = fontEntry{info: info, face: face, baselineOffset: baselineOffset}
}

// BaselineOffset returns the registered vertical correction for a font, or 0.
func BaselineOffset(name string) int {
	if name == "" {
		name = DefaultFontName
	}
	if e, ok := registry[name]; ok {
		return e.baselineOffset
	}
	return 0
}

// Face returns the face registered under name (case-sensitive). An empty
// name returns the default face. An unknown name returns an error.
func Face(name string) (font.Face, error) {
	if name == "" {
		name = DefaultFontName
	}
	e, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown font %q", name)
	}
	return e.face, nil
}

// FontInfo returns metadata for a registered font, or zero value + false.
func FontInfoFor(name string) (FontInfo, bool) {
	if name == "" {
		name = DefaultFontName
	}
	e, ok := registry[name]
	if !ok {
		return FontInfo{}, false
	}
	return e.info, true
}

// AvailableFonts returns all registered fonts sorted by name.
func AvailableFonts() []FontInfo {
	out := make([]FontInfo, 0, len(registry))
	for _, e := range registry {
		out = append(out, e.info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
