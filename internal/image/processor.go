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
func ResizeExact(img image.Image, width, height int) *image.NRGBA {
	return imaging.Resize(img, width, height, imaging.Lanczos)
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
