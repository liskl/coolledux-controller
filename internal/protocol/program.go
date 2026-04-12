package protocol

import "encoding/binary"

// putBE16 writes v as a big-endian uint16 into buf[0:2].
func putBE16(buf []byte, v uint16) {
	binary.BigEndian.PutUint16(buf, v)
}

// putBE32 writes v as a big-endian uint32 into buf[0:4].
func putBE32(buf []byte, v uint32) {
	binary.BigEndian.PutUint32(buf, v)
}

// BuildGraffitiContent builds a graffiti/image content block.
//
//	Format:
//	[length:4 BE][0x02][0x00 x7][layerType=0x01][startCol:2 BE=0][startRow:2 BE=0]
//	[showWidth:2 BE][showHeight:2 BE][mode][speed][stayTime][dataLen:4 BE][imageData...]
func BuildGraffitiContent(width, height int, mode, speed, stayTime uint8, imageData []byte) []byte {
	// Header (4 length) + 1 type + 7 reserved + 1 layer + 2 startCol + 2 startRow
	// + 2 showWidth + 2 showHeight + 1 mode + 1 speed + 1 stayTime + 4 dataLen + imageData
	// = 4 + 1 + 7 + 1 + 2 + 2 + 2 + 2 + 1 + 1 + 1 + 4 + len(imageData)
	// = 28 + len(imageData)
	totalLen := 28 + len(imageData)
	buf := make([]byte, totalLen)

	putBE32(buf[0:4], uint32(totalLen))
	buf[4] = 0x02 // content type: graffiti
	// buf[5:12] = 0x00 x7 (reserved, already zero)
	buf[12] = 0x01 // layer type
	// buf[13:15] startCol = 0 (already zero)
	// buf[15:17] startRow = 0 (already zero)
	putBE16(buf[17:19], uint16(width))
	putBE16(buf[19:21], uint16(height))
	buf[21] = mode
	buf[22] = speed
	buf[23] = stayTime
	putBE32(buf[24:28], uint32(len(imageData)))
	copy(buf[28:], imageData)

	return buf
}

// BuildAnimationContent builds an animation content block.
//
//	Format:
//	[length:4 BE][0x03][0x01][0x00 x6][layerType=0x01][startCol:2 BE=0][startRow:2 BE=0]
//	[showWidth:2 BE][showHeight:2 BE][0x00][frameCount:2 BE][delays:2*N BE][frameData...]
func BuildAnimationContent(width, height int, frames [][]byte, delays []uint16) []byte {
	frameCount := len(frames)

	// Calculate total frame data size.
	var frameDataLen int
	for _, f := range frames {
		frameDataLen += len(f)
	}

	// Header: 4 length + 1 type + 1 mode + 6 reserved + 1 layer + 2 startCol + 2 startRow
	// + 2 showWidth + 2 showHeight + 1 reserved + 2 frameCount + 2*N delays + frameData
	// = 4 + 1 + 1 + 6 + 1 + 2 + 2 + 2 + 2 + 1 + 2 + 2*frameCount + frameDataLen
	// = 24 + 2*frameCount + frameDataLen
	totalLen := 24 + 2*frameCount + frameDataLen
	buf := make([]byte, totalLen)

	putBE32(buf[0:4], uint32(totalLen))
	buf[4] = 0x03 // content type: animation
	buf[5] = 0x01 // mode/loop flag
	// buf[6:12] = 0x00 x6 (reserved, already zero)
	buf[12] = 0x01 // layer type
	// buf[13:15] startCol = 0
	// buf[15:17] startRow = 0
	putBE16(buf[17:19], uint16(width))
	putBE16(buf[19:21], uint16(height))
	// buf[21] = 0x00 reserved
	putBE16(buf[22:24], uint16(frameCount))

	// Write per-frame delays.
	off := 24
	for i := 0; i < frameCount; i++ {
		d := uint16(0)
		if i < len(delays) {
			d = delays[i]
		}
		putBE16(buf[off:off+2], d)
		off += 2
	}

	// Write concatenated frame data.
	for _, f := range frames {
		copy(buf[off:], f)
		off += len(f)
	}

	return buf
}

// BuildTextContent builds a text content block.
//
//	Format:
//	[length:4 BE][0x01][0x00 x7][layerType=0x01][startCol:2 BE=0][startRow:2 BE=0]
//	[showWidth:2 BE][showHeight:2 BE][mode][speed][stayTime][moveSpace:2 BE][textData...]
func BuildTextContent(width, height int, mode, speed, stayTime uint8, moveSpace uint16, textData []byte) []byte {
	// 4 + 1 + 7 + 1 + 2 + 2 + 2 + 2 + 1 + 1 + 1 + 2 + len(textData)
	// = 26 + len(textData)
	totalLen := 26 + len(textData)
	buf := make([]byte, totalLen)

	putBE32(buf[0:4], uint32(totalLen))
	buf[4] = 0x01 // content type: text
	// buf[5:12] = 0x00 x7 (reserved, already zero)
	buf[12] = 0x01 // layer type
	// buf[13:15] startCol = 0
	// buf[15:17] startRow = 0
	putBE16(buf[17:19], uint16(width))
	putBE16(buf[19:21], uint16(height))
	buf[21] = mode
	buf[22] = speed
	buf[23] = stayTime
	putBE16(buf[24:26], moveSpace)
	copy(buf[26:], textData)

	return buf
}

// WrapProgramPayload wraps one or more content blocks in the program envelope.
//
//	Format: [0x00 x8][contentCount:1][0x00][content1...][content2...]
func WrapProgramPayload(contents ...[]byte) []byte {
	dataLen := 0
	for _, c := range contents {
		dataLen += len(c)
	}
	// 8 reserved + 1 count + 1 separator + content data
	buf := make([]byte, 10+dataLen)
	// buf[0:8] = 0x00 x8 (already zero)
	buf[8] = byte(len(contents))
	// buf[9] = 0x00 separator (already zero)

	off := 10
	for _, c := range contents {
		copy(buf[off:], c)
		off += len(c)
	}
	return buf
}

// BuildProgramStartPacket builds the program start packet.
//
// Inner payload: [0x02][CRC32_of_raw:4 BE][dataLen:4 BE][index][count][showCount]
// CRC32 is big-endian (unlike control commands which use LE).
// Wrapped in BLE packet (CmdType=program, CmdSubtype=data_transmission) then stream frame.
func BuildProgramStartPacket(programData []byte, index, count, showCount uint8) []byte {
	crc := Calculate(programData)

	// 1 marker + 4 CRC + 4 dataLen + 1 index + 1 count + 1 showCount = 12
	inner := make([]byte, 12)
	inner[0] = ProgramStartMarker
	putBE32(inner[1:5], crc)
	putBE32(inner[5:9], uint32(len(programData)))
	inner[9] = index
	inner[10] = count
	inner[11] = showCount

	return BuildStreamFrame(inner)
}

// BuildProgramDataChunks splits compressed data into chunks and builds framed
// packets for each.
//
// Each chunk payload:
//
//	[0x03][0x00][totalLen:4 BE][chunkIdx:2 BE][chunkLen:2 BE][chunkData...][xorChecksum:1]
//
// XOR checksum covers bytes from offset 1 (the 0x00) through end of chunk data
// (excludes the 0x03 marker and the checksum byte itself).
//
// Each chunk is wrapped in BLE packet (CmdType=program, CmdSubtype=data_packet)
// then stream frame.
func BuildProgramDataChunks(compressedData []byte, chunkSize int) [][]byte {
	if chunkSize <= 0 {
		chunkSize = ProgramChunkSize
	}

	totalLen := len(compressedData)
	numChunks := (totalLen + chunkSize - 1) / chunkSize
	packets := make([][]byte, 0, numChunks)

	for i := 0; i < numChunks; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > totalLen {
			end = totalLen
		}
		chunk := compressedData[start:end]

		// 1 marker + 1 reserved + 4 totalLen + 2 chunkIdx + 2 chunkLen + N data + 1 xor = 11 + N
		inner := make([]byte, 11+len(chunk))
		inner[0] = ProgramDataMarker
		inner[1] = 0x00
		putBE32(inner[2:6], uint32(totalLen))
		putBE16(inner[6:8], uint16(i))
		putBE16(inner[8:10], uint16(len(chunk)))
		copy(inner[10:10+len(chunk)], chunk)

		// XOR checksum: bytes from offset 1 through end of chunk data.
		var xor byte
		for _, b := range inner[1 : 10+len(chunk)] {
			xor ^= b
		}
		inner[10+len(chunk)] = xor

		packets = append(packets, BuildStreamFrame(inner))
	}

	return packets
}
