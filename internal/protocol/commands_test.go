package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
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

func TestBuildTimeCommand(t *testing.T) {
	tests := []struct {
		name                    string
		hour, minute, second    uint8
	}{
		{"midnight", 0, 0, 0},
		{"noon", 12, 0, 0},
		{"late_evening", 23, 59, 59},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := BuildTimeCommand(tt.hour, tt.minute, tt.second)
			payload := roundTripCommand(t, frame)

			// payload: [CMD_TIME][hour][minute][second][CRC32 x4]
			if len(payload) != 8 {
				t.Fatalf("payload length = %d, want 8", len(payload))
			}
			if payload[0] != CMD_TIME {
				t.Errorf("command code = 0x%02X, want 0x%02X", payload[0], CMD_TIME)
			}
			if payload[1] != tt.hour || payload[2] != tt.minute || payload[3] != tt.second {
				t.Errorf("time = %d:%d:%d, want %d:%d:%d",
					payload[1], payload[2], payload[3],
					tt.hour, tt.minute, tt.second)
			}

			verifyCRC(t, payload)
		})
	}
}

func TestBuildTimerCommand(t *testing.T) {
	tests := []struct {
		name  string
		items []TimerItem
	}{
		{
			name: "single_item",
			items: []TimerItem{
				{Hour: 7, Minute: 30, On: true, Days: DayWeekdays},
			},
		},
		{
			name: "multiple_items",
			items: []TimerItem{
				{Hour: 7, Minute: 0, On: true, Days: DayDaily},
				{Hour: 22, Minute: 0, On: false, Days: DayDaily},
				{Hour: 9, Minute: 15, On: true, Days: DayWeekends},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := BuildTimerCommand(tt.items)
			payload := roundTripCommand(t, frame)

			if payload[0] != CMD_TIMER {
				t.Errorf("command code = 0x%02X, want 0x%02X", payload[0], CMD_TIMER)
			}

			// payload: [CMD_TIMER][count][items...][CRC32 x4]
			// Each item is 4 bytes: hour, minute, on/off, days
			expectedLen := 1 + 1 + len(tt.items)*4 + 4
			if len(payload) != expectedLen {
				t.Fatalf("payload length = %d, want %d", len(payload), expectedLen)
			}

			if int(payload[1]) != len(tt.items) {
				t.Errorf("item count = %d, want %d", payload[1], len(tt.items))
			}

			for i, item := range tt.items {
				off := 2 + i*4
				if payload[off] != item.Hour {
					t.Errorf("item[%d] hour = %d, want %d", i, payload[off], item.Hour)
				}
				if payload[off+1] != item.Minute {
					t.Errorf("item[%d] minute = %d, want %d", i, payload[off+1], item.Minute)
				}
				wantOn := byte(0x00)
				if item.On {
					wantOn = 0x01
				}
				if payload[off+2] != wantOn {
					t.Errorf("item[%d] on = 0x%02X, want 0x%02X", i, payload[off+2], wantOn)
				}
				if payload[off+3] != item.Days {
					t.Errorf("item[%d] days = 0x%02X, want 0x%02X", i, payload[off+3], item.Days)
				}
			}

			verifyCRC(t, payload)
		})
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
