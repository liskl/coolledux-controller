// Bisect the LZSS round-trip bug: find the shortest input that fails to round-trip.
package main

import (
	"bytes"
	"fmt"

	"github.com/liskl/coolledux-controller/internal/protocol"
)

func roundTrip(data []byte) bool {
	c, _ := protocol.Compress(data)
	r := protocol.Decompress(c)
	return bytes.Equal(r, data)
}

func dump(data []byte) string {
	s := ""
	for i, b := range data {
		if i >= 40 {
			s += "..."
			break
		}
		s += fmt.Sprintf("%02X ", b)
	}
	return s
}

func main() {
	// Reproduce the column-0 vertical line pattern: 32 bytes of 00 0F pairs,
	// followed by 3040 bytes of zeros.
	pattern := []byte{}
	for i := 0; i < 16; i++ {
		pattern = append(pattern, 0x00, 0x0F)
	}
	for i := 0; i < 3040; i++ {
		pattern = append(pattern, 0x00)
	}
	fmt.Printf("full 3072-byte pattern: round-trip ok=%v\n\n", roundTrip(pattern))

	// Bisect: shortest failing prefix.
	for n := 33; n <= len(pattern); n++ {
		if !roundTrip(pattern[:n]) {
			fmt.Printf("shortest failing prefix: %d bytes\n", n)
			fmt.Printf("  input:  %s\n", dump(pattern[:n]))
			c, _ := protocol.Compress(pattern[:n])
			r := protocol.Decompress(c)
			fmt.Printf("  compr:  %s\n", dump(c))
			fmt.Printf("  output: %s\n", dump(r))
			// Show diff
			for i := 0; i < len(pattern[:n]) && i < len(r); i++ {
				if pattern[i] != r[i] {
					fmt.Printf("  first diff at byte %d: in=%02X out=%02X\n", i, pattern[i], r[i])
					break
				}
			}
			break
		}
	}

	// Also try: what if we replace "00 0F" repetition with "AA BB" repetition?
	// Does it also fail? This tells us if the bug is content-sensitive.
	fmt.Println()
	p2 := []byte{}
	for i := 0; i < 16; i++ {
		p2 = append(p2, 0xAA, 0xBB)
	}
	for i := 0; i < 3040; i++ {
		p2 = append(p2, 0x00)
	}
	fmt.Printf("AA BB × 16 + zeros: round-trip ok=%v\n", roundTrip(p2))

	// And: constant bytes (no alternation)?
	p3 := append(bytes.Repeat([]byte{0x42}, 32), bytes.Repeat([]byte{0x00}, 3040)...)
	fmt.Printf("42 × 32 + zeros:     round-trip ok=%v\n", roundTrip(p3))
}
