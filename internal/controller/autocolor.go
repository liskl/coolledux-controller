package controller

import (
	"encoding/binary"
	"fmt"

	"github.com/liskl/coolledux-controller/internal/models"
	"github.com/liskl/coolledux-controller/internal/text"
)

// autoColorEntry maps an autoColorType (1-28) to the firmware's animation
// mode, direction byte, and RGB444 palette. Extracted from CoolledUXUtils.java
// getDataWithTextAutoColorProgramContent (lines 3655-3895).
type autoColorEntry struct {
	animMode  uint8
	direction uint8
	palette   []byte
}

// Rainbow gradient: 44 colors cycling R→Y→G→C→B→M→R in RGB444 [0R,GB] pairs.
var paletteRainbow = []byte{
	0x0F, 0x00, 0x0F, 0x20, 0x0F, 0x40, 0x0F, 0x60,
	0x0F, 0x80, 0x0F, 0xA0, 0x0F, 0xC0, 0x0F, 0xF0,
	0x0C, 0xF0, 0x0A, 0xF0, 0x08, 0xF0, 0x06, 0xF0,
	0x04, 0xF0, 0x02, 0xF0, 0x00, 0xF0, 0x00, 0xF2,
	0x00, 0xF4, 0x00, 0xF6, 0x00, 0xF8, 0x00, 0xFA,
	0x00, 0xFC, 0x00, 0xFF, 0x00, 0xCF, 0x00, 0xAF,
	0x00, 0x8F, 0x00, 0x6F, 0x00, 0x4F, 0x00, 0x2F,
	0x00, 0x0F, 0x02, 0x0F, 0x04, 0x0F, 0x06, 0x0F,
	0x08, 0x0F, 0x0A, 0x0F, 0x0C, 0x0F, 0x0F, 0x0F,
	0x0F, 0x0C, 0x0F, 0x0A, 0x0F, 0x08, 0x0F, 0x06,
	0x0F, 0x04, 0x0F, 0x02, 0x0F, 0x00,
}

// 6-color palette: red, yellow, green, cyan, blue, white.
var palette6Color = []byte{
	0x0F, 0x00, 0x0F, 0xF0, 0x00, 0xF0, 0x00, 0xFF, 0x00, 0x0F, 0x0F, 0x0F,
}

// 3-color fixed palette used by auto-color types 15-28.
var palette3Fixed = []byte{0x00, 0xFF, 0x0F, 0x0F, 0x0F, 0xF0}

// autoColorTypes maps autoColorType (1-28) to firmware animation parameters.
// Each pair of types shares an animMode but differs in direction, producing
// a visual variant (e.g. left-to-right vs right-to-left).
var autoColorTypes = map[int]autoColorEntry{
	1:  {1, 0, paletteRainbow},
	2:  {1, 1, paletteRainbow},
	3:  {2, 0, paletteRainbow},
	4:  {3, 2, paletteRainbow},
	5:  {3, 3, paletteRainbow},
	6:  {4, 4, paletteRainbow},
	7:  {4, 5, paletteRainbow},
	8:  {5, 0, palette6Color},
	9:  {6, 0, palette6Color},
	10: {6, 1, palette6Color},
	11: {7, 0, paletteRainbow},
	12: {7, 1, paletteRainbow},
	13: {8, 0, paletteRainbow},
	14: {8, 1, paletteRainbow},
	15: {1, 0, palette3Fixed},
	16: {1, 1, palette3Fixed},
	17: {2, 0, palette3Fixed},
	18: {3, 2, palette3Fixed},
	19: {3, 3, palette3Fixed},
	20: {4, 4, palette3Fixed},
	21: {4, 5, palette3Fixed},
	22: {5, 0, palette3Fixed},
	23: {6, 0, palette3Fixed},
	24: {6, 1, palette3Fixed},
	25: {7, 0, palette3Fixed},
	26: {7, 1, palette3Fixed},
	27: {8, 0, palette3Fixed},
	28: {8, 1, palette3Fixed},
}

// buildAutoColorContent builds a content type 0x05 auto-color block that
// tells the firmware to apply a color animation palette to the text region.
// This block is uploaded as part of a composite program alongside a 0x01
// text content block.
//
//	[totalLen:4 BE][0x05][0x00 × 7][startCol:2 BE][startRow:2 BE]
//	[showWidth:2 BE][showHeight:2 BE][animMode:1][speed:1][direction:1]
//	[0x00][paletteSize:2 BE][palette...]
func buildAutoColorContent(startCol, startRow, width, height int, autoColorType int, speed uint8) ([]byte, error) {
	entry, ok := autoColorTypes[autoColorType]
	if !ok {
		return nil, fmt.Errorf("autoColorType %d not in range 1-28", autoColorType)
	}

	contentLen := 26 + len(entry.palette)
	content := make([]byte, contentLen)
	binary.BigEndian.PutUint32(content[0:4], uint32(contentLen))
	content[4] = 0x05
	// content[5:12] reserved zeros
	binary.BigEndian.PutUint16(content[12:14], uint16(startCol))
	binary.BigEndian.PutUint16(content[14:16], uint16(startRow))
	binary.BigEndian.PutUint16(content[16:18], uint16(width))
	binary.BigEndian.PutUint16(content[18:20], uint16(height))
	content[20] = entry.animMode
	content[21] = speed
	content[22] = entry.direction
	content[23] = 0x00
	binary.BigEndian.PutUint16(content[24:26], uint16(len(entry.palette)))
	copy(content[26:], entry.palette)

	return content, nil
}

// buildTextContent builds a raw content type 0x01 text block (no wrapper).
// Used both by buildTextProgram (single-content upload) and by the composite
// auto-color path.
//
//	[totalLen:4 BE][0x01][0x00 × 7][layerType=1:1]
//	[startCol:2 BE][startRow:2 BE][width:2 BE][height:2 BE]
//	[mode:1][speed:1][stayTime:1][moveSpace:2 BE][textData...]
func buildTextContent(width, height int, mode models.TextShowMode, speed, stayTime uint8, moveSpace uint16, textData []byte) []byte {
	contentLen := 26 + len(textData)
	content := make([]byte, contentLen)
	binary.BigEndian.PutUint32(content[0:4], uint32(contentLen))
	content[4] = 0x01
	content[12] = 0x01 // layer_type MUST be 1
	binary.BigEndian.PutUint16(content[13:15], 0)
	binary.BigEndian.PutUint16(content[15:17], 0)
	binary.BigEndian.PutUint16(content[17:19], uint16(width))
	binary.BigEndian.PutUint16(content[19:21], uint16(height))
	content[21] = uint8(mode)
	content[22] = speed
	content[23] = stayTime
	binary.BigEndian.PutUint16(content[24:26], moveSpace)
	if len(textData) > 0 {
		copy(content[26:], textData)
	}
	return content
}

// renderMonoTextData rasterizes a string via the legacy 16-column bitmap font
// into the RenderMono format expected by content type 0x01.
func renderMonoTextData(s string, showWidth int, centered bool) []byte {
	return text.RenderMono(s, showWidth, 1, centered)
}
