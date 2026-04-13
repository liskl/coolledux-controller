package protocol

// Compress applies LZSS compression to data, matching the CoolLED 1248 Android
// reference implementation. Ported from the Python SDK's LZSSCompressor.
//
// Returns the compressed output and true if compression reduced size.
// Returns the original data and false if compression was not beneficial.
//
// Parameters:
//   - Window size: 512 bytes
//   - Lookahead size: 18 bytes
//   - Match threshold: 2 (matches of 2 or fewer bytes are stored as literals)
//   - Initial write pointer: 494 (WindowSize - LookaheadSize)
//
// Encoding: groups of 8 operations preceded by a 1-byte flag.
// Flag bit=1 means literal (1 byte), flag bit=0 means match ref (2 bytes).
// Flags packed LSB-first.
func Compress(data []byte) ([]byte, bool) {
	if len(data) == 0 {
		return data, false
	}

	buf := make([]byte, LZSSWindowSize)
	r := LZSSInitBufPos // write pointer into the sliding window
	historyLen := 0     // bytes actually written to the window (grows up to WindowSize)

	var result []byte
	codeBuf := make([]byte, 17)
	codeBufPtr := 1
	var flags byte
	flagMask := byte(1)

	pos := 0
	for pos < len(data) {
		maxMatch := LZSSLookaheadSize
		if rem := len(data) - pos; rem < maxMatch {
			maxMatch = rem
		}
		matchLen := 0
		matchPos := 0

		// Only search within actual history, scanning by distance (most-recent first).
		if historyLen > 0 {
			maxSearch := historyLen
			if maxSearch > LZSSWindowSize {
				maxSearch = LZSSWindowSize
			}
			for dist := 1; dist <= maxSearch; dist++ {
				idx := (r - dist) & (LZSSWindowSize - 1)
				// Self-referential matches (matchLen > dist) are valid in
				// standard Okumura LZSS but the CoolLEDUX firmware decoder
				// does not handle them correctly -- it produces an off-by-one
				// byte past the reference distance. Cap extension at `dist`
				// bytes so every emitted match is pure non-overlapping copy.
				extMax := maxMatch
				if dist < extMax {
					extMax = dist
				}
				length := 0
				for length < extMax {
					if buf[(idx+length)&(LZSSWindowSize-1)] != data[pos+length] {
						break
					}
					length++
				}
				if length > matchLen {
					matchLen = length
					matchPos = idx
					if matchLen == maxMatch {
						break
					}
				}
			}
		}

		if matchLen > LZSSMatchThreshold {
			codeBuf[codeBufPtr] = byte(matchPos & 0xFF)
			codeBuf[codeBufPtr+1] = byte(((matchPos >> 4) & 0xF0) | ((matchLen - 3) & 0x0F))
			codeBufPtr += 2
		} else {
			matchLen = 1
			flags |= flagMask
			codeBuf[codeBufPtr] = data[pos]
			codeBufPtr++
		}

		flagMask <<= 1
		if flagMask == 0 { // completed a group of 8
			codeBuf[0] = flags
			result = append(result, codeBuf[:codeBufPtr]...)
			codeBuf = make([]byte, 17)
			codeBufPtr = 1
			flags = 0
			flagMask = 1
		}

		for i := 0; i < matchLen; i++ {
			buf[r] = data[pos+i]
			r = (r + 1) & (LZSSWindowSize - 1)
		}
		historyLen += matchLen
		if historyLen > LZSSWindowSize {
			historyLen = LZSSWindowSize
		}
		pos += matchLen
	}

	if codeBufPtr > 1 {
		codeBuf[0] = flags
		result = append(result, codeBuf[:codeBufPtr]...)
	}

	if len(result) < len(data) {
		return result, true
	}
	return data, false
}

// Decompress reverses LZSS compression, matching the CoolLED 1248 Android
// reference implementation. Ported from the Python SDK's LZSSCompressor.
func Decompress(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	buf := make([]byte, LZSSWindowSize)
	r := LZSSInitBufPos
	var out []byte

	idx := 0
	flags := uint32(0)
	for idx < len(data) {
		flags >>= 1
		if flags&0x100 == 0 {
			if idx >= len(data) {
				break
			}
			flags = uint32(data[idx]) | 0xFF00
			idx++
		}
		if flags&1 != 0 {
			if idx >= len(data) {
				break
			}
			b := data[idx]
			idx++
			out = append(out, b)
			buf[r] = b
			r = (r + 1) & (LZSSWindowSize - 1)
		} else {
			if idx+1 >= len(data) {
				break
			}
			low := data[idx]
			high := data[idx+1]
			idx += 2
			matchPos := (int(low) | (int(high&0xF0) << 4)) & (LZSSWindowSize - 1)
			matchLen := int(high&0x0F) + 3
			for i := 0; i < matchLen; i++ {
				b := buf[matchPos]
				matchPos = (matchPos + 1) & (LZSSWindowSize - 1)
				out = append(out, b)
				buf[r] = b
				r = (r + 1) & (LZSSWindowSize - 1)
			}
		}
	}

	return out
}
