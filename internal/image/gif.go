package ledimage

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
)

// DecodeGIF decodes GIF data from raw bytes.
func DecodeGIF(data []byte) (*gif.GIF, error) {
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decoding gif: %w", err)
	}
	return g, nil
}

// ExtractFrames extracts each frame from a GIF as resized RGBA pixel slices.
// Each frame is composited onto a canvas to handle disposal methods correctly.
// Delays are returned in milliseconds (GIF stores them in centiseconds).
func ExtractFrames(g *gif.GIF, width, height int) (frames [][]color.RGBA, delays []uint16) {
	if len(g.Image) == 0 {
		return nil, nil
	}

	canvasWidth := g.Config.Width
	canvasHeight := g.Config.Height
	if canvasWidth == 0 || canvasHeight == 0 {
		bounds := g.Image[0].Bounds()
		canvasWidth = bounds.Dx()
		canvasHeight = bounds.Dy()
	}

	// Persistent canvas for frame compositing
	canvas := image.NewRGBA(image.Rect(0, 0, canvasWidth, canvasHeight))

	frames = make([][]color.RGBA, 0, len(g.Image))
	delays = make([]uint16, 0, len(g.Image))

	for i, frame := range g.Image {
		// Draw this frame onto the canvas
		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)

		// Resize the composited canvas to the target dimensions
		resized := ResizeExact(canvas, width, height)
		pixels := ImageToRGBA(resized)
		frames = append(frames, pixels)

		// GIF delays are in centiseconds; convert to milliseconds.
		// A delay of 0 typically means "use a default" (often 100ms).
		var delayMs uint16
		if i < len(g.Delay) && g.Delay[i] > 0 {
			delayMs = uint16(g.Delay[i]) * 10
		} else {
			delayMs = 100
		}
		delays = append(delays, delayMs)

		// Handle disposal method for next frame
		var disposal byte
		if i < len(g.Disposal) {
			disposal = g.Disposal[i]
		}
		switch disposal {
		case gif.DisposalBackground:
			// Clear the frame area to transparent
			draw.Draw(canvas, frame.Bounds(), image.Transparent, image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			// Restoring to previous is complex; for LED display purposes
			// we approximate by doing nothing (same as DisposalNone).
		}
	}

	return frames, delays
}
