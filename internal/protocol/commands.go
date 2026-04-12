package protocol

import "time"

// TimerItem represents a single timer schedule entry.
// Verified against APK com.jtkj.led1248 decompilation.
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

// BuildPasswordCommand builds a framed password verify or set command.
// verify=true uses CMD_CHECK_PASSWORD (0x0D), verify=false uses CMD_SET_PASSWORD (0x0E).
func BuildPasswordCommand(password string, verify bool) []byte {
	cmdCode := CMD_SET_PASSWORD
	op := PasswordOpSet
	if verify {
		cmdCode = CMD_CHECK_PASSWORD
		op = PasswordOpVerify
	}
	data := make([]byte, 1+len(password))
	data[0] = op
	copy(data[1:], []byte(password))
	return buildControlCommand(cmdCode, data)
}

// BuildTimeSyncCommand builds a framed time-sync command matching the APK format.
// The APK sends: stream_frame([0x09][year-2000][month][day][weekday_iso][hour][minute][second])
// No CRC is appended (the APK never uses CRC for this command, verified on hardware).
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

// BuildSetTimerCommand builds a framed set-timer command matching the APK format.
// The APK sends: stream_frame([0x0A][count][per item: enable(1), hour(1), minute(1), days(1), power_on(1), 0x00])
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
// The APK sends: stream_frame([0x0B]) to read timer slots from the device.
func BuildGetTimerCommand() []byte {
	return BuildStreamFrame([]byte{CMD_GET_TIMER})
}

// BuildDeviceInfoCommand builds a framed device info request (0x1F).
// The APK uses 0x1F for device info (not 0x0D which is password check).
func BuildDeviceInfoCommand() []byte {
	return BuildStreamFrame([]byte{CMD_DEVICE_INFO})
}
