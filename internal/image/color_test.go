package ledimage

import (
	"image/color"
	"testing"
)

func TestRGB444Transfer(t *testing.T) {
	tests := []struct {
		name  string
		input uint8
		want  uint8
	}{
		{"zero", 0, 0},
		{"at_low_boundary", 30, 0},
		{"just_above_low", 31, 1},
		{"mid_range_237", 237, 14},
		{"at_high_boundary", 238, 15},
		{"max_255", 255, 15},
		// Spot checks within the linear region
		{"value_45", 45, 2},
		{"value_60", 60, 3},
		{"value_120", 120, 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RGB444Transfer(tt.input)
			if got != tt.want {
				t.Errorf("RGB444Transfer(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestEncodePixelRGB444(t *testing.T) {
	tests := []struct {
		name string
		r, g, b uint8
		want [2]byte
	}{
		{
			name: "white",
			r: 255, g: 255, b: 255,
			want: [2]byte{0x0F, 0xFF},
		},
		{
			name: "black",
			r: 0, g: 0, b: 0,
			want: [2]byte{0x00, 0x00},
		},
		{
			name: "pure_red",
			r: 255, g: 0, b: 0,
			want: [2]byte{0x0F, 0x00},
		},
		{
			name: "pure_green",
			r: 0, g: 255, b: 0,
			want: [2]byte{0x00, 0xF0},
		},
		{
			name: "pure_blue",
			r: 0, g: 0, b: 255,
			want: [2]byte{0x00, 0x0F},
		},
		{
			// RGB(200,100,50): R=12, G=5, B=2 -> [0x0C, 0x52]
			name: "example_from_spec",
			r: 200, g: 100, b: 50,
			want: [2]byte{0x0C, 0x52},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EncodePixelRGB444(tt.r, tt.g, tt.b)
			if got != tt.want {
				t.Errorf("EncodePixelRGB444(%d,%d,%d) = [0x%02X,0x%02X], want [0x%02X,0x%02X]",
					tt.r, tt.g, tt.b, got[0], got[1], tt.want[0], tt.want[1])
			}
		})
	}
}

func TestEncodeImageColumnMajor(t *testing.T) {
	// 2x2 image in row-major order:
	//   (0,0)=Red   (1,0)=Green
	//   (0,1)=Blue  (1,1)=White
	pixels := []color.RGBA{
		{R: 255, G: 0, B: 0, A: 255},   // row 0, col 0
		{R: 0, G: 255, B: 0, A: 255},   // row 0, col 1
		{R: 0, G: 0, B: 255, A: 255},   // row 1, col 0
		{R: 255, G: 255, B: 255, A: 255}, // row 1, col 1
	}

	got := EncodeImageColumnMajor(pixels, 2, 2)

	// Column-major order: col0 (Red, Blue), col1 (Green, White)
	// Red:   R=15,G=0,B=0  -> [0x0F, 0x00]
	// Blue:  R=0,G=0,B=15  -> [0x00, 0x0F]
	// Green: R=0,G=15,B=0  -> [0x00, 0xF0]
	// White: R=15,G=15,B=15 -> [0x0F, 0xFF]
	want := []byte{
		0x0F, 0x00, // col 0, row 0 (Red)
		0x00, 0x0F, // col 0, row 1 (Blue)
		0x00, 0xF0, // col 1, row 0 (Green)
		0x0F, 0xFF, // col 1, row 1 (White)
	}

	if len(got) != len(want) {
		t.Fatalf("EncodeImageColumnMajor length = %d, want %d", len(got), len(want))
	}

	for i := range got {
		if got[i] != want[i] {
			t.Errorf("byte %d: got 0x%02X, want 0x%02X", i, got[i], want[i])
		}
	}
}

func TestEncodeImageColumnMajor_3x2(t *testing.T) {
	// 3 columns x 2 rows, verifying wider-than-tall layout
	//   row0: A(255,0,0)  B(0,255,0)  C(0,0,255)
	//   row1: D(255,255,0) E(255,0,255) F(0,255,255)
	pixels := []color.RGBA{
		{R: 255, G: 0, B: 0, A: 255},     // A
		{R: 0, G: 255, B: 0, A: 255},     // B
		{R: 0, G: 0, B: 255, A: 255},     // C
		{R: 255, G: 255, B: 0, A: 255},   // D
		{R: 255, G: 0, B: 255, A: 255},   // E
		{R: 0, G: 255, B: 255, A: 255},   // F
	}

	got := EncodeImageColumnMajor(pixels, 3, 2)

	// Column-major: col0(A,D), col1(B,E), col2(C,F)
	want := []byte{
		0x0F, 0x00, // A: R=15,G=0,B=0
		0x0F, 0xF0, // D: R=15,G=15,B=0
		0x00, 0xF0, // B: R=0,G=15,B=0
		0x0F, 0x0F, // E: R=15,G=0,B=15
		0x00, 0x0F, // C: R=0,G=0,B=15
		0x00, 0xFF, // F: R=0,G=15,B=15
	}

	if len(got) != len(want) {
		t.Fatalf("length = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("byte %d: got 0x%02X, want 0x%02X", i, got[i], want[i])
		}
	}
}
