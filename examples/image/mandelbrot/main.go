// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"image"
	"image/color"
	"log"
	"math"

	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/type1"
	pdfimage "seehuhn.de/go/pdf/graphics/image"
	"seehuhn.de/go/pdf/graphics/matrix"
)

func main() {
	err := run("out.pdf")
	if err != nil {
		log.Fatal(err)
	}
}

func run(fname string) error {
	paper := document.A4
	page, err := document.CreateSinglePage(fname, paper, nil)
	if err != nil {
		return err
	}

	raw := mandelbrot()
	img, err := pdfimage.EmbedPNG(page.Out, raw, "I")
	if err != nil {
		return err
	}

	b := raw.Bounds()
	q := float64(b.Dx()) / float64(b.Dy())
	width := 0.75 * (paper.URx - paper.LLx)
	height := width / q
	left := (paper.URx - paper.LLx - width) / 2
	bottom := paper.URy - 72 - height

	page.PushGraphicsState()
	page.Transform(matrix.Translate(left, bottom))
	page.Transform(matrix.Scale(width, height))
	page.DrawXObject(img)
	page.PopGraphicsState()

	roman, err := type1.TimesRoman.Embed(page.Out, &font.Options{ResName: "R"})
	if err != nil {
		return err
	}
	bold, err := type1.TimesBold.Embed(page.Out, &font.Options{ResName: "B"})
	if err != nil {
		return err
	}
	page.TextStart()
	page.TextFirstLine(72, bottom-20)
	page.TextSetFont(bold, 10)
	page.TextShow("Figure 1.")
	page.TextSetFont(roman, 10)
	gg := page.TextLayout(" A graphical depiction of the Mandelbrot set.")
	// make the leading space wider than normal
	gg.Seq[0].Advance = gg.Seq[0].Advance * 3
	page.TextShowGlyphs(gg)
	page.TextEnd()

	return page.Close()
}

func mandelbrot() image.Image {
	const (
		xmin, ymin, xmax, ymax = -2.5, -1.5, +1.5, +1.5
		width, height          = 1536, 1152
	)
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for i := 0; i < width; i++ {
	yLoop:
		for j := 0; j < height; j++ {
			x := xmin + (xmax-xmin)*float64(i)/width
			y := ymin + (ymax-ymin)*float64(j)/height
			z := complex(x, y)

			maxDepth := 200
			var v complex128
			for n := 0; n < maxDepth; n++ {
				v = v*v + z
				if (real(v)*real(v) + imag(v)*imag(v)) > 4 {
					img.Set(i, j, palette(n, maxDepth))
					continue yLoop
				}
			}
			img.Set(i, j, color.RGBA{0, 0, 0, 255})
		}
	}

	return img
}

// A nice curve through RGB color space.
// pos < max, large value of pos means darker color
func palette(pos, max int) color.Color {
	q := 1 - float64(pos)/float64(max-1)
	r := uint8(220 * math.Pow(q, 1.3))
	g := uint8(230 * math.Pow(q, 3.2))
	b := uint8(220 * math.Pow(q, 0.9))
	if pos%2 == 0 {
		r += 20
		g += 20
		b += 20
	}
	return color.RGBA{r, g, b, 255}
}
