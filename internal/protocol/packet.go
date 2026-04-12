package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var (
	ErrPacketTooShort   = errors.New("packet too short")
	ErrInvalidHeader    = errors.New("invalid packet header")
	ErrInvalidStartByte = errors.New("invalid stream frame start byte")
	ErrInvalidEndByte   = errors.New("missing or invalid stream frame end byte")
	ErrTruncatedEscape  = errors.New("truncated escape sequence")
	ErrFrameTooShort    = errors.New("stream frame too short for length field")
)

// BuildBLEPacket constructs a Layer 1 BLE packet frame.
//
// Format: [0x52][0x52][cmdType][cmdSubtype][lenLE:2][payload...]
// Length is little-endian uint16.
func BuildBLEPacket(cmdType, cmdSubtype byte, payload []byte) []byte {
	pktLen := 6 + len(payload) // 2 header + 1 cmdType + 1 cmdSubtype + 2 length + payload
	pkt := make([]byte, pktLen)

	pkt[0] = PacketHeader[0]
	pkt[1] = PacketHeader[1]
	pkt[2] = cmdType
	pkt[3] = cmdSubtype
	binary.LittleEndian.PutUint16(pkt[4:6], uint16(len(payload)))
	copy(pkt[6:], payload)

	return pkt
}

// ParseBLEPacket parses a Layer 1 BLE packet frame, returning the command type,
// command subtype, and payload.
func ParseBLEPacket(data []byte) (cmdType, cmdSubtype byte, payload []byte, err error) {
	if len(data) < 6 {
		return 0, 0, nil, ErrPacketTooShort
	}

	if data[0] != PacketHeader[0] || data[1] != PacketHeader[1] {
		return 0, 0, nil, ErrInvalidHeader
	}

	cmdType = data[2]
	cmdSubtype = data[3]
	payloadLen := int(binary.LittleEndian.Uint16(data[4:6]))

	if len(data) < 6+payloadLen {
		return 0, 0, nil, fmt.Errorf("%w: need %d payload bytes, have %d", ErrPacketTooShort, payloadLen, len(data)-6)
	}

	payload = make([]byte, payloadLen)
	copy(payload, data[6:6+payloadLen])
	return cmdType, cmdSubtype, payload, nil
}

// escapeStreamBytes escapes bytes in the range 0x01-0x03 for Layer 2 framing.
// Each such byte B is replaced with [ESCAPE_BYTE, B XOR XOR_MASK].
func escapeStreamBytes(data []byte) []byte {
	var escaped []byte
	for _, b := range data {
		if b >= 0x01 && b <= 0x03 {
			escaped = append(escaped, ESCAPE_BYTE, b^XOR_MASK)
		} else {
			escaped = append(escaped, b)
		}
	}
	return escaped
}

// unescapeStreamBytes reverses the escaping applied by escapeStreamBytes.
func unescapeStreamBytes(data []byte) ([]byte, error) {
	var result []byte
	for i := 0; i < len(data); i++ {
		if data[i] == ESCAPE_BYTE {
			i++
			if i >= len(data) {
				return nil, ErrTruncatedEscape
			}
			result = append(result, data[i]^XOR_MASK)
		} else {
			result = append(result, data[i])
		}
	}
	return result, nil
}

// BuildStreamFrame constructs a Layer 2 stream frame wrapping the given data.
//
// Format: [START_BYTE][lenBE:2][escapedPayload...][END_BYTE]
//
// The length field is big-endian and counts the payload bytes BEFORE escaping.
// Both the length bytes and the payload bytes are then escaped together.
func BuildStreamFrame(data []byte) []byte {
	// Length = size of data, before escaping, encoded as big-endian uint16.
	var lenBytes [2]byte
	binary.BigEndian.PutUint16(lenBytes[:], uint16(len(data)))

	// The escaped region includes both the length bytes and the payload.
	toEscape := make([]byte, 2+len(data))
	toEscape[0] = lenBytes[0]
	toEscape[1] = lenBytes[1]
	copy(toEscape[2:], data)

	escaped := escapeStreamBytes(toEscape)

	frame := make([]byte, 1+len(escaped)+1)
	frame[0] = START_BYTE
	copy(frame[1:], escaped)
	frame[len(frame)-1] = END_BYTE

	return frame
}

// ParseStreamFrame parses a Layer 2 stream frame and returns the inner payload.
//
// It verifies the start and end bytes, unescapes the interior, reads the
// big-endian length, and returns the payload data.
func ParseStreamFrame(data []byte) ([]byte, error) {
	if len(data) < 4 {
		// Minimum: start byte + 2 length bytes (possibly escaped) + end byte.
		return nil, ErrFrameTooShort
	}

	if data[0] != START_BYTE {
		return nil, ErrInvalidStartByte
	}

	if data[len(data)-1] != END_BYTE {
		return nil, ErrInvalidEndByte
	}

	// Unescape the interior (between start and end bytes).
	interior := data[1 : len(data)-1]
	unescaped, err := unescapeStreamBytes(interior)
	if err != nil {
		return nil, err
	}

	if len(unescaped) < 2 {
		return nil, ErrFrameTooShort
	}

	payloadLen := int(binary.BigEndian.Uint16(unescaped[0:2]))
	payload := unescaped[2:]

	if len(payload) < payloadLen {
		return nil, fmt.Errorf("%w: length field says %d bytes, have %d", ErrPacketTooShort, payloadLen, len(payload))
	}

	result := make([]byte, payloadLen)
	copy(result, payload[:payloadLen])
	return result, nil
}
