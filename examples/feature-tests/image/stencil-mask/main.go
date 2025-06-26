// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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
	"fmt"
	goimg "image"
	gocol "image/color"
	"math"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/image"
	"seehuhn.de/go/pdf/graphics/text"
	"seehuhn.de/go/pdf/internal/gibberish"
)

func main() {
	err := run("test.pdf")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("wrote test.pdf")
}

func run(filename string) error {
	page, err := document.CreateSinglePage(filename, document.A5, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	gray := color.DeviceGray(0.5)

	F := text.F{
		Font:  standard.TimesRoman.New(),
		Size:  12,
		Color: gray,
	}

	text.Show(page.Writer,
		text.M{X: 20, Y: 570},
		F,
		text.Wrap(370, gibberish.Generate(480, 0)),
	)

	page.SetFillColor(color.Blue)

	page.PushGraphicsState()
	circle1 := image.FromImageMask(Circle(17))
	page.Transform(matrix.Translate(36, 452))
	page.Transform(matrix.Scale(100, 100))
	page.DrawXObject(circle1)
	page.PopGraphicsState()

	page.PushGraphicsState()
	circle2 := image.FromImageMask(Circle(70))
	circle2.Interpolate = true
	page.Transform(matrix.Translate(186, 400))
	page.Transform(matrix.Scale(100, 100))
	page.DrawXObject(circle2)
	page.PopGraphicsState()

	err = page.Close()
	if err != nil {
		return err
	}
	return nil
}

func Circle(n int) goimg.Image {
	// Create a new alpha image of size n×n
	img := goimg.NewAlpha(goimg.Rect(0, 0, n, n))

	// Calculate center and radius
	center := float64(n) / 2.0
	radius := float64(n) / 2.0

	// Iterate through each pixel
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			// Calculate distance from center of pixel to center of image
			dx := float64(x) + 0.5 - center
			dy := float64(y) + 0.5 - center
			distance := math.Sqrt(dx*dx + dy*dy)

			// If pixel is within the circle, make it opaque
			if distance <= radius {
				img.SetAlpha(x, y, gocol.Alpha{255}) // fully opaque
			}
			// Pixels outside the circle remain transparent (default alpha = 0)
		}
	}

	return img
}
