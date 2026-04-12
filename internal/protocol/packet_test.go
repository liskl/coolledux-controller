package protocol

import (
	"bytes"
	"testing"
)

func TestBuildBLEPacket_Basic(t *testing.T) {
	payload := []byte{0x05, 0x01} // power on
	pkt := BuildBLEPacket(CMD_TYPE_CONTROL, CMD_SUBTYPE_CONTROL, payload)

	// Expected: [0x52, 0x52, 0x02, 0x02, 0x02, 0x00, 0x05, 0x01]
	//                                       len=2 LE
	expected := []byte{0x52, 0x52, 0x02, 0x02, 0x02, 0x00, 0x05, 0x01}
	if !bytes.Equal(pkt, expected) {
		t.Errorf("BuildBLEPacket = %#v, want %#v", pkt, expected)
	}
}

func TestBuildBLEPacket_EmptyPayload(t *testing.T) {
	pkt := BuildBLEPacket(CMD_TYPE_CONTROL, CMD_SUBTYPE_CONTROL, nil)
	expected := []byte{0x52, 0x52, 0x02, 0x02, 0x00, 0x00}
	if !bytes.Equal(pkt, expected) {
		t.Errorf("BuildBLEPacket(empty) = %#v, want %#v", pkt, expected)
	}
}

func TestParseBLEPacket_RoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		cmdType    byte
		cmdSubtype byte
		payload    []byte
	}{
		{
			name:       "control command with payload",
			cmdType:    CMD_TYPE_CONTROL,
			cmdSubtype: CMD_SUBTYPE_CONTROL,
			payload:    []byte{0x05, 0x01, 0xAA, 0xBB, 0xCC, 0xDD},
		},
		{
			name:       "program command",
			cmdType:    CMD_TYPE_PROGRAM,
			cmdSubtype: CMD_SUBTYPE_DATA_TRANSMISSION,
			payload:    []byte{0x08, 0x02, 0x00, 0x00, 0x00},
		},
		{
			name:       "empty payload",
			cmdType:    CMD_TYPE_CONTROL,
			cmdSubtype: CMD_SUBTYPE_CONTROL,
			payload:    []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkt := BuildBLEPacket(tt.cmdType, tt.cmdSubtype, tt.payload)
			gotType, gotSub, gotPayload, err := ParseBLEPacket(pkt)
			if err != nil {
				t.Fatalf("ParseBLEPacket: %v", err)
			}
			if gotType != tt.cmdType {
				t.Errorf("cmdType = 0x%02X, want 0x%02X", gotType, tt.cmdType)
			}
			if gotSub != tt.cmdSubtype {
				t.Errorf("cmdSubtype = 0x%02X, want 0x%02X", gotSub, tt.cmdSubtype)
			}
			if !bytes.Equal(gotPayload, tt.payload) {
				t.Errorf("payload = %#v, want %#v", gotPayload, tt.payload)
			}
		})
	}
}

func TestParseBLEPacket_Errors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "too short", data: []byte{0x52, 0x52, 0x02}},
		{name: "wrong header", data: []byte{0x00, 0x00, 0x02, 0x02, 0x00, 0x00}},
		{name: "payload truncated", data: []byte{0x52, 0x52, 0x02, 0x02, 0x05, 0x00, 0xAA}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := ParseBLEPacket(tt.data)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestBuildStreamFrame_Basic(t *testing.T) {
	data := []byte{0x52, 0x52, 0x04, 0x05}
	frame := BuildStreamFrame(data)

	// start byte
	if frame[0] != START_BYTE {
		t.Errorf("frame[0] = 0x%02X, want 0x%02X (START_BYTE)", frame[0], START_BYTE)
	}
	// end byte
	if frame[len(frame)-1] != END_BYTE {
		t.Errorf("last byte = 0x%02X, want 0x%02X (END_BYTE)", frame[len(frame)-1], END_BYTE)
	}
}

func TestBuildStreamFrame_ParseRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "no special bytes",
			data: []byte{0x52, 0x52, 0x04, 0x05, 0x06, 0x07},
		},
		{
			name: "payload contains 0x01",
			data: []byte{0x01, 0xFF},
		},
		{
			name: "payload contains 0x02",
			data: []byte{0x02, 0xAA},
		},
		{
			name: "payload contains 0x03",
			data: []byte{0x03, 0xBB},
		},
		{
			name: "payload contains all special bytes",
			data: []byte{0x01, 0x02, 0x03, 0x00, 0x04},
		},
		{
			name: "empty payload",
			data: []byte{},
		},
		{
			name: "single zero byte",
			data: []byte{0x00},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := BuildStreamFrame(tt.data)
			got, err := ParseStreamFrame(frame)
			if err != nil {
				t.Fatalf("ParseStreamFrame: %v", err)
			}
			if !bytes.Equal(got, tt.data) {
				t.Errorf("round-trip failed:\n  input:  %#v\n  output: %#v", tt.data, got)
			}
		})
	}
}

func TestBuildStreamFrame_EscapesLengthBytes(t *testing.T) {
	// Craft a payload of length 0x0001. The big-endian length bytes are [0x00, 0x01].
	// 0x01 falls in the escape range, so it should be escaped to [0x02, 0x05].
	data := []byte{0xFF}
	frame := BuildStreamFrame(data)

	// frame[0] = 0x01 (start byte)
	// Next should be the escaped length+payload region.
	// Length BE = [0x00, 0x01]. 0x00 is not escaped. 0x01 IS escaped to [0x02, 0x05].
	// Then payload 0xFF is not escaped.
	// Then 0x03 (end byte).
	expected := []byte{0x01, 0x00, 0x02, 0x05, 0xFF, 0x03}
	if !bytes.Equal(frame, expected) {
		t.Errorf("frame = %#v, want %#v", frame, expected)
	}
}

func TestBuildStreamFrame_EscapesPayloadBytes(t *testing.T) {
	// Payload with bytes that need escaping.
	data := []byte{0x01, 0x02, 0x03}
	frame := BuildStreamFrame(data)

	// Verify round-trip works (escaping + unescaping is self-consistent).
	got, err := ParseStreamFrame(frame)
	if err != nil {
		t.Fatalf("ParseStreamFrame: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("round-trip: got %#v, want %#v", got, data)
	}

	// The frame should be longer than a naive frame because of escaping.
	// Length field [0x00, 0x03]: 0x03 is escaped -> [0x02, 0x07].
	// Payload [0x01, 0x02, 0x03]: each escaped ->
	//   0x01 -> [0x02, 0x05]
	//   0x02 -> [0x02, 0x06]
	//   0x03 -> [0x02, 0x07]
	// So: START(1) + 0x00(1) + esc(0x03)(2) + esc(0x01)(2) + esc(0x02)(2) + esc(0x03)(2) + END(1) = 11
	if len(frame) != 11 {
		t.Errorf("frame length = %d, want 11 (with escaping)", len(frame))
	}
}

func TestParseStreamFrame_Errors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "too short", data: []byte{0x01, 0x03}},
		{name: "wrong start byte", data: []byte{0x00, 0x00, 0x00, 0x03}},
		{name: "wrong end byte", data: []byte{0x01, 0x00, 0x00, 0x04}},
		{name: "truncated escape", data: []byte{0x01, 0x00, 0x00, 0x02, 0x03}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseStreamFrame(tt.data)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestFullFramingRoundTrip(t *testing.T) {
	// Build a BLE packet, wrap it in a stream frame, then unwrap both layers.
	payload := []byte{CMD_POWER, PowerOn, 0x01, 0x02, 0x03}
	blePkt := BuildBLEPacket(CMD_TYPE_CONTROL, CMD_SUBTYPE_CONTROL, payload)
	frame := BuildStreamFrame(blePkt)

	// Parse stream frame.
	innerData, err := ParseStreamFrame(frame)
	if err != nil {
		t.Fatalf("ParseStreamFrame: %v", err)
	}

	// Parse BLE packet.
	cmdType, cmdSubtype, gotPayload, err := ParseBLEPacket(innerData)
	if err != nil {
		t.Fatalf("ParseBLEPacket: %v", err)
	}

	if cmdType != CMD_TYPE_CONTROL {
		t.Errorf("cmdType = 0x%02X, want 0x%02X", cmdType, CMD_TYPE_CONTROL)
	}
	if cmdSubtype != CMD_SUBTYPE_CONTROL {
		t.Errorf("cmdSubtype = 0x%02X, want 0x%02X", cmdSubtype, CMD_SUBTYPE_CONTROL)
	}
	if !bytes.Equal(gotPayload, payload) {
		t.Errorf("payload = %#v, want %#v", gotPayload, payload)
	}
}
