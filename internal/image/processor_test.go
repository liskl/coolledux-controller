package ledimage

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/liskl/coolledux-controller/internal/models"
)

// newTestImage creates a solid-colored RGBA image of the given size.
func newTestImage(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// newCheckerImage creates a 2-color checkerboard image. Even pixels get c1, odd get c2.
func newCheckerImage(w, h int, c1, c2 color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if (x+y)%2 == 0 {
				img.SetRGBA(x, y, c1)
			} else {
				img.SetRGBA(x, y, c2)
			}
		}
	}
	return img
}

func TestResizeToFit(t *testing.T) {
	tests := []struct {
		name              string
		srcW, srcH        int
		targetW, targetH  int
		expectW, expectH  int
	}{
		{
			name:    "landscape to smaller, preserves aspect",
			srcW:    200, srcH: 100,
			targetW: 96, targetH: 16,
			// 200x100 has aspect 2:1. Fitting into 96x16:
			// width-limited: 96x48 (too tall). height-limited: 32x16.
			expectW: 32, expectH: 16,
		},
		{
			name:    "square to landscape target",
			srcW:    100, srcH: 100,
			targetW: 96, targetH: 16,
			expectW: 16, expectH: 16,
		},
		{
			name:    "already fits",
			srcW:    50, srcH: 10,
			targetW: 96, targetH: 16,
			expectW: 50, expectH: 10,
		},
		{
			name:    "tall image to landscape target",
			srcW:    16, srcH: 100,
			targetW: 96, targetH: 16,
			// height-limited: 16/100*16 ~= 2.56 -> 2x16 (rounded down by Lanczos)
			expectW: 2, expectH: 16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := newTestImage(tt.srcW, tt.srcH, color.RGBA{R: 255, A: 255})
			result := ResizeToFit(src, tt.targetW, tt.targetH)

			bounds := result.Bounds()
			gotW := bounds.Dx()
			gotH := bounds.Dy()

			if gotW > tt.targetW || gotH > tt.targetH {
				t.Errorf("result %dx%d exceeds target %dx%d", gotW, gotH, tt.targetW, tt.targetH)
			}
			if gotW != tt.expectW || gotH != tt.expectH {
				t.Errorf("expected %dx%d, got %dx%d", tt.expectW, tt.expectH, gotW, gotH)
			}
		})
	}
}

func TestResizeExact(t *testing.T) {
	tests := []struct {
		name             string
		srcW, srcH       int
		targetW, targetH int
	}{
		{"200x100 to 96x16", 200, 100, 96, 16},
		{"10x10 to 96x16", 10, 10, 96, 16},
		{"1x1 to 96x16", 1, 1, 96, 16},
		{"96x16 to 96x16 (identity)", 96, 16, 96, 16},
		{"large to small", 500, 500, 32, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := newTestImage(tt.srcW, tt.srcH, color.RGBA{G: 128, A: 255})
			result := ResizeExact(src, tt.targetW, tt.targetH)

			bounds := result.Bounds()
			if bounds.Dx() != tt.targetW || bounds.Dy() != tt.targetH {
				t.Errorf("expected %dx%d, got %dx%d", tt.targetW, tt.targetH, bounds.Dx(), bounds.Dy())
			}
		})
	}
}

func TestRotate(t *testing.T) {
	tests := []struct {
		name           string
		srcW, srcH     int
		angle          int
		expectW, expectH int
	}{
		{"0 degrees (no-op)", 96, 16, 0, 96, 16},
		{"90 degrees", 96, 16, 90, 16, 96},
		{"180 degrees", 96, 16, 180, 96, 16},
		{"270 degrees", 96, 16, 270, 16, 96},
		{"unsupported angle (clone)", 96, 16, 45, 96, 16},
		{"90 degrees square", 50, 50, 90, 50, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := newTestImage(tt.srcW, tt.srcH, color.RGBA{B: 200, A: 255})
			result := Rotate(src, tt.angle)

			bounds := result.Bounds()
			if bounds.Dx() != tt.expectW || bounds.Dy() != tt.expectH {
				t.Errorf("expected %dx%d, got %dx%d", tt.expectW, tt.expectH, bounds.Dx(), bounds.Dy())
			}
		})
	}
}

func TestFlip(t *testing.T) {
	// Use a checkerboard so we can verify pixel positions change.
	red := color.RGBA{R: 255, A: 255}
	blue := color.RGBA{B: 255, A: 255}

	tests := []struct {
		name string
		mode models.FlipMode
	}{
		{"none", models.FlipModeNone},
		{"horizontal", models.FlipModeHorizontal},
		{"vertical", models.FlipModeVertical},
		{"both", models.FlipModeBoth},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := newCheckerImage(4, 4, red, blue)
			result := Flip(src, tt.mode)

			bounds := result.Bounds()
			if bounds.Dx() != 4 || bounds.Dy() != 4 {
				t.Fatalf("expected 4x4, got %dx%d", bounds.Dx(), bounds.Dy())
			}

			// Verify pixels actually moved (or didn't, for FlipNone).
			srcTopLeft := src.RGBAAt(0, 0)
			resultTopLeft := result.NRGBAAt(0, 0)
			srcBottomRight := src.RGBAAt(3, 3)

			switch tt.mode {
			case models.FlipModeNone:
				// Top-left should remain the same.
				if resultTopLeft.R != srcTopLeft.R || resultTopLeft.B != srcTopLeft.B {
					t.Error("FlipNone changed top-left pixel")
				}
			case models.FlipModeHorizontal:
				// Top-left becomes top-right of source.
				srcTopRight := src.RGBAAt(3, 0)
				if resultTopLeft.R != srcTopRight.R || resultTopLeft.B != srcTopRight.B {
					t.Error("FlipH: top-left should match source top-right")
				}
			case models.FlipModeVertical:
				// Top-left becomes bottom-left of source.
				srcBottomLeft := src.RGBAAt(0, 3)
				if resultTopLeft.R != srcBottomLeft.R || resultTopLeft.B != srcBottomLeft.B {
					t.Error("FlipV: top-left should match source bottom-left")
				}
			case models.FlipModeBoth:
				// Top-left becomes bottom-right of source.
				if resultTopLeft.R != srcBottomRight.R || resultTopLeft.B != srcBottomRight.B {
					t.Error("FlipBoth: top-left should match source bottom-right")
				}
			}
		})
	}
}

func TestImageToRGBA(t *testing.T) {
	tests := []struct {
		name   string
		w, h   int
		fill   color.RGBA
		checkR uint8
		checkG uint8
		checkB uint8
		checkA uint8
	}{
		{"red 2x2", 2, 2, color.RGBA{R: 255, A: 255}, 255, 0, 0, 255},
		{"green 3x1", 3, 1, color.RGBA{G: 200, A: 255}, 0, 200, 0, 255},
		{"blue 1x3", 1, 3, color.RGBA{B: 128, A: 128}, 0, 0, 128, 128},
		{"white 1x1", 1, 1, color.RGBA{R: 255, G: 255, B: 255, A: 255}, 255, 255, 255, 255},
		{"black 2x2", 2, 2, color.RGBA{A: 255}, 0, 0, 0, 255},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := newTestImage(tt.w, tt.h, tt.fill)
			pixels := ImageToRGBA(img)

			expectedCount := tt.w * tt.h
			if len(pixels) != expectedCount {
				t.Fatalf("expected %d pixels, got %d", expectedCount, len(pixels))
			}

			for i, px := range pixels {
				if px.R != tt.checkR || px.G != tt.checkG || px.B != tt.checkB || px.A != tt.checkA {
					t.Errorf("pixel[%d]: expected RGBA(%d,%d,%d,%d), got RGBA(%d,%d,%d,%d)",
						i, tt.checkR, tt.checkG, tt.checkB, tt.checkA,
						px.R, px.G, px.B, px.A)
					break
				}
			}
		})
	}
}

func TestImageToRGBA_MultiColor(t *testing.T) {
	// 2x1 image: pixel(0,0)=red, pixel(1,0)=blue
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	img.SetRGBA(1, 0, color.RGBA{B: 255, A: 255})

	pixels := ImageToRGBA(img)
	if len(pixels) != 2 {
		t.Fatalf("expected 2 pixels, got %d", len(pixels))
	}
	if pixels[0].R != 255 || pixels[0].B != 0 {
		t.Errorf("pixel[0] should be red, got RGBA(%d,%d,%d,%d)", pixels[0].R, pixels[0].G, pixels[0].B, pixels[0].A)
	}
	if pixels[1].B != 255 || pixels[1].R != 0 {
		t.Errorf("pixel[1] should be blue, got RGBA(%d,%d,%d,%d)", pixels[1].R, pixels[1].G, pixels[1].B, pixels[1].A)
	}
}

func TestDecodeImage(t *testing.T) {
	tests := []struct {
		name    string
		encode  func() []byte
		wantErr bool
	}{
		{
			name: "valid PNG",
			encode: func() []byte {
				img := newTestImage(10, 10, color.RGBA{R: 100, G: 50, B: 200, A: 255})
				var buf bytes.Buffer
				if err := png.Encode(&buf, img); err != nil {
					panic(err)
				}
				return buf.Bytes()
			},
			wantErr: false,
		},
		{
			name: "invalid data",
			encode: func() []byte {
				return []byte("not an image")
			},
			wantErr: true,
		},
		{
			name: "empty data",
			encode: func() []byte {
				return []byte{}
			},
			wantErr: true,
		},
		{
			name: "truncated PNG",
			encode: func() []byte {
				img := newTestImage(10, 10, color.RGBA{R: 100, A: 255})
				var buf bytes.Buffer
				if err := png.Encode(&buf, img); err != nil {
					panic(err)
				}
				data := buf.Bytes()
				return data[:len(data)/2]
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := tt.encode()
			img, err := DecodeImage(data)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if img == nil {
				t.Fatal("expected non-nil image")
			}

			bounds := img.Bounds()
			if bounds.Dx() != 10 || bounds.Dy() != 10 {
				t.Errorf("expected 10x10, got %dx%d", bounds.Dx(), bounds.Dy())
			}
		})
	}
}

func TestDecodeImage_RoundTrip(t *testing.T) {
	// Encode a known image, decode it, and verify pixel content.
	original := newTestImage(4, 4, color.RGBA{R: 200, G: 100, B: 50, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, original); err != nil {
		t.Fatalf("encoding PNG: %v", err)
	}

	decoded, err := DecodeImage(buf.Bytes())
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}

	bounds := decoded.Bounds()
	if bounds.Dx() != 4 || bounds.Dy() != 4 {
		t.Fatalf("expected 4x4, got %dx%d", bounds.Dx(), bounds.Dy())
	}

	// Check the center pixel.
	r, g, b, a := decoded.At(2, 2).RGBA()
	if uint8(r>>8) != 200 || uint8(g>>8) != 100 || uint8(b>>8) != 50 || uint8(a>>8) != 255 {
		t.Errorf("pixel mismatch at (2,2): got RGBA(%d,%d,%d,%d)",
			uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8))
	}
}
