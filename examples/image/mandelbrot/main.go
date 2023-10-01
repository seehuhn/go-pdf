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

	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/type1"
	pdfimage "seehuhn.de/go/pdf/image"
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
	page.Translate(left, bottom)
	page.Scale(width, height)
	page.DrawImage(img)
	page.PopGraphicsState()

	roman, err := type1.TimesRoman.Embed(page.Out, "R")
	if err != nil {
		return err
	}
	bold, err := type1.TimesBold.Embed(page.Out, "B")
	if err != nil {
		return err
	}
	page.TextStart()
	page.TextFirstLine(72, bottom-15)
	page.TextSetFont(bold, 10)
	page.TextShow("Figure 1. ")
	page.TextSetFont(roman, 10)
	page.TextShow("A graphical depiction of the Mandelbrot set.")
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
					break
				}
			}
		}
	}

	return img
}

// A nice curve through RGB color space.
// pos < max, large value of pos means darker color
func palette(pos, max int) color.Color {
	q := float64(pos) / float64(max-1)
	r := uint8(245 * (1 - q))
	g := uint8(245 * (1 - q) * (1 - q))
	b := uint8(245 * (0.9 - 0.8*q*q*q))
	if pos%2 == 0 {
		r += 10
		g += 10
		b += 10
	}
	return color.RGBA{r, g, b, 255}
}