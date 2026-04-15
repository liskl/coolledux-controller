package protocol

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// The palette strings below are the APK's `colorModeN` fields verbatim
// (CoolledUXUtils.java:34-63). Several aliases exist and are collapsed in
// colorModeTable: colorMode1==2==4==5==6, colorMode9==11==12==15==16,
// colorMode13==31, colorMode17==18. Keeping them as separate constants
// makes it trivial to diff against the APK if a future firmware tweak
// changes any single one.
const (
	colorMode1Hex = "0F,00,0F,10,0F,20,0F,30,0F,40,0F,50,0F,60,0F,70,0F,80,0F,90,0F,A0,0F,B0,0F,C0,0F,D0,0F,E0," +
		"0F,F0,0E,F0,0D,F0,0C,F0,0B,F0,0A,F0,09,F0,08,F0,07,F0,06,F0,05,F0,04,F0,03,F0,02,F0,01,F0," +
		"00,F0,00,F1,00,F2,00,F3,00,F4,00,F5,00,F6,00,F7,00,F8,00,F9,00,FA,00,FB,00,FC,00,FD,00,FE," +
		"00,FF,00,EF,00,DF,00,CF,00,BF,00,AF,00,9F,00,8F,00,7F,00,6F,00,5F,00,4F,00,3F,00,2F,00,1F," +
		"00,0F,01,0F,02,0F,03,0F,04,0F,05,0F,06,0F,07,0F,08,0F,09,0F,0A,0F,0B,0F,0C,0F,0D,0F,0E,0F," +
		"0F,0F,0F,0E,0F,0D,0F,0C,0F,0B,0F,0A,0F,09,0F,08,0F,07,0F,06,0F,05,0F,04,0F,03,0F,02,0F,01"

	colorMode7Hex = "0F,00,0F,00,0F,00,0F,00,00,F0,00,F0,00,F0,00,F0,00,0F,00,0F,00,0F,00,0F"

	colorMode9Hex = "0F,00,0F,00,0F,00," +
		"0F,F0,0F,F0,0F,F0," +
		"00,F0,00,F0,00,F0," +
		"00,FF,00,FF,00,FF," +
		"00,0F,00,0F,00,0F," +
		"0F,0F,0F,0F,0F,0F"

	// colorMode10 differs from colorMode9 only in the third row
	// (0F,F0 repeat instead of 00,F0).
	colorMode10Hex = "0F,00,0F,00,0F,00," +
		"0F,F0,0F,F0,0F,F0," +
		"0F,F0,0F,F0,0F,F0," +
		"00,FF,00,FF,00,FF," +
		"00,0F,00,0F,00,0F," +
		"0F,0F,0F,0F,0F,0F"

	colorMode13Hex = "0F,00,00,F0,00,0F,0F,F0,00,FF,0F,0F"
	colorMode14Hex = "0F,00,00,0F"

	colorMode17Hex = "0F,00,0F,00,0F,00,00,00,00,00," +
		"0F,F0,0F,F0,0F,F0,00,00,00,00," +
		"00,F0,00,F0,00,F0,00,00,00,00," +
		"00,FF,00,FF,00,FF,00,00,00,00," +
		"00,0F,00,0F,00,0F,00,00,00,00," +
		"0F,0F,0F,0F,0F,0F,00,00,00,00"

	colorMode19Hex = "0F,00,0D,00,0B,00,09,00,07,00,05,00,03,00,01,00"
	colorMode20Hex = "01,00,03,00,05,00,07,00,09,00,0B,00,0D,00,0F,00"
	// colorMode21 in the APK has a trailing comma. parseHexList drops
	// empty tokens so that quirk doesn't leak into the palette bytes.
	colorMode21Hex = "00,F0,00,D0,00,B0,00,90,00,70,00,50,00,30,00,10,"
	colorMode22Hex = "00,10,00,30,00,50,00,70,00,90,00,B0,00,D0,00,F0"
	colorMode23Hex = "00,0F,00,0D,00,0B,00,09,00,07,00,05,00,03,00,01"
	colorMode24Hex = "00,01,00,03,00,05,00,07,00,09,00,0B,00,0D,00,0F"
	colorMode25Hex = "0F,F0,0D,D0,0B,B0,09,90,07,70,05,50,03,30,01,10"
	colorMode26Hex = "01,10,03,30,05,50,07,70,09,90,0B,B0,0D,D0,0F,F0"
	colorMode27Hex = "0F,0F,0D,0D,0B,0B,09,09,07,07,05,05,03,03,01,01"
	colorMode28Hex = "01,01,03,03,05,05,07,07,09,09,0B,0B,0D,0D,0F,0F"

	colorMode29Hex = "0F,00,0D,00,0B,00,09,00,07,00,05,00,03,00,01,00," +
		"00,F0,00,D0,00,B0,00,90,00,70,00,50,00,30,00,10," +
		"00,0F,00,0D,00,0B,00,09,00,07,00,05,00,03,00,01," +
		"0F,F0,0D,D0,0B,B0,09,90,07,70,05,50,03,30,01,10," +
		"00,FF,00,DD,00,BB,00,99,00,77,00,55,00,33,00,11," +
		"0F,0F,0D,0D,0B,0B,09,09,07,07,05,05,03,03,01,01"

	colorMode30Hex = "01,00,03,00,05,00,07,00,09,00,0B,00,0D,00,0F,00," +
		"00,10,00,30,00,50,00,70,00,90,00,B0,00,D0,00,F0," +
		"00,01,00,03,00,05,00,07,00,09,00,0B,00,0D,00,0F," +
		"01,10,03,30,05,50,07,70,09,90,0B,B0,0D,D0,0F,F0," +
		"00,11,00,33,00,55,00,77,00,99,00,BB,00,DD,00,FF," +
		"01,01,03,03,05,05,07,07,09,09,0B,0B,0D,0D,0F,0F"
)

// colorModeSpec describes the per-mode parameter bytes sent after the
// 0x13 0x03 header. i4 uses a signed type so -1 can encode "omit this byte
// entirely" — a distinction the firmware is sensitive to (total packet
// length changes, and on the hardware we've seen that changes behavior).
type colorModeSpec struct {
	i3      uint8
	i4      int8
	i2      uint8
	palette []byte
}

// colorModeTable is built at init from the hex constants above. Keys 3
// and 4 are intentionally omitted because the APK's setColorMode falls
// through both to an empty-palette no-op default; exposing them would
// advertise a mode that produces no output.
//
// Full derivation and byte-layout reasoning: see
// docs/specs/protocol-ble.md "Color Mode and Speed".
var colorModeTable map[int]colorModeSpec

// ColorModeIDs returns the sorted list of valid mode IDs.
func ColorModeIDs() []int {
	ids := make([]int, 0, len(colorModeTable))
	for id := range colorModeTable {
		ids = append(ids, id)
	}
	// Simple insertion sort — N is 29, don't reach for "sort".
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j-1] > ids[j]; j-- {
			ids[j-1], ids[j] = ids[j], ids[j-1]
		}
	}
	return ids
}

func init() {
	p1 := mustParseHexList(colorMode1Hex)
	p7 := mustParseHexList(colorMode7Hex)
	p9 := mustParseHexList(colorMode9Hex)
	p10 := mustParseHexList(colorMode10Hex)
	p13 := mustParseHexList(colorMode13Hex)
	p14 := mustParseHexList(colorMode14Hex)
	p17 := mustParseHexList(colorMode17Hex)
	p19 := mustParseHexList(colorMode19Hex)
	p20 := mustParseHexList(colorMode20Hex)
	p21 := mustParseHexList(colorMode21Hex)
	p22 := mustParseHexList(colorMode22Hex)
	p23 := mustParseHexList(colorMode23Hex)
	p24 := mustParseHexList(colorMode24Hex)
	p25 := mustParseHexList(colorMode25Hex)
	p26 := mustParseHexList(colorMode26Hex)
	p27 := mustParseHexList(colorMode27Hex)
	p28 := mustParseHexList(colorMode28Hex)
	p29 := mustParseHexList(colorMode29Hex)
	p30 := mustParseHexList(colorMode30Hex)

	colorModeTable = map[int]colorModeSpec{
		1:  {i3: 2, i4: 0, i2: 90, palette: p1},
		2:  {i3: 2, i4: 1, i2: 90, palette: p1},
		5:  {i3: 2, i4: 4, i2: 90, palette: p1},
		6:  {i3: 2, i4: 5, i2: 90, palette: p1},
		7:  {i3: 2, i4: 0, i2: 12, palette: p7},
		8:  {i3: 2, i4: 1, i2: 12, palette: p7},
		9:  {i3: 2, i4: 0, i2: 18, palette: p9},
		10: {i3: 2, i4: 1, i2: 18, palette: p10},
		11: {i3: 2, i4: 2, i2: 18, palette: p9},
		12: {i3: 2, i4: 3, i2: 18, palette: p9},
		13: {i3: 1, i4: -1, i2: 6, palette: p13},
		14: {i3: 1, i4: -1, i2: 2, palette: p14},
		15: {i3: 3, i4: 4, i2: 18, palette: p9},
		16: {i3: 3, i4: 5, i2: 18, palette: p9},
		17: {i3: 2, i4: 0, i2: 30, palette: p17},
		18: {i3: 2, i4: 1, i2: 30, palette: p17},
		19: {i3: 2, i4: 0, i2: 8, palette: p19},
		20: {i3: 2, i4: 1, i2: 8, palette: p20},
		21: {i3: 2, i4: 0, i2: 8, palette: p21},
		22: {i3: 2, i4: 1, i2: 8, palette: p22},
		23: {i3: 2, i4: 0, i2: 8, palette: p23},
		24: {i3: 2, i4: 1, i2: 8, palette: p24},
		25: {i3: 2, i4: 0, i2: 8, palette: p25},
		26: {i3: 2, i4: 1, i2: 8, palette: p26},
		27: {i3: 2, i4: 0, i2: 8, palette: p27},
		28: {i3: 2, i4: 1, i2: 8, palette: p28},
		29: {i3: 2, i4: 0, i2: 48, palette: p29},
		30: {i3: 2, i4: 1, i2: 48, palette: p30},
		31: {i3: 4, i4: -1, i2: 6, palette: p13},
	}
}

// parseHexList parses the APK's comma-separated hex-pair format
// ("0F,00,0F,10,...") into raw bytes. Whitespace and empty tokens are
// skipped so newlines inside the APK strings and trailing commas don't
// produce phantom bytes.
func parseHexList(s string) ([]byte, error) {
	tokens := strings.Split(s, ",")
	out := make([]byte, 0, len(tokens))
	for i, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if len(tok) != 2 {
			return nil, fmt.Errorf("token %d %q: want 2 hex chars", i, tok)
		}
		b, err := hex.DecodeString(tok)
		if err != nil {
			return nil, fmt.Errorf("token %d %q: %w", i, tok, err)
		}
		out = append(out, b[0])
	}
	return out, nil
}

func mustParseHexList(s string) []byte {
	b, err := parseHexList(s)
	if err != nil {
		panic(fmt.Sprintf("parseHexList: %v", err))
	}
	return b
}

// BuildColorSpeedCommand builds a 0x13/0x02 color-speed command. speed is
// passed through verbatim; typical range is 1-10. Wrapping uses stream
// framing only (no CRC) to match the APK's getSendDataWithInfo at
// CoolledUXUtils.java:4835 / ILedClockUtils.java:4324.
func BuildColorSpeedCommand(speed uint8) []byte {
	return BuildStreamFrame([]byte{CMD_COLOR, COLOR_SUBTYPE_SPEED, speed})
}

// BuildColorModeCommand builds a 0x13/0x03 color-mode command for a
// preset animation. Valid mode IDs are 1, 2, 5..31; 3 and 4 are absent
// because the APK fall-through makes them empty no-ops. Returns an error
// for any other input.
//
// The i4 byte is conditionally present per the APK: omitting it entirely
// is NOT the same as sending 0. See docs/specs/protocol-ble.md "Color
// Mode and Speed" for the full trace.
func BuildColorModeCommand(mode int) ([]byte, error) {
	spec, ok := colorModeTable[mode]
	if !ok {
		return nil, fmt.Errorf("color mode %d is not supported (valid: 1,2,5..31)", mode)
	}
	data := make([]byte, 0, 5+len(spec.palette))
	data = append(data, CMD_COLOR, COLOR_SUBTYPE_MODE, spec.i3)
	if spec.i4 >= 0 {
		data = append(data, uint8(spec.i4))
	}
	data = append(data, spec.i2)
	data = append(data, spec.palette...)
	return BuildStreamFrame(data), nil
}
