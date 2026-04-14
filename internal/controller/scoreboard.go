package controller

import (
	_ "embed"
	"encoding/binary"
)

// Scoreboard overlay builder for the 16x96 CoolLEDUX display.
//
// Content type 0x0b carries a composite layout with:
//   - Host team score (3-digit, 21x10)
//   - Visit team score (3-digit, 21x10)
//   - Host period/set counter (1-digit 4x5)
//   - Visit period/set counter (1-digit 4x5)
//   - Game clock MM:SS (4x5 digits with 1-column colon)
//
// All offsets, dimensions, and bitmap data are extracted verbatim from the
// CoolLED 1248 APK for DEVICE_ROW=16, DEVICE_COLUMN=96:
//   - DiscoverScoreboardActivity.getProgramData (layout values, line 557+)
//   - CoolledUXUtils.getDataWithScoreBoardCombineProgram (packet shape; baksmali'd
//     because jadx bailed on the 1264-instruction method)
//
// Unlike countdown/stopwatch, the score/time digit regions are not driven
// directly by the firmware clock — they update in response to 0x11 command
// subtypes (SetScores, SetTime). The program only defines the rendering slots.

// scoreboardBackgroundGIF is the APK's scoreboard background animation.
// Extracted from res/drawable-xxhdpi-v4/ic_scoreboard_bg_1696.gif (286 bytes;
// much smaller than the countdown/stopwatch GIFs, probably a sparse frame
// that just paints a border / "VS" separator over the overlay).
//
//go:embed assets/scoreboard_bg_1696.gif
var scoreboardBackgroundGIF []byte

// scoreBoardSmallDigits96x16 is the 10-digit, 4-byte-per-digit bitmap the
// APK uses for BOTH the "total score" (period/set counter) slot AND the
// game clock on 16x96. Extracted from CoolledUXUtils.smali line 480 (v13,
// reassigned from an earlier 220-byte value — the earlier const never
// reaches the 16x96 emission path). Column-major, 1 byte per column, 5-row
// MSB-packed (top 5 bits used): 3 glyph columns + 1 spacer = 4 bytes/digit.
// Matches scoreTotalNumWidth=4, scoreTotalNumHeight=5 (= timeNumWidth/Height).
var scoreBoardSmallDigits96x16 = []byte{
	// 0: vertical bars + top/bottom bar
	248, 136, 248, 0,
	// 1: trailing vertical bar only
	0, 248, 0, 0,
	// 2
	184, 168, 232, 0,
	// 3
	168, 168, 248, 0,
	// 4
	224, 32, 248, 0,
	// 5
	232, 168, 184, 0,
	// 6
	248, 168, 184, 0,
	// 7
	128, 128, 248, 0,
	// 8
	248, 168, 248, 0,
	// 9
	232, 168, 248, 0,
}

// scoreBoardColon96x16 is the 1-column colon separator between MM and SS on
// 16x96. Literal `"80"` in the APK smali (one byte, 0x50), which is a
// 5-row-tall dot pattern: rows 1 and 3 lit, others off (binary 01010000
// MSB-aligned). Paired with spaceMinuteWidth=1.
var scoreBoardColon96x16 = []byte{0x50}

// buildScoreboardBackgroundContent decodes the embedded scoreboard background
// gif into a content-type-0x03 animation block. See buildOverlayBackgroundContent.
func buildScoreboardBackgroundContent() ([]byte, error) {
	return buildOverlayBackgroundContent(scoreboardBackgroundGIF, "scoreboard")
}

// buildScoreboardContent96x16 produces the scoreboard content block (content
// type 0x0b) without a program wrapper, so it can be composed with an
// animation block. color is applied as the tint for every region — the APK
// uses white for every slot, but we accept a single parameter for simplicity.
func buildScoreboardContent96x16(color uint32) []byte {
	// Layout values for DEVICE_ROW=16 && DEVICE_COLUMN=96 per
	// DiscoverScoreboardActivity.java:557-598. With DEVICE_COLUMN=96 the
	// "(DEVICE_COLUMN-96)/2" offset is 0, so we can hardcode absolute columns.
	const (
		layerType          = 0x01
		scoreNumHeight     = 10
		scoreNumWidth      = 7
		scoreTotalNumH     = 5
		scoreTotalNumW     = 4
		timeNumH           = 5
		timeNumW           = 4
		hostScoreCol       = 14
		hostScoreRow       = 4
		hostScoreW         = 21
		hostScoreH         = 10
		visitScoreCol      = 61
		visitScoreRow      = 4
		visitScoreW        = 21
		visitScoreH        = 10
		totalHostCol       = 41
		totalHostRow       = 2
		totalHostW         = 4
		totalHostH         = 5
		totalVisitCol      = 51
		totalVisitRow      = 2
		totalVisitW        = 4
		totalVisitH        = 5
		minuteCol          = 39
		minuteRow          = 11
		minuteW            = 8
		minuteH            = 5
		spaceMinCol        = 47
		spaceMinRow        = 11
		spaceMinW          = 1
		spaceMinH          = 5
		secondsCol         = 49
		secondsRow         = 11
		secondsW           = 8
		secondsH           = 5
	)

	r4, gb := rgb444PackBitShift(color)
	col := []byte{r4, gb}

	// Build the content body incrementally; prepend the 4-byte total length
	// at the end. This avoids brittle offset math as the layout grows.
	body := make([]byte, 0, 640)

	// Header: content-type, 7 padding, layerType, 1 padding.
	body = append(body, 0x0b)
	body = append(body, make([]byte, 7)...)
	body = append(body, layerType)
	body = append(body, 0x00)

	u16BE := func(v uint16) []byte {
		var b [2]byte
		binary.BigEndian.PutUint16(b[:], v)
		return b[:]
	}

	// Score-digit glyph dimensions + bitmap (7 col × 10 row cell, 140 bytes,
	// reused from the time-count digit bitmap).
	body = append(body, u16BE(scoreNumHeight)...)
	body = append(body, u16BE(scoreNumWidth)...)
	body = append(body, u16BE(uint16(len(timeCountDigits96x16)))...)
	body = append(body, timeCountDigits96x16...)

	// Per-region serializer: color(2) col(2) row(2) w(2) h(2).
	region := func(c, r, w, h uint16) {
		body = append(body, col...)
		body = append(body, u16BE(c)...)
		body = append(body, u16BE(r)...)
		body = append(body, u16BE(w)...)
		body = append(body, u16BE(h)...)
	}

	region(hostScoreCol, hostScoreRow, hostScoreW, hostScoreH)
	region(visitScoreCol, visitScoreRow, visitScoreW, visitScoreH)

	// ScoreTotalNum glyph dimensions + bitmap (APK's v13, 140 bytes).
	body = append(body, u16BE(scoreTotalNumH)...)
	body = append(body, u16BE(scoreTotalNumW)...)
	body = append(body, u16BE(uint16(len(scoreBoardSmallDigits96x16)))...)
	body = append(body, scoreBoardSmallDigits96x16...)

	region(totalHostCol, totalHostRow, totalHostW, totalHostH)
	region(totalVisitCol, totalVisitRow, totalVisitW, totalVisitH)

	// Time-digit glyph dimensions + bitmap. On 16x96 the smali traces back
	// to the same v13 as the total-score digits, so reuse it.
	body = append(body, u16BE(timeNumH)...)
	body = append(body, u16BE(timeNumW)...)
	body = append(body, u16BE(uint16(len(scoreBoardSmallDigits96x16)))...)
	body = append(body, scoreBoardSmallDigits96x16...)

	region(minuteCol, minuteRow, minuteW, minuteH)
	region(spaceMinCol, spaceMinRow, spaceMinW, spaceMinH)

	// Colon separator bitmap (1 byte).
	body = append(body, u16BE(uint16(len(scoreBoardColon96x16)))...)
	body = append(body, scoreBoardColon96x16...)

	region(secondsCol, secondsRow, secondsW, secondsH)

	// Prepend the 4-byte total length (including itself).
	total := uint32(len(body) + 4)
	out := make([]byte, 4+len(body))
	binary.BigEndian.PutUint32(out[0:4], total)
	copy(out[4:], body)
	return out
}

// buildScoreboardProgram96x16 composes the scoreboard overlay: the APK
// background animation (content type 0x03) + the scoreboard content (0x0b).
// Falls back to content-only if the embedded GIF fails to decode.
func buildScoreboardProgram96x16(color uint32) []byte {
	content := buildScoreboardContent96x16(color)
	bg, err := buildScoreboardBackgroundContent()
	if err != nil {
		return wrapProgram(content)
	}
	return wrapCompositeProgram(bg, content)
}
