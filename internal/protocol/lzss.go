package protocol

// Compress applies LZSS compression to data.
//
// Returns the compressed output and true if compression reduced size.
// Returns the original data and false if compression was not beneficial.
//
// Parameters:
//   - Window size: 512 bytes
//   - Lookahead size: 18 bytes
//   - Match threshold: 2 (matches of 2 or fewer bytes are stored as literals)
//   - Initial buffer position: 494 (WindowSize - LookaheadSize)
//
// Encoding: groups of 8 operations preceded by a 1-byte flag.
// Flag bit=1 means literal (1 byte), flag bit=0 means match ref (2 bytes).
// Flags packed LSB-first.
func Compress(data []byte) ([]byte, bool) {
	if len(data) == 0 {
		return data, false
	}

	// Initialize the sliding window buffer with zeros.
	buf := make([]byte, LZSSWindowSize+LZSSLookaheadSize)
	bufPos := LZSSInitBufPos

	// Copy input into the lookahead portion.
	lookahead := len(data)
	if lookahead > LZSSLookaheadSize {
		lookahead = LZSSLookaheadSize
	}
	copy(buf[bufPos:], data[:lookahead])

	var result []byte
	srcPos := 0

	for srcPos < len(data) {
		// Collect up to 8 operations into a group.
		var flag byte
		var groupBuf []byte

		for bit := 0; bit < 8 && srcPos < len(data); bit++ {
			// Search for the longest match in the window.
			bestLen := 0
			bestPos := 0

			searchStart := 0
			if bufPos > LZSSWindowSize {
				searchStart = bufPos - LZSSWindowSize
			}

			remaining := len(data) - srcPos
			maxMatch := LZSSLookaheadSize
			if remaining < maxMatch {
				maxMatch = remaining
			}

			for s := searchStart; s < bufPos; s++ {
				matchLen := 0
				for matchLen < maxMatch && buf[(s+matchLen)%LZSSWindowSize] == data[srcPos+matchLen] {
					matchLen++
				}
				if matchLen > bestLen {
					bestLen = matchLen
					bestPos = s % LZSSWindowSize
				}
			}

			if bestLen > LZSSMatchThreshold {
				// Match reference: 2 bytes.
				byte0 := byte(bestPos & 0xFF)
				byte1 := byte(((bestPos >> 4) & 0xF0) | ((bestLen - 3) & 0x0F))
				groupBuf = append(groupBuf, byte0, byte1)
				// flag bit stays 0 (match)

				// Advance buffer and source.
				for i := 0; i < bestLen; i++ {
					buf[bufPos%LZSSWindowSize] = data[srcPos]
					bufPos++
					srcPos++
				}
			} else {
				// Literal byte.
				flag |= 1 << bit
				groupBuf = append(groupBuf, data[srcPos])
				buf[bufPos%LZSSWindowSize] = data[srcPos]
				bufPos++
				srcPos++
			}
		}

		result = append(result, flag)
		result = append(result, groupBuf...)
	}

	if len(result) < len(data) {
		return result, true
	}
	return data, false
}

// Decompress reverses LZSS compression.
//
// The input must be data produced by Compress. Reads groups of 8 operations
// each preceded by a flag byte. Flag bit=1 means literal, bit=0 means match
// reference (2 bytes decoded as position+length).
func Decompress(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	buf := make([]byte, LZSSWindowSize)
	bufPos := LZSSInitBufPos

	var result []byte
	pos := 0

	for pos < len(data) {
		if pos >= len(data) {
			break
		}
		flag := data[pos]
		pos++

		for bit := 0; bit < 8 && pos < len(data); bit++ {
			if flag&(1<<bit) != 0 {
				// Literal byte.
				b := data[pos]
				pos++
				result = append(result, b)
				buf[bufPos%LZSSWindowSize] = b
				bufPos++
			} else {
				// Match reference (2 bytes).
				if pos+1 >= len(data) {
					return result
				}
				byte0 := data[pos]
				byte1 := data[pos+1]
				pos += 2

				matchPos := int(byte0) | (int(byte1&0xF0) << 4)
				matchLen := int(byte1&0x0F) + 3

				for i := 0; i < matchLen; i++ {
					b := buf[(matchPos+i)%LZSSWindowSize]
					result = append(result, b)
					buf[bufPos%LZSSWindowSize] = b
					bufPos++
				}
			}
		}
	}

	return result
}
