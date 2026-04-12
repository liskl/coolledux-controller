package ledimage

import (
	"bytes"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"testing"
)

// buildTestGIF creates an in-memory GIF with the given number of frames.
// Each frame is a solid color from the plan9 palette.
func buildTestGIF(t *testing.T, w, h, numFrames int, delays []int) []byte {
	t.Helper()

	g := &gif.GIF{
		Config: image.Config{
			Width:  w,
			Height: h,
		},
	}

	for i := 0; i < numFrames; i++ {
		// Create a paletted image for each frame.
		rect := image.Rect(0, 0, w, h)
		pal := palette.Plan9
		paletted := image.NewPaletted(rect, pal)

		// Fill with a color that varies per frame.
		fillColor := pal[i%len(pal)]
		draw.Draw(paletted, rect, &image.Uniform{fillColor}, image.Point{}, draw.Src)

		g.Image = append(g.Image, paletted)
		if i < len(delays) {
			g.Delay = append(g.Delay, delays[i])
		} else {
			g.Delay = append(g.Delay, 10) // 10 centiseconds = 100ms
		}
		g.Disposal = append(g.Disposal, gif.DisposalNone)
	}

	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatalf("encoding test GIF: %v", err)
	}
	return buf.Bytes()
}

func TestDecodeGIF(t *testing.T) {
	tests := []struct {
		name    string
		data    func(t *testing.T) []byte
		wantErr bool
		frames  int
	}{
		{
			name: "valid 1-frame GIF",
			data: func(t *testing.T) []byte {
				return buildTestGIF(t, 10, 10, 1, []int{10})
			},
			wantErr: false,
			frames:  1,
		},
		{
			name: "valid 3-frame GIF",
			data: func(t *testing.T) []byte {
				return buildTestGIF(t, 20, 20, 3, []int{5, 10, 15})
			},
			wantErr: false,
			frames:  3,
		},
		{
			name: "invalid data",
			data: func(t *testing.T) []byte {
				return []byte("not a gif at all")
			},
			wantErr: true,
		},
		{
			name: "empty data",
			data: func(t *testing.T) []byte {
				return []byte{}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := tt.data(t)
			g, err := DecodeGIF(data)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if g == nil {
				t.Fatal("expected non-nil GIF")
			}
			if len(g.Image) != tt.frames {
				t.Errorf("expected %d frames, got %d", tt.frames, len(g.Image))
			}
		})
	}
}

func TestExtractFrames(t *testing.T) {
	tests := []struct {
		name          string
		numFrames     int
		srcW, srcH    int
		targetW, targetH int
		delays        []int
		expectDelays  []uint16
	}{
		{
			name:      "2-frame GIF resized to 96x16",
			numFrames: 2,
			srcW:      40, srcH: 40,
			targetW:   96, targetH: 16,
			delays:    []int{5, 10},
			// centiseconds -> milliseconds: 5*10=50, 10*10=100
			expectDelays: []uint16{50, 100},
		},
		{
			name:      "single frame with zero delay uses default 100ms",
			numFrames: 1,
			srcW:      10, srcH: 10,
			targetW:   32, targetH: 8,
			delays:    []int{0},
			expectDelays: []uint16{100},
		},
		{
			name:      "3 frames with various delays",
			numFrames: 3,
			srcW:      20, srcH: 20,
			targetW:   16, targetH: 16,
			delays:    []int{2, 0, 50},
			// 2*10=20, 0->default 100, 50*10=500
			expectDelays: []uint16{20, 100, 500},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gifData := buildTestGIF(t, tt.srcW, tt.srcH, tt.numFrames, tt.delays)
			g, err := DecodeGIF(gifData)
			if err != nil {
				t.Fatalf("decoding test GIF: %v", err)
			}

			frames, delays := ExtractFrames(g, tt.targetW, tt.targetH)

			if len(frames) != tt.numFrames {
				t.Fatalf("expected %d frames, got %d", tt.numFrames, len(frames))
			}
			if len(delays) != tt.numFrames {
				t.Fatalf("expected %d delays, got %d", tt.numFrames, len(delays))
			}

			// Each frame should have targetW * targetH pixels.
			expectedPixels := tt.targetW * tt.targetH
			for i, frame := range frames {
				if len(frame) != expectedPixels {
					t.Errorf("frame[%d]: expected %d pixels, got %d", i, expectedPixels, len(frame))
				}
			}

			// Check delay values.
			for i, d := range delays {
				if d != tt.expectDelays[i] {
					t.Errorf("delay[%d]: expected %d, got %d", i, tt.expectDelays[i], d)
				}
			}
		})
	}
}

func TestExtractFrames_EmptyGIF(t *testing.T) {
	g := &gif.GIF{}
	frames, delays := ExtractFrames(g, 96, 16)
	if frames != nil {
		t.Errorf("expected nil frames, got %d", len(frames))
	}
	if delays != nil {
		t.Errorf("expected nil delays, got %d", len(delays))
	}
}

func TestExtractFrames_DisposalBackground(t *testing.T) {
	// Build a GIF where the first frame uses DisposalBackground.
	// After extracting, the canvas should have been cleared between frames.
	g := &gif.GIF{
		Config: image.Config{Width: 4, Height: 4},
	}

	pal := color.Palette{color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}}

	frame1 := image.NewPaletted(image.Rect(0, 0, 4, 4), pal)
	draw.Draw(frame1, frame1.Bounds(), &image.Uniform{pal[0]}, image.Point{}, draw.Src)
	g.Image = append(g.Image, frame1)
	g.Delay = append(g.Delay, 10)
	g.Disposal = append(g.Disposal, gif.DisposalBackground)

	frame2 := image.NewPaletted(image.Rect(0, 0, 4, 4), pal)
	draw.Draw(frame2, frame2.Bounds(), &image.Uniform{pal[1]}, image.Point{}, draw.Src)
	g.Image = append(g.Image, frame2)
	g.Delay = append(g.Delay, 10)
	g.Disposal = append(g.Disposal, gif.DisposalNone)

	frames, delays := ExtractFrames(g, 4, 4)
	if len(frames) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(frames))
	}
	if len(delays) != 2 {
		t.Fatalf("expected 2 delays, got %d", len(delays))
	}

	// Just verify we got pixel data without crashing. Exact values
	// depend on compositing, but frame count and pixel count matter.
	for i, frame := range frames {
		if len(frame) != 16 { // 4*4
			t.Errorf("frame[%d]: expected 16 pixels, got %d", i, len(frame))
		}
	}
}

func TestExtractFrames_NoConfigDimensions(t *testing.T) {
	// Build a GIF where Config.Width and Config.Height are both 0.
	// ExtractFrames should fall back to the first frame's bounds.
	g := &gif.GIF{
		Config: image.Config{Width: 0, Height: 0},
	}

	pal := color.Palette{color.RGBA{R: 255, A: 255}, color.RGBA{A: 255}}
	frame := image.NewPaletted(image.Rect(0, 0, 8, 8), pal)
	draw.Draw(frame, frame.Bounds(), &image.Uniform{pal[0]}, image.Point{}, draw.Src)
	g.Image = append(g.Image, frame)
	g.Delay = append(g.Delay, 5)
	g.Disposal = append(g.Disposal, gif.DisposalNone)

	frames, delays := ExtractFrames(g, 4, 4)
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(frames))
	}
	if len(delays) != 1 {
		t.Fatalf("expected 1 delay, got %d", len(delays))
	}
	if len(frames[0]) != 16 {
		t.Errorf("expected 16 pixels, got %d", len(frames[0]))
	}
}
