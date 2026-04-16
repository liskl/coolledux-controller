package protocol

import (
	"crypto/rand"
	"fmt"
	"time"
)

// TimerItem represents a single timer schedule entry.
// Verified against CoolLED 1248 Android app.
type TimerItem struct {
	Enable  bool  // whether this timer slot is active
	Hour    uint8
	Minute  uint8
	Days    uint8 // bitmask: Mon=1, Tue=2, Wed=4, Thu=8, Fri=16, Sat=32, Sun=64. 0=never
	PowerOn bool  // true=turn on at scheduled time, false=turn off
}

// buildControlCommand assembles a control command: [cmd][data][CRC32_LE],
// then wraps it with stream framing [0x01][len_BE][escaped][0x03].
//
// The Python SDK (reference implementation) sends simple commands this way:
// raw packet bytes wrapped in SendDataUtils stream framing, WITHOUT the
// [0x52,0x52] BLE packet header. The 0x5252 header is only used for
// program upload packets.
func buildControlCommand(cmdCode byte, data []byte) []byte {
	inner := make([]byte, 1+len(data))
	inner[0] = cmdCode
	copy(inner[1:], data)

	withCRC := AppendCRC(inner)
	return BuildStreamFrame(withCRC)
}

// BuildPowerCommand builds a framed power on/off command.
func BuildPowerCommand(on bool) []byte {
	b := PowerOff
	if on {
		b = PowerOn
	}
	return buildControlCommand(CMD_POWER, []byte{b})
}

// BuildBrightnessCommand builds a framed brightness command.
func BuildBrightnessCommand(brightness uint8) []byte {
	return buildControlCommand(CMD_BRIGHTNESS, []byte{brightness})
}

// BuildFlipCommand builds a framed flip/mirror command.
// mode: 0=none, 1=horizontal, 2=vertical, 3=both.
func BuildFlipCommand(mode uint8) []byte {
	return buildControlCommand(CMD_FLIP, []byte{mode})
}

// BuildChannelCommand builds a framed program/channel switch command.
func BuildChannelCommand(channel uint8) []byte {
	return buildControlCommand(CMD_CHANNEL, []byte{channel})
}

// BuildColorCommand builds a framed single-color control command (0x13 / 0x01).
// The 8-bit RGB triple is converted to RGB444 using the device's piecewise
// transfer function and packed into two bytes as [0x0R, 0xGB].
//
// Matches the CoolLED 1248 Android app's setColor(color) call; observed on
// real hardware to change the global color used to tint monochrome content.
func BuildColorCommand(r, g, b uint8) []byte {
	r4 := rgb444Transfer(r)
	g4 := rgb444Transfer(g)
	b4 := rgb444Transfer(b)
	return buildControlCommand(CMD_COLOR, []byte{
		COLOR_SUBTYPE_SINGLE,
		r4,
		(g4 << 4) | b4,
	})
}

// rgb444Transfer is the device's piecewise 8-bit to 4-bit color mapping.
// Duplicated here to avoid a dependency on the image package from protocol.
// Source: TextEmojiManagerCoolLEDUX.rgb444Transfer in the APK decompilation.
func rgb444Transfer(v uint8) uint8 {
	if v >= 238 {
		return 15
	}
	if v <= 47 {
		return 0
	}
	return uint8((int(v)-47)/14 + 1)
}

// BuildCheckPasswordCommand builds a framed 0x0D password-verify command.
// See buildPasswordCommand for the packet layout and password encoding.
func BuildCheckPasswordCommand(password string) ([]byte, error) {
	return buildPasswordCommand(CMD_CHECK_PASSWORD, password, randomByte())
}

// BuildSetPasswordCommand builds a framed 0x0E password-set command.
func BuildSetPasswordCommand(password string) ([]byte, error) {
	return buildPasswordCommand(CMD_SET_PASSWORD, password, randomByte())
}

// buildPasswordCommand encodes either 0x0D (check) or 0x0E (set) per the
// APK's getCheckPasswordData / getSetPasswordData at CoolledUXUtils.java:2770
// and :4438. Packet layout after the stream frame:
//
//	[cmd][xorKey][nibble[0]^xorKey][nibble[1]^xorKey]...[xorChecksum]
//
// where each password character is interpreted as a single hex digit
// (0-9, a-f, case-insensitive) and therefore a 4-bit nibble. The
// xorChecksum is the XOR of every byte from the first encoded nibble
// through the last; the cmd byte and the xorKey byte are excluded.
//
// The xorKey is randomized per call — tests inject a fixed key via this
// unexported entry point for byte-exact assertions.
func buildPasswordCommand(cmdCode byte, password string, xorKey byte) ([]byte, error) {
	if len(password) < PasswordMinLen || len(password) > PasswordMaxLen {
		return nil, fmt.Errorf("password length %d outside [%d..%d]",
			len(password), PasswordMinLen, PasswordMaxLen)
	}
	nibbles := make([]byte, len(password))
	for i, c := range password {
		n, ok := hexNibble(c)
		if !ok {
			return nil, fmt.Errorf("password char %q at %d: want 0-9/a-f/A-F", c, i)
		}
		nibbles[i] = n ^ xorKey
	}
	var checksum byte
	for _, b := range nibbles {
		checksum ^= b
	}

	out := make([]byte, 0, 3+len(nibbles))
	out = append(out, cmdCode, xorKey)
	out = append(out, nibbles...)
	out = append(out, checksum)
	return BuildStreamFrame(out), nil
}

// hexNibble returns the 4-bit value of a hex-digit rune; ok reports
// whether the rune was a valid hex digit.
func hexNibble(r rune) (byte, bool) {
	switch {
	case r >= '0' && r <= '9':
		return byte(r - '0'), true
	case r >= 'a' && r <= 'f':
		return byte(r-'a') + 10, true
	case r >= 'A' && r <= 'F':
		return byte(r-'A') + 10, true
	}
	return 0, false
}

// randomByte reads one cryptographically random byte. crypto/rand read
// errors are fatal in Go — they only surface if the OS runs out of
// entropy, which is effectively impossible on Linux.
func randomByte() byte {
	var b [1]byte
	_, _ = rand.Read(b[:])
	return b[0]
}

// BuildTimeSyncCommand builds a framed time-sync command matching the Android app format.
// Format: stream_frame([0x09][year-2000][month][day][weekday_iso][hour][minute][second])
// No CRC is appended (no CRC, verified on hardware).
func BuildTimeSyncCommand(now time.Time) []byte {
	inner := []byte{
		CMD_TIME_SYNC,
		byte(now.Year() - 2000),
		byte(now.Month()),
		byte(now.Day()),
		isoWeekday(now.Weekday()),
		byte(now.Hour()),
		byte(now.Minute()),
		byte(now.Second()),
	}
	return BuildStreamFrame(inner)
}

// isoWeekday converts Go's time.Weekday (Sun=0..Sat=6) to ISO 8601 (Mon=1..Sun=7).
func isoWeekday(wd time.Weekday) byte {
	if wd == time.Sunday {
		return 7
	}
	return byte(wd)
}

// BuildSetTimerCommand builds a framed set-timer command matching the Android app format.
// Format: stream_frame([0x0A][count][per item: enable(1), hour(1), minute(1), days(1), power_on(1), 0x00])
// No CRC is appended (verified on hardware).
func BuildSetTimerCommand(items []TimerItem) []byte {
	// Each item is 6 bytes: enable, hour, minute, days, power_on, 0x00
	inner := make([]byte, 1+1+len(items)*6)
	inner[0] = CMD_SET_TIMER
	inner[1] = byte(len(items))
	for i, item := range items {
		off := 2 + i*6
		if item.Enable {
			inner[off] = 0x01
		}
		inner[off+1] = item.Hour
		inner[off+2] = item.Minute
		inner[off+3] = item.Days
		if item.PowerOn {
			inner[off+4] = 0x01
		}
		inner[off+5] = 0x00
	}
	return BuildStreamFrame(inner)
}

// BuildGetTimerCommand builds a framed get-timer command.
// Format: stream_frame([0x0B]) to read timer slots from the device.
func BuildGetTimerCommand() []byte {
	return BuildStreamFrame([]byte{CMD_GET_TIMER})
}

// BuildDeviceInfoCommand builds a framed device info request (0x1F).
// The Android app uses 0x1F for device info (not 0x0D which is password check).
func BuildDeviceInfoCommand() []byte {
	return BuildStreamFrame([]byte{CMD_DEVICE_INFO})
}

// BuildSetDeviceInfoCommand toggles one of the panel's boolean settings
// (show device ID, remote enable). Matches setDeviceInfo at
// CoolledUXUtils.java:4843 — stream-framed, no CRC. Subtype selects
// which setting; `on` is the new value.
func BuildSetDeviceInfoCommand(subtype byte, on bool) []byte {
	flag := byte(0x00)
	if on {
		flag = 0x01
	}
	return BuildStreamFrame([]byte{CMD_SET_DEVICE_INFO, subtype, flag})
}

// --- Countdown timer overlay (CMD_COUNTDOWN, 0x0F) ---

// BuildCountdownStatusCommand requests the current countdown state.
func BuildCountdownStatusCommand() []byte {
	return BuildStreamFrame([]byte{CMD_COUNTDOWN, COUNTDOWN_SUBTYPE_STATUS})
}

// BuildCountdownSetCommand sets the countdown duration. Each field is a
// single byte per the APK (getCountDownReset uses getHexListStringForInt).
func BuildCountdownSetCommand(hour, minute, second uint8) []byte {
	return BuildStreamFrame([]byte{
		CMD_COUNTDOWN, COUNTDOWN_SUBTYPE_SET,
		hour, minute, second,
	})
}

// BuildCountdownStartStopCommand starts (true) or stops (false) the countdown.
func BuildCountdownStartStopCommand(start bool) []byte {
	flag := byte(0x00)
	if start {
		flag = 0x01
	}
	return BuildStreamFrame([]byte{CMD_COUNTDOWN, COUNTDOWN_SUBTYPE_START_STOP, flag})
}

// --- Stopwatch overlay (CMD_STOPWATCH, 0x10) ---

// BuildStopwatchStatusCommand requests the current stopwatch state.
func BuildStopwatchStatusCommand() []byte {
	return BuildStreamFrame([]byte{CMD_STOPWATCH, STOPWATCH_SUBTYPE_STATUS})
}

// BuildStopwatchResetCommand resets the stopwatch to 00:00:00.
func BuildStopwatchResetCommand() []byte {
	return BuildStreamFrame([]byte{CMD_STOPWATCH, STOPWATCH_SUBTYPE_RESET})
}

// BuildStopwatchStartStopCommand starts (true) or stops (false) the stopwatch.
func BuildStopwatchStartStopCommand(start bool) []byte {
	flag := byte(0x00)
	if start {
		flag = 0x01
	}
	return BuildStreamFrame([]byte{CMD_STOPWATCH, STOPWATCH_SUBTYPE_START_STOP, flag})
}

// --- Scoreboard overlay (CMD_SCOREBOARD, 0x11) ---
//
// NOTE: the 16x96 firmware ACKs scoreboard packets but does not produce a
// visible scoreboard. The builders are exposed for completeness; behavior on
// other CoolLEDUX models may differ.

// BuildScoreboardStatusCommand requests the current scoreboard state.
func BuildScoreboardStatusCommand() []byte {
	return BuildStreamFrame([]byte{CMD_SCOREBOARD, SCOREBOARD_SUBTYPE_STATUS})
}

// BuildScoreboardSetScoresCommand sets the two team main scores and their
// period/set counters (displayed as the small digits above each main score
// on 16x96). Per the APK (CoolledUXUtils.getScoreBoardSetCore), the layout
// is scoreA:2 BE, scoreB:2 BE, totalA:1, totalB:1 — i.e. main scores are
// uint16 and period counters are uint8. DeviceManager.java:7155 confirms
// the arg order: (hostScore, visitScore, hostTotalScore, visitTotalScore).
func BuildScoreboardSetScoresCommand(scoreA, scoreB uint16, totalA, totalB uint8) []byte {
	return BuildStreamFrame([]byte{
		CMD_SCOREBOARD, SCOREBOARD_SUBTYPE_SET_SCORES,
		byte(scoreA >> 8), byte(scoreA),
		byte(scoreB >> 8), byte(scoreB),
		totalA, totalB,
	})
}

// BuildScoreboardSetTimeCommand sets the scoreboard's clock (hour, minute) and
// whether it counts up as a timer (isTimer=true) or shows wall time.
func BuildScoreboardSetTimeCommand(hour, minute uint8, isTimer bool) []byte {
	flag := byte(0x00)
	if isTimer {
		flag = 0x01
	}
	return BuildStreamFrame([]byte{
		CMD_SCOREBOARD, SCOREBOARD_SUBTYPE_SET_TIME,
		hour, minute, flag,
	})
}

// BuildScoreboardStartStopCommand starts (true) or stops (false) the scoreboard.
func BuildScoreboardStartStopCommand(start bool) []byte {
	flag := byte(0x00)
	if start {
		flag = 0x01
	}
	return BuildStreamFrame([]byte{CMD_SCOREBOARD, SCOREBOARD_SUBTYPE_START_STOP, flag})
}
