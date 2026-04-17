package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

// roundTripCommand takes a fully framed control command (stream frame wrapping
// [cmd][data][CRC32_LE]), parses the stream frame, and returns the inner payload.
// Control commands use stream framing only, without the [0x52,0x52] BLE packet header.
func roundTripCommand(t *testing.T, frame []byte) []byte {
	t.Helper()

	inner, err := ParseStreamFrame(frame)
	if err != nil {
		t.Fatalf("ParseStreamFrame: %v", err)
	}

	return inner
}

// verifyCRC checks that the last 4 bytes of payload are a valid little-endian CRC32
// over the preceding bytes.
func verifyCRC(t *testing.T, payload []byte) {
	t.Helper()
	if len(payload) < 5 {
		t.Fatalf("payload too short for CRC check: len=%d", len(payload))
	}
	data := payload[:len(payload)-4]
	gotCRC := binary.LittleEndian.Uint32(payload[len(payload)-4:])
	wantCRC := Calculate(data)
	if gotCRC != wantCRC {
		t.Errorf("CRC mismatch: got 0x%08X, want 0x%08X", gotCRC, wantCRC)
	}
}

func TestBuildPowerCommand(t *testing.T) {
	tests := []struct {
		name    string
		on      bool
		wantArg byte
	}{
		{"power_on", true, PowerOn},
		{"power_off", false, PowerOff},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := BuildPowerCommand(tt.on)
			payload := roundTripCommand(t, frame)

			// payload: [CMD_POWER][arg][CRC32 x4]
			if len(payload) != 6 {
				t.Fatalf("payload length = %d, want 6", len(payload))
			}
			if payload[0] != CMD_POWER {
				t.Errorf("command code = 0x%02X, want 0x%02X (CMD_POWER)", payload[0], CMD_POWER)
			}
			if payload[1] != tt.wantArg {
				t.Errorf("power arg = 0x%02X, want 0x%02X", payload[1], tt.wantArg)
			}

			verifyCRC(t, payload)
		})
	}
}

func TestBuildBrightnessCommand(t *testing.T) {
	tests := []struct {
		name       string
		brightness uint8
	}{
		{"min", 0},
		{"mid", 128},
		{"max", 255},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := BuildBrightnessCommand(tt.brightness)
			payload := roundTripCommand(t, frame)

			// payload: [CMD_BRIGHTNESS][brightness][CRC32 x4]
			if len(payload) != 6 {
				t.Fatalf("payload length = %d, want 6", len(payload))
			}
			if payload[0] != CMD_BRIGHTNESS {
				t.Errorf("command code = 0x%02X, want 0x%02X", payload[0], CMD_BRIGHTNESS)
			}
			if payload[1] != tt.brightness {
				t.Errorf("brightness = %d, want %d", payload[1], tt.brightness)
			}

			verifyCRC(t, payload)
		})
	}
}

func TestBuildFlipCommand(t *testing.T) {
	tests := []struct {
		name string
		mode uint8
	}{
		{"none", 0},
		{"horizontal", 1},
		{"vertical", 2},
		{"both", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := BuildFlipCommand(tt.mode)
			payload := roundTripCommand(t, frame)

			if payload[0] != CMD_FLIP {
				t.Errorf("command code = 0x%02X, want 0x%02X", payload[0], CMD_FLIP)
			}
			if payload[1] != tt.mode {
				t.Errorf("flip mode = %d, want %d", payload[1], tt.mode)
			}

			verifyCRC(t, payload)
		})
	}
}

func TestBuildPasswordCommand_ExactBytes(t *testing.T) {
	// Fixed key makes the XOR output deterministic so we can assert a
	// byte-exact packet. Password "1234" → nibbles [1, 2, 3, 4], key
	// 0xA5 → encoded nibbles [0xA4, 0xA7, 0xA6, 0xA1], checksum XOR
	// over those four = 0xA4 ^ 0xA7 ^ 0xA6 ^ 0xA1 = 0x04.
	payload, err := buildPasswordCommand(CMD_CHECK_PASSWORD, "1234", 0xA5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inner := roundTripCommand(t, payload)

	want := []byte{CMD_CHECK_PASSWORD, 0xA5, 0xA4, 0xA7, 0xA6, 0xA1, 0x04}
	if !bytes.Equal(inner, want) {
		t.Errorf("got  % X\nwant % X", inner, want)
	}
}

func TestBuildPasswordCommand_SetAndCheckDifferOnlyInCmdByte(t *testing.T) {
	check, err := buildPasswordCommand(CMD_CHECK_PASSWORD, "abcdef", 0x42)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	set, err := buildPasswordCommand(CMD_SET_PASSWORD, "abcdef", 0x42)
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	innerCheck := roundTripCommand(t, check)
	innerSet := roundTripCommand(t, set)

	if innerCheck[0] != CMD_CHECK_PASSWORD {
		t.Errorf("check cmd = 0x%02X, want 0x0D", innerCheck[0])
	}
	if innerSet[0] != CMD_SET_PASSWORD {
		t.Errorf("set cmd = 0x%02X, want 0x0E", innerSet[0])
	}
	// Everything after the cmd byte (key + nibbles + checksum) should
	// match since they share the same password and key.
	if !bytes.Equal(innerCheck[1:], innerSet[1:]) {
		t.Errorf("tail differs: check=% X set=% X", innerCheck[1:], innerSet[1:])
	}
}

func TestBuildPasswordCommand_ChecksumIsXOROverEncodedNibbles(t *testing.T) {
	// The APK's checksum explicitly skips the cmd and key bytes
	// (see CoolledUXUtils.java:2782: "for (int i2 = 2; i2 < ...)").
	// A bug that XOR'd the key or cmd in would still type-check and
	// pass most byte-range tests, so we assert the rule directly.
	payload, err := buildPasswordCommand(CMD_SET_PASSWORD, "deadbeef", 0xC3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inner := roundTripCommand(t, payload)
	// inner = [cmd][key][n0..nN-1][checksum]
	nibbles := inner[2 : len(inner)-1]
	storedChecksum := inner[len(inner)-1]
	var want byte
	for _, b := range nibbles {
		want ^= b
	}
	if storedChecksum != want {
		t.Errorf("checksum = 0x%02X, want 0x%02X (XOR over %X)", storedChecksum, want, nibbles)
	}
}

func TestBuildPasswordCommand_RejectsNonHex(t *testing.T) {
	if _, err := BuildCheckPasswordCommand("12g4"); err == nil {
		t.Error("expected error on non-hex char")
	}
	if _, err := BuildSetPasswordCommand("hello"); err == nil {
		t.Error("expected error on non-hex chars")
	}
}

func TestBuildPasswordCommand_RejectsOutOfRangeLength(t *testing.T) {
	if _, err := BuildCheckPasswordCommand("ab"); err == nil {
		t.Error("expected error on too-short password")
	}
	long := "0123456789abcdef0123" // 20 chars
	if _, err := BuildCheckPasswordCommand(long); err == nil {
		t.Error("expected error on too-long password")
	}
}

func TestBuildPasswordCommand_UppercaseHexAccepted(t *testing.T) {
	lower, err := buildPasswordCommand(CMD_CHECK_PASSWORD, "abcd", 0x00)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	upper, err := buildPasswordCommand(CMD_CHECK_PASSWORD, "ABCD", 0x00)
	if err != nil {
		t.Fatalf("upper: %v", err)
	}
	if !bytes.Equal(roundTripCommand(t, lower), roundTripCommand(t, upper)) {
		t.Error("lowercase and uppercase hex should produce identical packets")
	}
}

func TestBuildPasswordCommand_PublicWrappersUseRandomKey(t *testing.T) {
	// Two successive calls with the same password should produce
	// different packets because the XOR key is randomized per call.
	a, err := BuildCheckPasswordCommand("1234")
	if err != nil {
		t.Fatalf("a: %v", err)
	}
	b, err := BuildCheckPasswordCommand("1234")
	if err != nil {
		t.Fatalf("b: %v", err)
	}
	// Extremely unlikely to collide: 1/256 on any given run. If this
	// ever flakes, the test body can be extended to retry once.
	if bytes.Equal(a, b) {
		t.Error("two random-key calls produced identical packets — rand broken?")
	}
}

func TestBuildTimeSyncCommand(t *testing.T) {
	// 2026-04-10 Friday 14:30:45
	now := time.Date(2026, 4, 10, 14, 30, 45, 0, time.UTC)
	frame := BuildTimeSyncCommand(now)
	payload := roundTripCommand(t, frame)

	// payload: [CMD_TIME_SYNC][year-2000][month][day][weekday_iso][hour][minute][second]
	// No CRC appended.
	if len(payload) != 8 {
		t.Fatalf("payload length = %d, want 8", len(payload))
	}
	if payload[0] != CMD_TIME_SYNC {
		t.Errorf("command code = 0x%02X, want 0x%02X (CMD_TIME_SYNC)", payload[0], CMD_TIME_SYNC)
	}
	if payload[1] != 26 { // 2026 - 2000
		t.Errorf("year = %d, want 26", payload[1])
	}
	if payload[2] != 4 { // April
		t.Errorf("month = %d, want 4", payload[2])
	}
	if payload[3] != 10 {
		t.Errorf("day = %d, want 10", payload[3])
	}
	if payload[4] != 5 { // Friday = ISO weekday 5
		t.Errorf("weekday = %d, want 5 (Friday)", payload[4])
	}
	if payload[5] != 14 {
		t.Errorf("hour = %d, want 14", payload[5])
	}
	if payload[6] != 30 {
		t.Errorf("minute = %d, want 30", payload[6])
	}
	if payload[7] != 45 {
		t.Errorf("second = %d, want 45", payload[7])
	}
}

func TestBuildTimeSyncCommand_Sunday(t *testing.T) {
	// Sunday should map to ISO weekday 7
	now := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC) // 2026-04-12 is Sunday
	frame := BuildTimeSyncCommand(now)
	payload := roundTripCommand(t, frame)

	if payload[4] != 7 {
		t.Errorf("Sunday weekday = %d, want 7", payload[4])
	}
}

func TestBuildSetTimerCommand(t *testing.T) {
	tests := []struct {
		name  string
		items []TimerItem
	}{
		{
			name: "single_item",
			items: []TimerItem{
				{Enable: true, Hour: 7, Minute: 30, Days: DayWeekdays, PowerOn: true},
			},
		},
		{
			name: "multiple_items",
			items: []TimerItem{
				{Enable: true, Hour: 7, Minute: 0, Days: DayDaily, PowerOn: true},
				{Enable: true, Hour: 22, Minute: 0, Days: DayDaily, PowerOn: false},
				{Enable: false, Hour: 9, Minute: 15, Days: DayWeekends, PowerOn: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := BuildSetTimerCommand(tt.items)
			payload := roundTripCommand(t, frame)

			if payload[0] != CMD_SET_TIMER {
				t.Errorf("command code = 0x%02X, want 0x%02X (CMD_SET_TIMER)", payload[0], CMD_SET_TIMER)
			}

			// payload: [CMD_SET_TIMER][count][per item: enable(1), hour(1), minute(1), days(1), power_on(1), 0x00]
			// No CRC appended.
			expectedLen := 1 + 1 + len(tt.items)*6
			if len(payload) != expectedLen {
				t.Fatalf("payload length = %d, want %d", len(payload), expectedLen)
			}

			if int(payload[1]) != len(tt.items) {
				t.Errorf("item count = %d, want %d", payload[1], len(tt.items))
			}

			for i, item := range tt.items {
				off := 2 + i*6
				wantEnable := byte(0x00)
				if item.Enable {
					wantEnable = 0x01
				}
				if payload[off] != wantEnable {
					t.Errorf("item[%d] enable = 0x%02X, want 0x%02X", i, payload[off], wantEnable)
				}
				if payload[off+1] != item.Hour {
					t.Errorf("item[%d] hour = %d, want %d", i, payload[off+1], item.Hour)
				}
				if payload[off+2] != item.Minute {
					t.Errorf("item[%d] minute = %d, want %d", i, payload[off+2], item.Minute)
				}
				if payload[off+3] != item.Days {
					t.Errorf("item[%d] days = 0x%02X, want 0x%02X", i, payload[off+3], item.Days)
				}
				wantPowerOn := byte(0x00)
				if item.PowerOn {
					wantPowerOn = 0x01
				}
				if payload[off+4] != wantPowerOn {
					t.Errorf("item[%d] power_on = 0x%02X, want 0x%02X", i, payload[off+4], wantPowerOn)
				}
				if payload[off+5] != 0x00 {
					t.Errorf("item[%d] padding = 0x%02X, want 0x00", i, payload[off+5])
				}
			}
		})
	}
}

func TestBuildGetTimerCommand(t *testing.T) {
	frame := BuildGetTimerCommand()
	payload := roundTripCommand(t, frame)

	// payload: [CMD_GET_TIMER] only, no CRC
	if len(payload) != 1 {
		t.Fatalf("payload length = %d, want 1", len(payload))
	}
	if payload[0] != CMD_GET_TIMER {
		t.Errorf("command code = 0x%02X, want 0x%02X (CMD_GET_TIMER)", payload[0], CMD_GET_TIMER)
	}
}

func TestBuildDeviceInfoCommand(t *testing.T) {
	frame := BuildDeviceInfoCommand()
	payload := roundTripCommand(t, frame)

	// Device info uses BuildStreamFrame directly (no CRC), so payload is just [0x1F].
	if len(payload) != 1 {
		t.Fatalf("payload length = %d, want 1", len(payload))
	}
	if payload[0] != CMD_DEVICE_INFO {
		t.Errorf("command code = 0x%02X, want 0x%02X (CMD_DEVICE_INFO)", payload[0], CMD_DEVICE_INFO)
	}
}

func TestBuildColorCommand(t *testing.T) {
	tests := []struct {
		name       string
		r, g, b    uint8
		wantR4     uint8 // nibble
		wantG4     uint8
		wantB4     uint8
	}{
		{"black", 0, 0, 0, 0, 0, 0},
		{"white", 255, 255, 255, 15, 15, 15},
		{"pure_red", 255, 0, 0, 15, 0, 0},
		{"pure_green", 0, 255, 0, 0, 15, 0},
		{"pure_blue", 0, 0, 255, 0, 0, 15},
		{"near_white_boundary", 238, 238, 238, 15, 15, 15},
		{"at_black_threshold", 47, 47, 47, 0, 0, 0}, // v<=47 -> 0
		{"just_above_threshold", 48, 48, 48, 1, 1, 1},
		{"mid", 128, 128, 128, 6, 6, 6}, // (128-47)/14+1 = 6
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := BuildColorCommand(tt.r, tt.g, tt.b)
			payload := roundTripCommand(t, frame)

			// payload: [CMD_COLOR][COLOR_SUBTYPE_SINGLE][0R][GB][CRC32 x4]
			if len(payload) != 8 {
				t.Fatalf("payload length = %d, want 8", len(payload))
			}
			if payload[0] != CMD_COLOR {
				t.Errorf("command code = 0x%02X, want 0x%02X", payload[0], CMD_COLOR)
			}
			if payload[1] != COLOR_SUBTYPE_SINGLE {
				t.Errorf("subtype = 0x%02X, want 0x%02X", payload[1], COLOR_SUBTYPE_SINGLE)
			}
			if payload[2] != tt.wantR4 {
				t.Errorf("R byte = 0x%02X, want 0x%02X", payload[2], tt.wantR4)
			}
			if got := (payload[3] >> 4) & 0x0F; got != tt.wantG4 {
				t.Errorf("G nibble = 0x%X, want 0x%X", got, tt.wantG4)
			}
			if got := payload[3] & 0x0F; got != tt.wantB4 {
				t.Errorf("B nibble = 0x%X, want 0x%X", got, tt.wantB4)
			}

			verifyCRC(t, payload)
		})
	}
}

func TestBuildColorCommandMatchesAPKPattern(t *testing.T) {
	// White (#FFFFFF) produces [0x0F, 0xFF] per the APK's two-byte RGB444 layout.
	frame := BuildColorCommand(255, 255, 255)
	payload := roundTripCommand(t, frame)
	want := []byte{CMD_COLOR, COLOR_SUBTYPE_SINGLE, 0x0F, 0xFF}
	if !bytes.Equal(payload[:4], want) {
		t.Errorf("header = % X, want % X", payload[:4], want)
	}
}

func TestBuildChannelCommand(t *testing.T) {
	tests := []struct {
		name    string
		channel uint8
	}{
		{"slot 0", 0},
		{"slot 1", 1},
		{"slot 8", 8},
		{"max", 255},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := BuildChannelCommand(tt.channel)
			payload := roundTripCommand(t, frame)

			// payload: [CMD_CHANNEL][channel][CRC32 x4]
			if len(payload) != 6 {
				t.Fatalf("payload length = %d, want 6", len(payload))
			}
			if payload[0] != CMD_CHANNEL {
				t.Errorf("command code = 0x%02X, want 0x%02X (CMD_CHANNEL)", payload[0], CMD_CHANNEL)
			}
			if payload[1] != tt.channel {
				t.Errorf("channel = %d, want %d", payload[1], tt.channel)
			}
			verifyCRC(t, payload)
		})
	}
}

func TestBuildCountdownStatusCommand(t *testing.T) {
	payload := roundTripCommand(t, BuildCountdownStatusCommand())
	want := []byte{CMD_COUNTDOWN, COUNTDOWN_SUBTYPE_STATUS}
	if !bytes.Equal(payload, want) {
		t.Errorf("got % X, want % X", payload, want)
	}
}

func TestBuildCountdownSetCommand(t *testing.T) {
	payload := roundTripCommand(t, BuildCountdownSetCommand(1, 30, 45))
	want := []byte{CMD_COUNTDOWN, COUNTDOWN_SUBTYPE_SET, 1, 30, 45}
	if !bytes.Equal(payload, want) {
		t.Errorf("got % X, want % X", payload, want)
	}
}

func TestBuildCountdownStartStopCommand(t *testing.T) {
	tests := []struct {
		name  string
		start bool
		flag  byte
	}{
		{"start", true, 0x01},
		{"stop", false, 0x00},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := roundTripCommand(t, BuildCountdownStartStopCommand(tt.start))
			want := []byte{CMD_COUNTDOWN, COUNTDOWN_SUBTYPE_START_STOP, tt.flag}
			if !bytes.Equal(payload, want) {
				t.Errorf("got % X, want % X", payload, want)
			}
		})
	}
}

func TestBuildStopwatchStatusCommand(t *testing.T) {
	payload := roundTripCommand(t, BuildStopwatchStatusCommand())
	want := []byte{CMD_STOPWATCH, STOPWATCH_SUBTYPE_STATUS}
	if !bytes.Equal(payload, want) {
		t.Errorf("got % X, want % X", payload, want)
	}
}

func TestBuildStopwatchResetCommand(t *testing.T) {
	payload := roundTripCommand(t, BuildStopwatchResetCommand())
	want := []byte{CMD_STOPWATCH, STOPWATCH_SUBTYPE_RESET}
	if !bytes.Equal(payload, want) {
		t.Errorf("got % X, want % X", payload, want)
	}
}

func TestBuildStopwatchStartStopCommand(t *testing.T) {
	tests := []struct {
		name  string
		start bool
		flag  byte
	}{
		{"start", true, 0x01},
		{"stop", false, 0x00},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := roundTripCommand(t, BuildStopwatchStartStopCommand(tt.start))
			want := []byte{CMD_STOPWATCH, STOPWATCH_SUBTYPE_START_STOP, tt.flag}
			if !bytes.Equal(payload, want) {
				t.Errorf("got % X, want % X", payload, want)
			}
		})
	}
}

func TestBuildScoreboardStatusCommand(t *testing.T) {
	payload := roundTripCommand(t, BuildScoreboardStatusCommand())
	want := []byte{CMD_SCOREBOARD, SCOREBOARD_SUBTYPE_STATUS}
	if !bytes.Equal(payload, want) {
		t.Errorf("got % X, want % X", payload, want)
	}
}

func TestBuildScoreboardSetScoresCommand(t *testing.T) {
	// scoreA=0x0123 -> [0x01, 0x23]; scoreB=0x00FF -> [0x00, 0xFF]; totals as uint8s.
	payload := roundTripCommand(t, BuildScoreboardSetScoresCommand(0x0123, 0x00FF, 3, 2))
	want := []byte{
		CMD_SCOREBOARD, SCOREBOARD_SUBTYPE_SET_SCORES,
		0x01, 0x23,
		0x00, 0xFF,
		3, 2,
	}
	if !bytes.Equal(payload, want) {
		t.Errorf("got % X, want % X", payload, want)
	}
}

func TestBuildScoreboardSetTimeCommand(t *testing.T) {
	tests := []struct {
		name    string
		hour    uint8
		minute  uint8
		isTimer bool
		flag    byte
	}{
		{"wallclock", 14, 30, false, 0x00},
		{"timer", 0, 0, true, 0x01},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := roundTripCommand(t, BuildScoreboardSetTimeCommand(tt.hour, tt.minute, tt.isTimer))
			want := []byte{
				CMD_SCOREBOARD, SCOREBOARD_SUBTYPE_SET_TIME,
				tt.hour, tt.minute, tt.flag,
			}
			if !bytes.Equal(payload, want) {
				t.Errorf("got % X, want % X", payload, want)
			}
		})
	}
}

func TestBuildScoreboardStartStopCommand(t *testing.T) {
	tests := []struct {
		name  string
		start bool
		flag  byte
	}{
		{"start", true, 0x01},
		{"stop", false, 0x00},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := roundTripCommand(t, BuildScoreboardStartStopCommand(tt.start))
			want := []byte{CMD_SCOREBOARD, SCOREBOARD_SUBTYPE_START_STOP, tt.flag}
			if !bytes.Equal(payload, want) {
				t.Errorf("got % X, want % X", payload, want)
			}
		})
	}
}

func TestBuildSetDeviceInfoCommand(t *testing.T) {
	tests := []struct {
		name    string
		subtype byte
		on      bool
		want    []byte
	}{
		{"show_id on", DEVICE_INFO_SUBTYPE_SHOW_ID, true, []byte{CMD_SET_DEVICE_INFO, 0x01, 0x01}},
		{"show_id off", DEVICE_INFO_SUBTYPE_SHOW_ID, false, []byte{CMD_SET_DEVICE_INFO, 0x01, 0x00}},
		{"remote on", DEVICE_INFO_SUBTYPE_REMOTE, true, []byte{CMD_SET_DEVICE_INFO, 0x02, 0x01}},
		{"remote off", DEVICE_INFO_SUBTYPE_REMOTE, false, []byte{CMD_SET_DEVICE_INFO, 0x02, 0x00}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := BuildSetDeviceInfoCommand(tt.subtype, tt.on)
			payload := roundTripCommand(t, frame)
			if !bytes.Equal(payload, tt.want) {
				t.Errorf("got % X, want % X", payload, tt.want)
			}
		})
	}
}
