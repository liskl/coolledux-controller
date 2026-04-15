package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestBuildGraffitiContent(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		height    int
		mode      uint8
		speed     uint8
		stayTime  uint8
		imageData []byte
	}{
		{
			name:      "small_image",
			width:     32,
			height:    16,
			mode:      1,
			speed:     5,
			stayTime:  10,
			imageData: []byte{0xFF, 0x00, 0xAA, 0xBB},
		},
		{
			name:      "empty_image_data",
			width:     96,
			height:    16,
			mode:      0,
			speed:     1,
			stayTime:  0,
			imageData: []byte{},
		},
		{
			name:      "large_dimensions",
			width:     512,
			height:    512,
			mode:      3,
			speed:     10,
			stayTime:  255,
			imageData: bytes.Repeat([]byte{0xDE, 0xAD}, 50),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := BuildGraffitiContent(tt.width, tt.height, tt.mode, tt.speed, tt.stayTime, tt.imageData)

			expectedLen := 28 + len(tt.imageData)
			if len(buf) != expectedLen {
				t.Fatalf("length = %d, want %d", len(buf), expectedLen)
			}

			// Total length field (big-endian at offset 0:4)
			gotTotalLen := binary.BigEndian.Uint32(buf[0:4])
			if gotTotalLen != uint32(expectedLen) {
				t.Errorf("totalLen field = %d, want %d", gotTotalLen, expectedLen)
			}

			// Content type = 0x02 (graffiti)
			if buf[4] != 0x02 {
				t.Errorf("content type = 0x%02X, want 0x02", buf[4])
			}

			// 7 reserved zero bytes at offset 5..11
			for i := 5; i < 12; i++ {
				if buf[i] != 0x00 {
					t.Errorf("reserved byte[%d] = 0x%02X, want 0x00", i, buf[i])
				}
			}

			// Layer type = 0x01
			if buf[12] != 0x01 {
				t.Errorf("layerType = 0x%02X, want 0x01", buf[12])
			}

			// startCol = 0 (big-endian at 13:15)
			if binary.BigEndian.Uint16(buf[13:15]) != 0 {
				t.Error("startCol != 0")
			}

			// startRow = 0 (big-endian at 15:17)
			if binary.BigEndian.Uint16(buf[15:17]) != 0 {
				t.Error("startRow != 0")
			}

			// showWidth (big-endian at 17:19)
			gotWidth := binary.BigEndian.Uint16(buf[17:19])
			if gotWidth != uint16(tt.width) {
				t.Errorf("showWidth = %d, want %d", gotWidth, tt.width)
			}

			// showHeight (big-endian at 19:21)
			gotHeight := binary.BigEndian.Uint16(buf[19:21])
			if gotHeight != uint16(tt.height) {
				t.Errorf("showHeight = %d, want %d", gotHeight, tt.height)
			}

			// mode, speed, stayTime
			if buf[21] != tt.mode {
				t.Errorf("mode = %d, want %d", buf[21], tt.mode)
			}
			if buf[22] != tt.speed {
				t.Errorf("speed = %d, want %d", buf[22], tt.speed)
			}
			if buf[23] != tt.stayTime {
				t.Errorf("stayTime = %d, want %d", buf[23], tt.stayTime)
			}

			// dataLen field (big-endian at 24:28)
			gotDataLen := binary.BigEndian.Uint32(buf[24:28])
			if gotDataLen != uint32(len(tt.imageData)) {
				t.Errorf("dataLen = %d, want %d", gotDataLen, len(tt.imageData))
			}

			// Image data at offset 28
			if !bytes.Equal(buf[28:], tt.imageData) {
				t.Errorf("imageData mismatch")
			}
		})
	}
}

func TestBuildAnimationContent(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		frames [][]byte
		delays []uint16
	}{
		{
			name:   "single_frame",
			width:  32,
			height: 16,
			frames: [][]byte{{0xFF, 0x00, 0xAA}},
			delays: []uint16{100},
		},
		{
			name:   "multiple_frames",
			width:  96,
			height: 16,
			frames: [][]byte{
				{0x01, 0x02},
				{0x03, 0x04},
				{0x05, 0x06},
			},
			delays: []uint16{50, 100, 200},
		},
		{
			name:   "delays_shorter_than_frames",
			width:  16,
			height: 16,
			frames: [][]byte{{0xAA}, {0xBB}, {0xCC}},
			delays: []uint16{100},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := BuildAnimationContent(tt.width, tt.height, tt.frames, tt.delays)

			frameCount := len(tt.frames)
			var frameDataLen int
			for _, f := range tt.frames {
				frameDataLen += len(f)
			}
			expectedLen := 24 + 2*frameCount + frameDataLen

			if len(buf) != expectedLen {
				t.Fatalf("length = %d, want %d", len(buf), expectedLen)
			}

			// Total length field
			gotTotalLen := binary.BigEndian.Uint32(buf[0:4])
			if gotTotalLen != uint32(expectedLen) {
				t.Errorf("totalLen field = %d, want %d", gotTotalLen, expectedLen)
			}

			// Content type = 0x03 (animation)
			if buf[4] != 0x03 {
				t.Errorf("content type = 0x%02X, want 0x03", buf[4])
			}

			// Mode/loop flag = 0x01
			if buf[5] != 0x01 {
				t.Errorf("mode flag = 0x%02X, want 0x01", buf[5])
			}

			// 6 reserved zero bytes at offset 6..11
			for i := 6; i < 12; i++ {
				if buf[i] != 0x00 {
					t.Errorf("reserved byte[%d] = 0x%02X, want 0x00", i, buf[i])
				}
			}

			// Width and height
			gotWidth := binary.BigEndian.Uint16(buf[17:19])
			if gotWidth != uint16(tt.width) {
				t.Errorf("showWidth = %d, want %d", gotWidth, tt.width)
			}
			gotHeight := binary.BigEndian.Uint16(buf[19:21])
			if gotHeight != uint16(tt.height) {
				t.Errorf("showHeight = %d, want %d", gotHeight, tt.height)
			}

			// Frame count (big-endian at 22:24)
			gotFrameCount := binary.BigEndian.Uint16(buf[22:24])
			if gotFrameCount != uint16(frameCount) {
				t.Errorf("frameCount = %d, want %d", gotFrameCount, frameCount)
			}

			// Per-frame delays (starting at offset 24)
			off := 24
			for i := 0; i < frameCount; i++ {
				gotDelay := binary.BigEndian.Uint16(buf[off : off+2])
				wantDelay := uint16(0)
				if i < len(tt.delays) {
					wantDelay = tt.delays[i]
				}
				if gotDelay != wantDelay {
					t.Errorf("delay[%d] = %d, want %d", i, gotDelay, wantDelay)
				}
				off += 2
			}

			// Frame data
			for i, f := range tt.frames {
				got := buf[off : off+len(f)]
				if !bytes.Equal(got, f) {
					t.Errorf("frame[%d] data mismatch", i)
				}
				off += len(f)
			}
		})
	}
}

func TestBuildRawGIFContent(t *testing.T) {
	tests := []struct {
		name     string
		startCol int
		startRow int
		width    int
		height   int
		gifData  []byte
	}{
		{
			name:     "minimal_gif",
			startCol: 0,
			startRow: 0,
			width:    96,
			height:   16,
			gifData:  []byte("GIF89a\x60\x00\x10\x00\x00\x00\x00\x00"),
		},
		{
			name:     "with_offset",
			startCol: 32,
			startRow: 4,
			width:    48,
			height:   8,
			gifData:  bytes.Repeat([]byte{0xDE, 0xAD, 0xBE, 0xEF}, 64),
		},
		{
			name:     "empty_payload",
			startCol: 0,
			startRow: 0,
			width:    96,
			height:   16,
			gifData:  []byte{},
		},
		{
			name:     "large_payload",
			startCol: 10,
			startRow: 2,
			width:    64,
			height:   14,
			gifData:  bytes.Repeat([]byte{0x42}, 4096),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := BuildRawGIFContent(tt.startCol, tt.startRow, tt.width, tt.height, tt.gifData)

			expectedLen := 26 + len(tt.gifData)
			if len(buf) != expectedLen {
				t.Fatalf("length = %d, want %d", len(buf), expectedLen)
			}

			if got := binary.BigEndian.Uint32(buf[0:4]); got != uint32(expectedLen) {
				t.Errorf("totalLen = %d, want %d", got, expectedLen)
			}
			if buf[4] != 0x0C {
				t.Errorf("content type = 0x%02X, want 0x0C", buf[4])
			}
			for i := 5; i < 12; i++ {
				if buf[i] != 0x00 {
					t.Errorf("reserved[%d] = 0x%02X, want 0x00", i, buf[i])
				}
			}
			if buf[12] != 0x01 {
				t.Errorf("layerType = 0x%02X, want 0x01", buf[12])
			}
			if buf[13] != 0x00 {
				t.Errorf("reserved[13] = 0x%02X, want 0x00", buf[13])
			}
			if got := binary.BigEndian.Uint16(buf[14:16]); got != uint16(tt.startCol) {
				t.Errorf("startCol = %d, want %d", got, tt.startCol)
			}
			if got := binary.BigEndian.Uint16(buf[16:18]); got != uint16(tt.startRow) {
				t.Errorf("startRow = %d, want %d", got, tt.startRow)
			}
			if got := binary.BigEndian.Uint16(buf[18:20]); got != uint16(tt.width) {
				t.Errorf("width = %d, want %d", got, tt.width)
			}
			if got := binary.BigEndian.Uint16(buf[20:22]); got != uint16(tt.height) {
				t.Errorf("height = %d, want %d", got, tt.height)
			}
			if got := binary.BigEndian.Uint32(buf[22:26]); got != uint32(len(tt.gifData)) {
				t.Errorf("gifLen field = %d, want %d", got, len(tt.gifData))
			}
			if !bytes.Equal(buf[26:], tt.gifData) {
				t.Errorf("gif data mismatch")
			}
		})
	}
}

func TestBuildTextContent(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		height    int
		mode      uint8
		speed     uint8
		stayTime  uint8
		moveSpace uint16
		textData  []byte
	}{
		{
			name:      "simple_text",
			width:     96,
			height:    16,
			mode:      2,
			speed:     5,
			stayTime:  10,
			moveSpace: 1,
			textData:  []byte("hello"),
		},
		{
			name:      "empty_text",
			width:     32,
			height:    32,
			mode:      0,
			speed:     0,
			stayTime:  0,
			moveSpace: 0,
			textData:  []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := BuildTextContent(tt.width, tt.height, tt.mode, tt.speed, tt.stayTime, tt.moveSpace, tt.textData)

			expectedLen := 26 + len(tt.textData)
			if len(buf) != expectedLen {
				t.Fatalf("length = %d, want %d", len(buf), expectedLen)
			}

			// Total length field
			gotTotalLen := binary.BigEndian.Uint32(buf[0:4])
			if gotTotalLen != uint32(expectedLen) {
				t.Errorf("totalLen field = %d, want %d", gotTotalLen, expectedLen)
			}

			// Content type = 0x01 (text)
			if buf[4] != 0x01 {
				t.Errorf("content type = 0x%02X, want 0x01", buf[4])
			}

			// 7 reserved zero bytes at offset 5..11
			for i := 5; i < 12; i++ {
				if buf[i] != 0x00 {
					t.Errorf("reserved byte[%d] = 0x%02X, want 0x00", i, buf[i])
				}
			}

			// Width, height
			gotWidth := binary.BigEndian.Uint16(buf[17:19])
			if gotWidth != uint16(tt.width) {
				t.Errorf("showWidth = %d, want %d", gotWidth, tt.width)
			}
			gotHeight := binary.BigEndian.Uint16(buf[19:21])
			if gotHeight != uint16(tt.height) {
				t.Errorf("showHeight = %d, want %d", gotHeight, tt.height)
			}

			// mode, speed, stayTime
			if buf[21] != tt.mode {
				t.Errorf("mode = %d, want %d", buf[21], tt.mode)
			}
			if buf[22] != tt.speed {
				t.Errorf("speed = %d, want %d", buf[22], tt.speed)
			}
			if buf[23] != tt.stayTime {
				t.Errorf("stayTime = %d, want %d", buf[23], tt.stayTime)
			}

			// moveSpace (big-endian at 24:26)
			gotMoveSpace := binary.BigEndian.Uint16(buf[24:26])
			if gotMoveSpace != tt.moveSpace {
				t.Errorf("moveSpace = %d, want %d", gotMoveSpace, tt.moveSpace)
			}

			// Text data at offset 26
			if !bytes.Equal(buf[26:], tt.textData) {
				t.Errorf("textData mismatch")
			}
		})
	}
}

func TestWrapProgramPayload(t *testing.T) {
	tests := []struct {
		name     string
		contents [][]byte
	}{
		{
			name:     "single_content",
			contents: [][]byte{{0x01, 0x02, 0x03}},
		},
		{
			name: "multiple_contents",
			contents: [][]byte{
				{0xAA, 0xBB},
				{0xCC, 0xDD, 0xEE},
			},
		},
		{
			name:     "empty_content",
			contents: [][]byte{{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := WrapProgramPayload(tt.contents...)

			// Calculate expected data length
			dataLen := 0
			for _, c := range tt.contents {
				dataLen += len(c)
			}
			expectedLen := 10 + dataLen
			if len(buf) != expectedLen {
				t.Fatalf("length = %d, want %d", len(buf), expectedLen)
			}

			// 8 zero prefix bytes
			for i := 0; i < 8; i++ {
				if buf[i] != 0x00 {
					t.Errorf("prefix byte[%d] = 0x%02X, want 0x00", i, buf[i])
				}
			}

			// Content count
			if buf[8] != byte(len(tt.contents)) {
				t.Errorf("content count = %d, want %d", buf[8], len(tt.contents))
			}

			// Separator byte
			if buf[9] != 0x00 {
				t.Errorf("separator = 0x%02X, want 0x00", buf[9])
			}

			// Content data starting at offset 10
			off := 10
			for i, c := range tt.contents {
				got := buf[off : off+len(c)]
				if !bytes.Equal(got, c) {
					t.Errorf("content[%d] mismatch", i)
				}
				off += len(c)
			}
		})
	}
}

func TestBuildProgramStartPacket(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		index     uint8
		count     uint8
		showCount uint8
	}{
		{
			name:      "basic_program",
			data:      []byte{0x01, 0x02, 0x03, 0x04, 0x05},
			index:     0,
			count:     1,
			showCount: 1,
		},
		{
			name:      "multi_program",
			data:      bytes.Repeat([]byte{0xAA}, 100),
			index:     2,
			count:     5,
			showCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := BuildProgramStartPacket(tt.data, tt.index, tt.count, tt.showCount)

			// Program start packets are wrapped in a stream frame.
			payload, err := ParseStreamFrame(frame)
			if err != nil {
				t.Fatalf("ParseStreamFrame: %v", err)
			}

			// payload: [0x02][CRC32:4 BE][dataLen:4 BE][index][count][showCount]
			if len(payload) != 12 {
				t.Fatalf("payload length = %d, want 12", len(payload))
			}

			// Marker
			if payload[0] != ProgramStartMarker {
				t.Errorf("marker = 0x%02X, want 0x%02X", payload[0], ProgramStartMarker)
			}

			// CRC32 in big-endian
			gotCRC := binary.BigEndian.Uint32(payload[1:5])
			wantCRC := Calculate(tt.data)
			if gotCRC != wantCRC {
				t.Errorf("CRC32 = 0x%08X, want 0x%08X", gotCRC, wantCRC)
			}

			// Data length in big-endian
			gotLen := binary.BigEndian.Uint32(payload[5:9])
			if gotLen != uint32(len(tt.data)) {
				t.Errorf("dataLen = %d, want %d", gotLen, len(tt.data))
			}

			// index, count, showCount
			if payload[9] != tt.index {
				t.Errorf("index = %d, want %d", payload[9], tt.index)
			}
			if payload[10] != tt.count {
				t.Errorf("count = %d, want %d", payload[10], tt.count)
			}
			if payload[11] != tt.showCount {
				t.Errorf("showCount = %d, want %d", payload[11], tt.showCount)
			}
		})
	}
}

func TestBuildProgramDataChunks(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		chunkSize int
		wantCount int
	}{
		{
			name:      "single_chunk",
			data:      []byte{0x01, 0x02, 0x03, 0x04, 0x05},
			chunkSize: 10,
			wantCount: 1,
		},
		{
			name:      "exact_fit",
			data:      bytes.Repeat([]byte{0xAA}, 20),
			chunkSize: 10,
			wantCount: 2,
		},
		{
			name:      "remainder_chunk",
			data:      bytes.Repeat([]byte{0xBB}, 25),
			chunkSize: 10,
			wantCount: 3,
		},
		{
			name:      "default_chunk_size",
			data:      bytes.Repeat([]byte{0xCC}, 2048),
			chunkSize: 0, // should use ProgramChunkSize (1024)
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packets := BuildProgramDataChunks(tt.data, tt.chunkSize)

			if len(packets) != tt.wantCount {
				t.Fatalf("chunk count = %d, want %d", len(packets), tt.wantCount)
			}

			effectiveChunkSize := tt.chunkSize
			if effectiveChunkSize <= 0 {
				effectiveChunkSize = ProgramChunkSize
			}

			// Verify each chunk
			for i, pkt := range packets {
				// Program data packets are wrapped in a stream frame.
				payload, err := ParseStreamFrame(pkt)
				if err != nil {
					t.Fatalf("chunk[%d] ParseStreamFrame: %v", i, err)
				}

				// Marker
				if payload[0] != ProgramDataMarker {
					t.Errorf("chunk[%d] marker = 0x%02X, want 0x%02X", i, payload[0], ProgramDataMarker)
				}

				// Reserved byte
				if payload[1] != 0x00 {
					t.Errorf("chunk[%d] reserved = 0x%02X, want 0x00", i, payload[1])
				}

				// Total length (big-endian at 2:6)
				gotTotalLen := binary.BigEndian.Uint32(payload[2:6])
				if gotTotalLen != uint32(len(tt.data)) {
					t.Errorf("chunk[%d] totalLen = %d, want %d", i, gotTotalLen, len(tt.data))
				}

				// Chunk index (big-endian at 6:8)
				gotIdx := binary.BigEndian.Uint16(payload[6:8])
				if gotIdx != uint16(i) {
					t.Errorf("chunk[%d] chunkIdx = %d, want %d", i, gotIdx, i)
				}

				// Calculate expected chunk data
				start := i * effectiveChunkSize
				end := start + effectiveChunkSize
				if end > len(tt.data) {
					end = len(tt.data)
				}
				expectedChunk := tt.data[start:end]

				// Chunk length (big-endian at 8:10)
				gotChunkLen := binary.BigEndian.Uint16(payload[8:10])
				if gotChunkLen != uint16(len(expectedChunk)) {
					t.Errorf("chunk[%d] chunkLen = %d, want %d", i, gotChunkLen, len(expectedChunk))
				}

				// Chunk data (at 10:10+chunkLen)
				gotChunkData := payload[10 : 10+len(expectedChunk)]
				if !bytes.Equal(gotChunkData, expectedChunk) {
					t.Errorf("chunk[%d] data mismatch", i)
				}

				// XOR checksum: covers bytes from offset 1 through end of chunk data
				// (the 0x00 reserved through end of data, not the marker or the checksum itself)
				var wantXOR byte
				for _, b := range payload[1 : 10+len(expectedChunk)] {
					wantXOR ^= b
				}
				gotXOR := payload[10+len(expectedChunk)]
				if gotXOR != wantXOR {
					t.Errorf("chunk[%d] XOR checksum = 0x%02X, want 0x%02X", i, gotXOR, wantXOR)
				}
			}
		})
	}
}
