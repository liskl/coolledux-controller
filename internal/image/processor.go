package ledimage

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"

	"github.com/disintegration/imaging"
	"github.com/liskl/coolledux-controller/internal/models"

	_ "golang.org/x/image/bmp"
)

// ResizeToFit scales img to fit within width x height while preserving
// the aspect ratio. The result may be smaller than the target on one axis.
func ResizeToFit(img image.Image, width, height int) *image.NRGBA {
	return imaging.Fit(img, width, height, imaging.Lanczos)
}

// ResizeExact scales img to exactly width x height, ignoring aspect ratio.
// Uses nearest-neighbor on identity resizes to avoid Lanczos ringing that
// corrupts pure-color pixels into off-channel values on 1:1 passes.
func ResizeExact(img image.Image, width, height int) *image.NRGBA {
	bounds := img.Bounds()
	if bounds.Dx() == width && bounds.Dy() == height {
		return imaging.Clone(img)
	}
	return imaging.Resize(img, width, height, imaging.Lanczos)
}

// Letterbox preserves the source aspect ratio: it resizes to fit fully within
// width x height, then centers the result on a black canvas of that size.
// Empty regions (the "bars") are pure black so they read as off pixels on
// the matrix.
func Letterbox(img image.Image, width, height int) *image.NRGBA {
	fitted := imaging.Fit(img, width, height, imaging.Lanczos)
	canvas := imaging.New(width, height, color.NRGBA{0, 0, 0, 255})
	x := (width - fitted.Bounds().Dx()) / 2
	y := (height - fitted.Bounds().Dy()) / 2
	return imaging.Paste(canvas, fitted, image.Pt(x, y))
}

// Cover preserves the source aspect ratio while filling width x height,
// cropping the overflow from the center.
func Cover(img image.Image, width, height int) *image.NRGBA {
	return imaging.Fill(img, width, height, imaging.Center, imaging.Lanczos)
}

// FitMode selects how source images are mapped to the matrix dimensions.
type FitMode int

const (
	// FitLetterbox preserves aspect ratio with black bars (default).
	FitLetterbox FitMode = iota
	// FitStretch ignores aspect ratio (matches the historic ResizeExact).
	FitStretch
	// FitCover preserves aspect ratio and crops overflow from the center.
	FitCover
)

// ParseFitMode parses a fit-mode string; empty selects the default
// (FitLetterbox). Recognised values: "letterbox", "stretch", "cover".
func ParseFitMode(s string) (FitMode, error) {
	switch s {
	case "", "letterbox":
		return FitLetterbox, nil
	case "stretch":
		return FitStretch, nil
	case "cover":
		return FitCover, nil
	default:
		return 0, fmt.Errorf("unknown fit mode %q (want letterbox|stretch|cover)", s)
	}
}

// ResizeForMatrix applies the chosen fit mode and returns an NRGBA image of
// exactly width x height suitable for column-major encoding.
func ResizeForMatrix(img image.Image, width, height int, fit FitMode) *image.NRGBA {
	switch fit {
	case FitStretch:
		return ResizeExact(img, width, height)
	case FitCover:
		return Cover(img, width, height)
	default:
		return Letterbox(img, width, height)
	}
}

// Rotate rotates img by the given angle in degrees. Only 0, 90, 180, and 270
// are supported.
func Rotate(img image.Image, angle int) *image.NRGBA {
	switch angle {
	case 90:
		return imaging.Rotate90(img)
	case 180:
		return imaging.Rotate180(img)
	case 270:
		return imaging.Rotate270(img)
	default:
		return imaging.Clone(img)
	}
}

// Flip mirrors img according to the given FlipMode.
func Flip(img image.Image, mode models.FlipMode) *image.NRGBA {
	switch mode {
	case models.FlipModeHorizontal:
		return imaging.FlipH(img)
	case models.FlipModeVertical:
		return imaging.FlipV(img)
	case models.FlipModeBoth:
		return imaging.FlipV(imaging.FlipH(img))
	default:
		return imaging.Clone(img)
	}
}

// ImageToRGBA extracts all pixels from img as a flat row-major slice of
// color.RGBA values.
func ImageToRGBA(img image.Image) []color.RGBA {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	pixels := make([]color.RGBA, w*h)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			idx := (y-bounds.Min.Y)*w + (x - bounds.Min.X)
			pixels[idx] = color.RGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: uint8(a >> 8),
			}
		}
	}
	return pixels
}

// DecodeImage decodes image data from PNG, JPEG, or BMP bytes.
func DecodeImage(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}
	return img, nil
}
