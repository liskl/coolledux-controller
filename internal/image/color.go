package ledimage

import "image/color"

// RGB444Transfer converts an 8-bit color channel value to a 4-bit (0-15)
// value using the CoolLEDUX piecewise transfer function.
//
// The mapping is NOT a simple bit shift:
//   - >= 238 -> 15
//   - <= 30  -> 0
//   - else   -> (value - 30) / 15 + 1
func RGB444Transfer(value uint8) uint8 {
	if value >= 238 {
		return 15
	}
	if value <= 30 {
		return 0
	}
	return uint8((int(value)-30)/15 + 1)
}

// EncodePixelRGB444 converts an RGB888 pixel to the 2-byte RGB444 format
// used by the device. Returns [0x0R, 0xGB] where R, G, B are each a
// single hex nibble.
func EncodePixelRGB444(r, g, b uint8) [2]byte {
	r4 := RGB444Transfer(r)
	g4 := RGB444Transfer(g)
	b4 := RGB444Transfer(b)
	return [2]byte{r4, (g4 << 4) | b4}
}

// EncodeImageColumnMajor encodes pixels in column-major order to the RGB444
// format expected by the device.
//
// Input pixels are in row-major order (left-to-right, top-to-bottom).
// Output iterates columns first, then rows within each column, emitting
// 2 bytes per pixel.
//
//	for col := 0; col < width; col++ {
//	    for row := 0; row < height; row++ {
//	        pixel = pixels[row*width + col]  // row-major source
//	        emit [0x0R, 0xGB]
//	    }
//	}
func EncodeImageColumnMajor(pixels []color.RGBA, width, height int) []byte {
	out := make([]byte, 0, width*height*2)
	for col := 0; col < width; col++ {
		for row := 0; row < height; row++ {
			srcIdx := row*width + col
			px := pixels[srcIdx]
			encoded := EncodePixelRGB444(px.R, px.G, px.B)
			out = append(out, encoded[0], encoded[1])
		}
	}
	return out
}
