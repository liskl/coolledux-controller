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

func TestBuildPasswordCommand(t *testing.T) {
	tests := []struct {
		name     string
		password string
		verify   bool
		wantOp   byte
	}{
		{"verify", "1234", true, PasswordOpVerify},
		{"set", "abcdef", false, PasswordOpSet},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := BuildPasswordCommand(tt.password, tt.verify)
			payload := roundTripCommand(t, frame)

			if payload[0] != CMD_PASSWORD {
				t.Errorf("command code = 0x%02X, want 0x%02X", payload[0], CMD_PASSWORD)
			}
			if payload[1] != tt.wantOp {
				t.Errorf("op = 0x%02X, want 0x%02X", payload[1], tt.wantOp)
			}

			// The password bytes follow the op byte, before the CRC.
			pwBytes := payload[2 : len(payload)-4]
			if !bytes.Equal(pwBytes, []byte(tt.password)) {
				t.Errorf("password bytes = %q, want %q", pwBytes, tt.password)
			}

			verifyCRC(t, payload)
		})
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

func TestBuildInfoCommand(t *testing.T) {
	frame := BuildInfoCommand()
	payload := roundTripCommand(t, frame)

	// payload: [CMD_INFO][CRC32 x4]
	if len(payload) != 5 {
		t.Fatalf("payload length = %d, want 5", len(payload))
	}
	if payload[0] != CMD_INFO {
		t.Errorf("command code = 0x%02X, want 0x%02X", payload[0], CMD_INFO)
	}

	verifyCRC(t, payload)
}

func TestBuildResetCommand(t *testing.T) {
	frame := BuildResetCommand()
	payload := roundTripCommand(t, frame)

	// payload: [CMD_RESET][CRC32 x4]
	if len(payload) != 5 {
		t.Fatalf("payload length = %d, want 5", len(payload))
	}
	if payload[0] != CMD_RESET {
		t.Errorf("command code = 0x%02X, want 0x%02X", payload[0], CMD_RESET)
	}

	verifyCRC(t, payload)
}
