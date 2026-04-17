package protocol

import (
	"bytes"
	"testing"
)

func TestCompressDecompress_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "empty",
			data: []byte{},
		},
		{
			name: "single byte",
			data: []byte{0x42},
		},
		{
			name: "short data",
			data: []byte{0x01, 0x02, 0x03, 0x04, 0x05},
		},
		{
			name: "repeated pattern compresses well",
			data: bytes.Repeat([]byte{0xAA, 0xBB, 0xCC, 0xDD}, 64),
		},
		{
			name: "long repeated single byte",
			data: bytes.Repeat([]byte{0x00}, 200),
		},
		{
			name: "alternating pattern",
			data: func() []byte {
				d := make([]byte, 128)
				for i := range d {
					d[i] = byte(i % 4)
				}
				return d
			}(),
		},
		{
			// Regression: a self-referential match (matchLen > distance) used
			// to corrupt the first byte past the repeating region by exactly
			// one byte. Surfaced on the matrix as a phantom pixel one column
			// over from a single-column vertical line.
			name: "alternating run then zeros (self-ref match)",
			data: append(
				bytes.Repeat([]byte{0x00, 0x0F}, 16),
				bytes.Repeat([]byte{0x00}, 64)...,
			),
		},
		{
			// 96x16 image with a single blue column encoded column-major as
			// RGB444 pairs: 32 bytes of 00 0F followed by 3040 zeros.
			name: "96x16 column0 blue line",
			data: append(
				bytes.Repeat([]byte{0x00, 0x0F}, 16),
				bytes.Repeat([]byte{0x00}, 3040)...,
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressed, wasCompressed := Compress(tt.data)

			if len(tt.data) == 0 {
				if wasCompressed {
					t.Error("empty input should not report wasCompressed=true")
				}
				return
			}

			var decompressed []byte
			if wasCompressed {
				decompressed = Decompress(compressed)
			} else {
				// If not compressed, the raw data was returned.
				decompressed = compressed
			}

			if !bytes.Equal(decompressed, tt.data) {
				t.Errorf("round-trip failed:\n  input:        %d bytes\n  compressed:   %d bytes (wasCompressed=%v)\n  decompressed: %d bytes",
					len(tt.data), len(compressed), wasCompressed, len(decompressed))
				if len(tt.data) <= 32 {
					t.Errorf("  input:        %#v", tt.data)
					t.Errorf("  decompressed: %#v", decompressed)
				}
			}
		})
	}
}

func TestCompress_IncompressibleData(t *testing.T) {
	// Random-ish data that shouldn't compress well.
	data := make([]byte, 50)
	for i := range data {
		data[i] = byte((i*7 + 13) % 256)
	}

	compressed, wasCompressed := Compress(data)
	if wasCompressed {
		t.Logf("compressed %d -> %d bytes", len(data), len(compressed))
		// If it claims compression, it should actually be smaller.
		if len(compressed) >= len(data) {
			t.Error("wasCompressed=true but compressed data is not smaller")
		}
	} else {
		// Should have returned the original data unmodified.
		if !bytes.Equal(compressed, data) {
			t.Error("wasCompressed=false but data was modified")
		}
	}
}

func TestCompress_RepeatablePattern(t *testing.T) {
	// Highly compressible: 256 bytes of repeating 4-byte pattern.
	data := bytes.Repeat([]byte{0xDE, 0xAD, 0xBE, 0xEF}, 64)

	compressed, wasCompressed := Compress(data)
	if !wasCompressed {
		t.Fatal("expected highly repetitive data to compress")
	}

	if len(compressed) >= len(data) {
		t.Errorf("compressed size %d should be less than input size %d", len(compressed), len(data))
	}

	decompressed := Decompress(compressed)
	if !bytes.Equal(decompressed, data) {
		t.Error("decompressed data does not match original")
	}
}

func TestDecompress_Empty(t *testing.T) {
	result := Decompress([]byte{})
	if len(result) != 0 {
		t.Errorf("Decompress(empty) returned %d bytes, want 0", len(result))
	}
}

func TestDecompress_TruncatedLiteral(t *testing.T) {
	// Flag byte 0xFF says "8 literals coming" but we only provide 1.
	// Decompress should break mid-group and return what it got without
	// panicking on the out-of-bounds slice access.
	out := Decompress([]byte{0xFF, 0xAA})
	if len(out) != 1 || out[0] != 0xAA {
		t.Errorf("got % X, want [AA]", out)
	}
}

func TestDecompress_TruncatedMatch(t *testing.T) {
	// Flag byte 0x00 says "8 match pairs coming" but only 1 byte follows,
	// so the idx+1 >= len check should fire before we try to read the high
	// nibble. No panic, no emitted bytes.
	out := Decompress([]byte{0x00, 0x42})
	if len(out) != 0 {
		t.Errorf("got % X, want empty", out)
	}
}

func TestCompress_FlagBitsAreLSBFirst(t *testing.T) {
	// Use a repeating input long enough to actually compress, but where we can
	// predict the flag layout. With 8 unique bytes followed by a long repeat of
	// the same 8-byte pattern, the first group should be all literals (flag = 0xFF).
	pattern := []byte{0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80}
	// Repeat enough to guarantee compression wins.
	data := make([]byte, 0, len(pattern)*32)
	for i := 0; i < 32; i++ {
		data = append(data, pattern...)
	}

	compressed, wasCompressed := Compress(data)
	if !wasCompressed {
		t.Fatal("expected data to compress")
	}

	if len(compressed) < 1 {
		t.Fatal("compressed output too short")
	}

	// The first group of 8 operations should all be literals because there are
	// no prior occurrences in the sliding window. That means all 8 flag bits
	// should be set: 0xFF.
	flag := compressed[0]
	expectedFlag := byte(0xFF)
	if flag != expectedFlag {
		t.Errorf("first flag byte = 0x%02X, want 0x%02X (all literals, LSB-first)", flag, expectedFlag)
	}
}
