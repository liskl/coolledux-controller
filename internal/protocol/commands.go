package protocol

// TimerItem represents a single timer schedule entry.
type TimerItem struct {
	Hour   uint8
	Minute uint8
	On     bool
	Days   uint8
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
// verify=true sends PasswordOpVerify (0x01), verify=false sends PasswordOpSet (0x00).
func BuildPasswordCommand(password string, verify bool) []byte {
	op := PasswordOpSet
	if verify {
		op = PasswordOpVerify
	}
	data := make([]byte, 1+len(password))
	data[0] = op
	copy(data[1:], []byte(password))
	return buildControlCommand(CMD_PASSWORD, data)
}

// BuildTimeCommand builds a framed time-set command.
func BuildTimeCommand(hour, minute, second uint8) []byte {
	return buildControlCommand(CMD_TIME, []byte{hour, minute, second})
}

// BuildTimerCommand builds a framed timer schedule command.
func BuildTimerCommand(items []TimerItem) []byte {
	data := make([]byte, 1+len(items)*4)
	data[0] = byte(len(items))
	for i, item := range items {
		off := 1 + i*4
		data[off] = item.Hour
		data[off+1] = item.Minute
		if item.On {
			data[off+2] = 0x01
		} else {
			data[off+2] = 0x00
		}
		data[off+3] = item.Days
	}
	return buildControlCommand(CMD_TIMER, data)
}

// BuildInfoCommand builds a framed device info request (no data payload).
func BuildInfoCommand() []byte {
	return buildControlCommand(CMD_INFO, nil)
}

// BuildResetCommand builds a framed device reset command (no data payload).
func BuildResetCommand() []byte {
	return buildControlCommand(CMD_RESET, nil)
}
