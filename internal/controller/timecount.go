package controller

import (
	_ "embed"
	"encoding/binary"
	"fmt"

	ledimage "github.com/liskl/coolledux-controller/internal/image"
)

// countdownBackgroundGIF is the APK's pre-baked 18-frame 96x16 countdown
// background animation (purple frame + hourglass), used verbatim as the
// content-type-0x03 animation that the time-count digits overlay on top of.
// Extracted from res/drawable-xxhdpi-v4/ic_countdown_bg_animation_1696.gif
// in the CoolLED 1248 APK.
//
//go:embed assets/countdown_bg_1696.gif
var countdownBackgroundGIF []byte

// buildCountdownBackgroundContent decodes the embedded background gif and
// returns the animation content block (content type 0x03, no program wrapper)
// suitable for composition with the time-count content.
func buildCountdownBackgroundContent() ([]byte, error) {
	const (
		w          = 96
		h          = 16
		startCol   = 0
		startRow   = 0
	)
	g, err := ledimage.DecodeGIF(countdownBackgroundGIF)
	if err != nil {
		return nil, fmt.Errorf("decoding countdown background: %w", err)
	}
	frames, delays := ledimage.ExtractFrames(g, w, h, ledimage.FitStretch)
	if len(frames) == 0 {
		return nil, fmt.Errorf("countdown background has no frames")
	}
	encoded := make([][]byte, 0, len(frames))
	for _, frame := range frames {
		encoded = append(encoded, ledimage.EncodeImageColumnMajor(frame, w, h))
	}
	return buildAnimationContent(startCol, startRow, w, h, encoded, delays), nil
}

// Time-count program builder for the 16x96 CoolLEDUX display.
//
// Content type 0x0a is the device's "time count" program: a bitmap-driven
// HH:MM:SS render whose contents are driven by the firmware countdown timer
// (set/start via 0x0F). The packet carries the digit and separator bitmaps
// plus per-position layout (column, row, width, height, color).
//
// All offsets, dimensions, and bitmap data here are extracted verbatim from
// the CoolLED 1248 APK for DEVICE_ROW=16 && DEVICE_COLUMN>=96 (specifically
// the 96-column branch). See:
//   - DiscoverCountdownActivity.getProgramData (the layout values)
//   - CoolledUXUtils.getDataWithTimeCountCombineProgram (the packet shape)

// timeCountDigits96x16 is the 0-9 digit bitmap data for the 16x96 display.
// 14 bytes per digit (7 columns × 2 bytes for 10-row MSB-packed columns),
// 140 bytes total. Extracted verbatim from CoolledUXUtils.smali line 16433
// (register v17 in getDataWithTimeCountCombineProgram) — this is the bitmap
// the APK actually uploads for 16x96 timeCountMode=0. The earlier 390-byte
// variant (str2) is a different geometry used for other device sizes; it's
// unused on 16x96 and produced our previous blocky render.
var timeCountDigits96x16 = []byte{
	// 0
	127, 128, 255, 192, 192, 192, 192, 192, 255, 192, 127, 128, 0, 0,
	// 1
	32, 192, 96, 192, 255, 192, 255, 192, 0, 192, 0, 192, 0, 0,
	// 2
	97, 192, 227, 192, 198, 192, 204, 192, 248, 192, 120, 192, 0, 0,
	// 3
	97, 128, 225, 192, 204, 192, 204, 192, 255, 192, 115, 128, 0, 0,
	// 4
	30, 0, 62, 0, 102, 0, 255, 192, 255, 192, 6, 0, 0, 0,
	// 5
	249, 128, 249, 192, 216, 192, 216, 192, 223, 192, 207, 128, 0, 0,
	// 6
	31, 128, 63, 192, 108, 192, 204, 192, 143, 192, 7, 128, 0, 0,
	// 7
	224, 0, 224, 0, 199, 192, 207, 192, 248, 0, 240, 0, 0, 0,
	// 8
	115, 128, 255, 192, 204, 192, 204, 192, 255, 192, 115, 128, 0, 0,
	// 9
	121, 128, 253, 192, 204, 192, 204, 192, 255, 192, 127, 128, 0, 0,
}

// timeCountSeparator96x16 is the colon (:) bitmap used between hour:minute
// and minute:seconds. From CoolledUXUtils.smali v16: "51, 0, 51, 0" =
// 0x33, 0x00, 0x33, 0x00 — two 2-byte columns each encoding a colon's
// dot pattern rendered into a numHeight-tall cell.
var timeCountSeparator96x16 = []byte{51, 0, 51, 0}

// rgb444PackBitShift packs an 8-bit-per-channel RGB color into the 2-byte
// RGB444 representation used by the time-count program: byte0 = 0x0R,
// byte1 = (G<<4)|B. This is the simple bit-shift variant
// (TextEmojiManagerCoolLEDUX.getColorDataWithColor), distinct from the
// piecewise transfer used elsewhere.
func rgb444PackBitShift(rgb uint32) (byte, byte) {
	r := uint8((rgb >> 16) & 0xFF)
	g := uint8((rgb >> 8) & 0xFF)
	b := uint8(rgb & 0xFF)
	r4 := r >> 4
	g4 := g >> 4
	b4 := b >> 4
	return r4, (g4 << 4) | b4
}

// buildTimeCountProgram96x16 assembles a composite program that matches
// what the APK uploads for its countdown UI: a frame/hourglass animation
// (content type 0x03) overlaid by the HH:MM:SS time-count (content type
// 0x0a). The firmware ticks the time-count via 0x0F commands.
//
// If the background animation can't be loaded (e.g. missing asset), we
// fall back to uploading the time-count alone — still functional, just
// without the purple frame + hourglass.
func buildTimeCountProgram96x16(c uint32) []byte {
	timeContent := buildTimeCountContent96x16With(c, timeCountDigits96x16)
	bgContent, err := buildCountdownBackgroundContent()
	if err != nil {
		// Fallback: time-count only.
		return wrapProgram(timeContent)
	}
	return wrapCompositeProgram(bgContent, timeContent)
}

// buildTimeCountProgram96x16With is the variant taking a pre-built digit
// bitmap (10 digits × 39 bytes = 390 bytes). Used for the probe endpoint
// to map firmware byte→pixel interpretation. Unlike buildTimeCountProgram96x16,
// this path does NOT include the background animation — probes need clean
// output with just digits + separators.
func buildTimeCountProgram96x16With(c uint32, digitBitmap []byte) []byte {
	return wrapProgram(buildTimeCountContent96x16With(c, digitBitmap))
}

// buildTimeCountContent96x16With produces just the time-count content block
// (content type 0x0a) without a program wrapper, so it can be composed with
// other content blocks in a multi-content program.
func buildTimeCountContent96x16With(c uint32, digitBitmap []byte) []byte {
	r4, gb := rgb444PackBitShift(c)
	colorBytes := []byte{r4, gb}

	// Per APK DiscoverCountdownActivity.getProgramData for ROW=16 COL>=96.
	const (
		layerType     = 0x01
		timeCountMode = 0
		numHeight     = 10
		numWidth      = 7
		hourStartCol   = 11
		spaceHourCol   = 26
		minuteStartCol = 30
		spaceMinuteCol = 45
		secondsCol     = 49
		startRow       = 3
		digitW         = 14 // two digits side by side
		digitH         = 10
		separatorW     = 2
		separatorH     = 10
	)

	w := func(buf []byte, off int, val uint16) int {
		binary.BigEndian.PutUint16(buf[off:off+2], val)
		return off + 2
	}

	contentLen := 1 + 7 + 1 + 1 + 2 + 2 + 2 + len(digitBitmap) +
		2 + 2 + 2 + 2 + 2 + // hour: color + col + row + w + h
		2 + 2 + 2 + 2 + 2 + // spaceHour
		2 + len(timeCountSeparator96x16) + // separator len + data
		2 + 2 + 2 + 2 + 2 + // minute
		2 + 2 + 2 + 2 + 2 + // spaceMinute
		2 + len(timeCountSeparator96x16) +
		2 + 2 + 2 + 2 + 2 // seconds
	totalLen := 4 + contentLen

	content := make([]byte, totalLen)
	binary.BigEndian.PutUint32(content[0:4], uint32(totalLen))
	off := 4
	content[off] = 0x0a
	off++
	off += 7 // 7 zero padding bytes
	content[off] = layerType
	off++
	content[off] = timeCountMode
	off++
	off = w(content, off, numHeight)
	off = w(content, off, numWidth)
	off = w(content, off, uint16(len(digitBitmap)))
	copy(content[off:], digitBitmap)
	off += len(digitBitmap)

	writePosition := func(col, row, width, height int) {
		content[off] = colorBytes[0]
		content[off+1] = colorBytes[1]
		off += 2
		off = w(content, off, uint16(col))
		off = w(content, off, uint16(row))
		off = w(content, off, uint16(width))
		off = w(content, off, uint16(height))
	}

	writePosition(hourStartCol, startRow, digitW, digitH)
	writePosition(spaceHourCol, startRow, separatorW, separatorH)
	off = w(content, off, uint16(len(timeCountSeparator96x16)))
	copy(content[off:], timeCountSeparator96x16)
	off += len(timeCountSeparator96x16)

	writePosition(minuteStartCol, startRow, digitW, digitH)
	writePosition(spaceMinuteCol, startRow, separatorW, separatorH)
	off = w(content, off, uint16(len(timeCountSeparator96x16)))
	copy(content[off:], timeCountSeparator96x16)
	off += len(timeCountSeparator96x16)

	writePosition(secondsCol, startRow, digitW, digitH)

	return content
}
