package protocol

import "time"

// Packet header bytes.
var PacketHeader = [2]byte{0x52, 0x52}

// Command codes (host to device).
// Verified against the CoolLED 1248 Android app and real hardware.
const (
	CMD_BRIGHTNESS byte = 0x04 // Verified: device uses 0x04 (SDK docs say 0x06)
	CMD_POWER      byte = 0x05 // Verified: matches SDK
	CMD_CHANNEL    byte = 0x07 // Switches program/channel slot (SDK calls this "flip" but it isn't)
	CMD_PROGRAM    byte = 0x08
	CMD_TIME_SYNC  byte = 0x09 // Verified: year/month/day/weekday/h/m/s (no CRC)
	CMD_SET_TIMER  byte = 0x0A // Verified: enable/hour/min/days/power_on/0x00 per slot (no CRC)
	CMD_GET_TIMER  byte = 0x0B // Verified: bare command to read timer slots back
	CMD_FLIP       byte = 0x0C // Verified: 0=none, 1=horizontal, 2=vertical, 3=both (SDK says 0x07)
	CMD_CHECK_PASSWORD byte = 0x0D // Password check with XOR encoding
	CMD_SET_PASSWORD   byte = 0x0E // Password set with XOR encoding
	CMD_COUNTDOWN      byte = 0x0F // Countdown timer overlay. Subtype 0x01=status, 0x02=set, 0x03=start/stop.
	CMD_STOPWATCH      byte = 0x10 // Stopwatch overlay. Subtype 0x01=status, 0x02=reset, 0x03=start/stop.
	CMD_SCOREBOARD     byte = 0x11 // Scoreboard overlay. Subtype 0x01=status, 0x02=set scores, 0x03=set time, 0x04=start/stop.
	CMD_COLOR          byte = 0x13 // Global color control. Subtype 0x01=single color, 0x02=speed, 0x03=mode.
	CMD_SET_DEVICE_INFO byte = 0x1E // Toggle booleans (show-device-id, remote-enable) on the panel.
	CMD_DEVICE_INFO    byte = 0x1F // Verified: 0x1F (not 0x0D)
)

// Subtype bytes for the overlay commands above.
const (
	COUNTDOWN_SUBTYPE_STATUS     byte = 0x01
	COUNTDOWN_SUBTYPE_SET        byte = 0x02
	COUNTDOWN_SUBTYPE_START_STOP byte = 0x03

	STOPWATCH_SUBTYPE_STATUS     byte = 0x01
	STOPWATCH_SUBTYPE_RESET      byte = 0x02
	STOPWATCH_SUBTYPE_START_STOP byte = 0x03

	SCOREBOARD_SUBTYPE_STATUS     byte = 0x01
	SCOREBOARD_SUBTYPE_SET_SCORES byte = 0x02
	SCOREBOARD_SUBTYPE_SET_TIME   byte = 0x03
	SCOREBOARD_SUBTYPE_START_STOP byte = 0x04

	// 0x1E setDeviceInfo subtypes. Values 1 and 2 are wired to checkboxes
	// in SettingsCoolleduxFragment; value 3 is for a different product
	// line (iLedClock). See CoolledUXUtils.java:4843.
	DEVICE_INFO_SUBTYPE_SHOW_ID byte = 0x01
	DEVICE_INFO_SUBTYPE_REMOTE  byte = 0x02
)

// Color command subtypes (second byte after CMD_COLOR).
const (
	COLOR_SUBTYPE_SINGLE byte = 0x01
	COLOR_SUBTYPE_SPEED  byte = 0x02
	COLOR_SUBTYPE_MODE   byte = 0x03
)

// Command types (Layer 1 BLE packet frame).
const (
	CMD_TYPE_PROGRAM byte = 0x00
	CMD_TYPE_CONTROL byte = 0x02
)

// Command subtypes (Layer 1 BLE packet frame).
const (
	CMD_SUBTYPE_DATA_TRANSMISSION byte = 0x01
	CMD_SUBTYPE_CONTROL           byte = 0x02
	CMD_SUBTYPE_DATA_PACKET       byte = 0x03
)

// Response types (device to host).
const (
	RESPONSE_TYPE_PROGRAM_START   byte = 0x02
	RESPONSE_TYPE_PROGRAM_DATA    byte = 0x03
	RESPONSE_TYPE_BRIGHTNESS      byte = 0x04
	RESPONSE_TYPE_POWER           byte = 0x05
	RESPONSE_TYPE_CHANNEL         byte = 0x07
	RESPONSE_TYPE_TIME_SYNC       byte = 0x09
	RESPONSE_TYPE_SET_TIMER       byte = 0x0A
	RESPONSE_TYPE_GET_TIMER       byte = 0x0B
	RESPONSE_TYPE_FLIP            byte = 0x0C
	RESPONSE_TYPE_PASSWORD_VERIFY byte = 0x0D
	RESPONSE_TYPE_PASSWORD_SET    byte = 0x0E
	RESPONSE_TYPE_COLOR           byte = 0x13
	RESPONSE_TYPE_DEVICE_INFO     byte = 0x1F
)

// Response status codes.
const (
	STATUS_SUCCESS byte = 0x00
	STATUS_ERROR   byte = 0x01
	STATUS_TIMEOUT byte = 0x02
	STATUS_INVALID byte = 0x03
)

// Error codes.
const (
	ERR_INVALID_COMMAND    byte = 0x01
	ERR_INVALID_DATA       byte = 0x02
	ERR_CHECKSUM_FAILED    byte = 0x03
	ERR_DEVICE_BUSY        byte = 0x04
	ERR_MEMORY_FULL        byte = 0x05
	ERR_TIMEOUT            byte = 0x06
	ERR_PASSWORD_INCORRECT byte = 0x07
	ERR_DEVICE_LOCKED      byte = 0x08
	ERR_UNSUPPORTED        byte = 0x09
	ERR_UNKNOWN            byte = 0xFF
)

// Stream framing (Layer 2).
const (
	START_BYTE  byte = 0x01
	END_BYTE    byte = 0x03
	ESCAPE_BYTE byte = 0x02
	XOR_MASK    byte = 0x04
)

// Protocol limits.
const (
	MaxPacketSize     = 4096
	MinPacketSize     = 8
	MaxProgramSize    = 65536
	MaxTextLength     = 256
	MaxAnimFrames     = 100
	MaxTimerItems     = 10
	BrightnessMax     = 255
	SpeedMin          = 1
	SpeedMax          = 10
	PasswordMinLen    = 4
	PasswordMaxLen    = 16
	DisplayMaxWidth   = 512
	DisplayMaxHeight  = 512
	DisplayMinWidth   = 8
	DisplayMinHeight  = 8
	ProgramChunkSize  = 1024
)

// Timing constants.
const (
	CommandTimeout         = 5000 * time.Millisecond
	ResponseTimeout        = 3000 * time.Millisecond
	RetryCount             = 3
	RetryDelay             = 100 * time.Millisecond
	ReconnectDelay         = 1 * time.Second
	NotificationStartDelay = 400 * time.Millisecond
	MaxNotificationRetries = 3
)

// Default values.
const (
	DefaultBrightness    = 128
	DefaultSpeed         = 1
	DefaultDuration      = 0 // infinite
	DefaultFrameDuration = 100 * time.Millisecond
	DefaultTextSize      = 16
	DefaultColor         = 0xFFFFFF
)

// BLE connection parameters.
const (
	MTURequest        = 247
	MaxWritePayload   = 180
	DefaultPayload    = 20
	MTUPayloadCutoff  = 23
)

// Timer day bitmasks.
const (
	DayMonday    byte = 0x01
	DayTuesday   byte = 0x02
	DayWednesday byte = 0x04
	DayThursday  byte = 0x08
	DayFriday    byte = 0x10
	DaySaturday  byte = 0x20
	DaySunday    byte = 0x40
	DayDaily     byte = 0x7F
	DayWeekdays  byte = 0x1F
	DayWeekends  byte = 0x60
)

// CRC32 parameters.
const (
	CRC32Polynomial uint32 = 0x4C11DB7
	CRC32Initial    uint32 = 0xFFFFFFFF
)

// LZSS compression parameters.
const (
	LZSSWindowSize     = 512
	LZSSLookaheadSize  = 18
	LZSSMatchThreshold = 2
	LZSSInitBufPos     = LZSSWindowSize - LZSSLookaheadSize // 494
)

// Program markers.
const (
	ProgramStartMarker byte = 0x02
	ProgramDataMarker  byte = 0x03
)

// Password operations.
const (
	PasswordOpVerify byte = 0x01
	PasswordOpSet    byte = 0x00
)

// Power states.
const (
	PowerOn  byte = 0x01
	PowerOff byte = 0x00
)
