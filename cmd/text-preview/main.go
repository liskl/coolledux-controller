package main

import (
	"fmt"
	"image"
	"image/color"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"github.com/liskl/coolledux-controller/internal/text"
)

func main() {
	for _, name := range []string{"7x13", "7x14b", "8x16"} {
		face, _ := text.Face(name)
		m := face.Metrics()
		fmt.Printf("\n== font %s == ascent=%d descent=%d height=%d\n",
			name, m.Ascent.Ceil(), m.Descent.Ceil(), m.Height.Ceil())
		// Try different baselines
		for baseline := 10; baseline <= 16; baseline++ {
			img := image.NewRGBA(image.Rect(0, 0, 96, 16))
			d := &font.Drawer{
				Dst:  img,
				Src:  image.NewUniform(color.RGBA{0, 255, 0, 255}),
				Face: face,
				Dot:  fixed.P(0, baseline),
			}
			d.DrawString("HI")
			// Find vertical extent
			minY, maxY := 16, -1
			for y := 0; y < 16; y++ {
				for x := 0; x < 96; x++ {
					r, g, b, _ := img.At(x, y).RGBA()
					if r|g|b != 0 {
						if y < minY { minY = y }
						if y > maxY { maxY = y }
					}
				}
			}
			fmt.Printf("  baseline=%d  ink y=[%d..%d]\n", baseline, minY, maxY)
		}
	}
}
