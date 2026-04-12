package protocol

import "encoding/binary"

// Calculate computes the CRC32 of data using polynomial 0x4C11DB7 with
// initial value 0xFFFFFFFF and no final XOR.
//
// This is NOT standard CRC32. It processes 32 iterations per input byte,
// shifting through the full 32-bit CRC register while also shifting through
// the input byte value.
func Calculate(data []byte) uint32 {
	crc := CRC32Initial
	for _, b := range data {
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
}

// ToBytes returns the CRC32 value as 4 bytes in little-endian order.
func ToBytes(crc uint32) [4]byte {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], crc)
	return buf
}

// AppendCRC computes CRC32 over data and appends the 4-byte little-endian
// result to a copy of data.
func AppendCRC(data []byte) []byte {
	crc := Calculate(data)
	buf := ToBytes(crc)
	result := make([]byte, len(data)+4)
	copy(result, data)
	copy(result[len(data):], buf[:])
	return result
}

// Verify computes CRC32 over data and checks whether it matches expected.
func Verify(data []byte, expected uint32) bool {
	return Calculate(data) == expected
}
