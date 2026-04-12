package protocol

import (
	"encoding/binary"
	"testing"
)

func TestCalculate(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want uint32
	}{
		{
			name: "empty input",
			data: []byte{},
			want: CRC32Initial,
		},
		{
			name: "single zero byte",
			data: []byte{0x00},
			want: func() uint32 {
				// Manually compute: 32 iterations on byte 0x00.
				// tmp=0x00, so the "if tmp&xbit" branch never fires.
				crc := CRC32Initial
				for i := 0; i < 32; i++ {
					if crc&0x80000000 != 0 {
						crc = (crc << 1) ^ CRC32Polynomial
					} else {
						crc = crc << 1
					}
				}
				return crc
			}(),
		},
		{
			name: "single byte 0xFF",
			data: []byte{0xFF},
			want: func() uint32 {
				crc := CRC32Initial
				xbit := uint32(0x80000000)
				tmp := uint32(0xFF)
				for i := 0; i < 32; i++ {
					if crc&0x80000000 != 0 {
						crc = (crc << 1) ^ CRC32Polynomial
					} else {
						crc = crc << 1
					}
					if tmp&xbit != 0 {
						crc ^= CRC32Polynomial
					}
					xbit >>= 1
				}
				return crc
			}(),
		},
		{
			name: "multi-byte sequence [0x05, 0x01]",
			data: []byte{0x05, 0x01},
			want: func() uint32 {
				// Reference computation for power-on command payload.
				crc := CRC32Initial
				for _, b := range []byte{0x05, 0x01} {
					xbit := uint32(0x80000000)
					tmp := uint32(b) & 0xFF
					for i := 0; i < 32; i++ {
						if crc&0x80000000 != 0 {
							crc = (crc << 1) ^ CRC32Polynomial
						} else {
							crc = crc << 1
						}
						if tmp&xbit != 0 {
							crc ^= CRC32Polynomial
						}
						xbit >>= 1
					}
				}
				return crc
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Calculate(tt.data)
			if got != tt.want {
				t.Errorf("Calculate(%#v) = 0x%08X, want 0x%08X", tt.data, got, tt.want)
			}
		})
	}
}

func TestCalculate_32IterationsPerByte(t *testing.T) {
	// Verify that our CRC processes 32 iterations per byte, not 8.
	// With a standard 8-iterations-per-byte CRC, the result for [0x01] would
	// differ. We compute both and assert they aren't equal.
	data := []byte{0x01}

	fullCRC := Calculate(data)

	// 8-iteration version (standard approach):
	crc8 := CRC32Initial
	for _, b := range data {
		xbit := uint32(0x80000000)
		tmp := uint32(b) & 0xFF
		for i := 0; i < 8; i++ {
			if crc8&0x80000000 != 0 {
				crc8 = (crc8 << 1) ^ CRC32Polynomial
			} else {
				crc8 = crc8 << 1
			}
			if tmp&xbit != 0 {
				crc8 ^= CRC32Polynomial
			}
			xbit >>= 1
		}
	}

	if fullCRC == crc8 {
		t.Errorf("32-iteration CRC should differ from 8-iteration CRC for input [0x01], both returned 0x%08X", fullCRC)
	}
}

func TestToBytes_LittleEndian(t *testing.T) {
	crc := uint32(0xDEADBEEF)
	got := ToBytes(crc)
	want := [4]byte{0xEF, 0xBE, 0xAD, 0xDE}
	if got != want {
		t.Errorf("ToBytes(0x%08X) = %#v, want %#v", crc, got, want)
	}

	// Verify round-trip with binary.LittleEndian.
	decoded := binary.LittleEndian.Uint32(got[:])
	if decoded != crc {
		t.Errorf("round-trip failed: decoded 0x%08X, want 0x%08X", decoded, crc)
	}
}

func TestAppendCRC_RoundTrip(t *testing.T) {
	data := []byte{CMD_POWER, PowerOn}
	result := AppendCRC(data)

	if len(result) != len(data)+4 {
		t.Fatalf("AppendCRC returned %d bytes, want %d", len(result), len(data)+4)
	}

	// First bytes should be original data.
	for i, b := range data {
		if result[i] != b {
			t.Errorf("result[%d] = 0x%02X, want 0x%02X", i, result[i], b)
		}
	}

	// Last 4 bytes should be the CRC in little-endian.
	appendedCRC := binary.LittleEndian.Uint32(result[len(data):])
	expectedCRC := Calculate(data)
	if appendedCRC != expectedCRC {
		t.Errorf("appended CRC = 0x%08X, want 0x%08X", appendedCRC, expectedCRC)
	}
}

func TestVerify(t *testing.T) {
	data := []byte{0x05, 0x01}
	crc := Calculate(data)

	if !Verify(data, crc) {
		t.Error("Verify returned false for correct CRC")
	}

	if Verify(data, crc^1) {
		t.Error("Verify returned true for incorrect CRC")
	}
}

func TestAppendCRC_EmptyData(t *testing.T) {
	data := []byte{}
	result := AppendCRC(data)

	if len(result) != 4 {
		t.Fatalf("AppendCRC on empty data returned %d bytes, want 4", len(result))
	}

	got := binary.LittleEndian.Uint32(result)
	if got != CRC32Initial {
		t.Errorf("CRC of empty data = 0x%08X, want 0x%08X", got, CRC32Initial)
	}
}
