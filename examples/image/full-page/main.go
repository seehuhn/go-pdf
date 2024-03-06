// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	pdfimage "seehuhn.de/go/pdf/graphics/image"
	"seehuhn.de/go/pdf/graphics/matrix"
)

const dpi = 300

func readImage(fname string) (*image.NRGBA, error) {
	fd, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	src, _, err := image.Decode(fd)
	if err != nil {
		return nil, err
	}

	// convert to NRGBA format
	b := src.Bounds()
	img := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(img, img.Bounds(), src, b.Min, draw.Src)
	return img, nil
}

func main() {
	name := os.Args[1]

	img, err := readImage(name)
	if err != nil {
		log.Fatal(err)
	}

	b := img.Bounds()
	width := float64(b.Dx()) / dpi * 72
	height := float64(b.Dy()) / dpi * 72
	paper := &pdf.Rectangle{
		URx: width,
		URy: height,
	}
	doc, err := document.CreateSinglePage("test.pdf", paper, pdf.V1_7, nil)
	if err != nil {
		log.Fatal(err)
	}

	embedded, err := pdfimage.EmbedJPEG(doc.Out, img, nil, "I")
	if err != nil {
		log.Fatal(err)
	}

	doc.Transform(matrix.Scale(width, height))
	doc.DrawXObject(embedded)

	doc.Out.GetMeta().Catalog.ViewerPreferences = pdf.Dict{
		"FitWindow":    pdf.Boolean(true),
		"HideWindowUI": pdf.Boolean(true),
	}

	err = doc.Close()
	if err != nil {
		log.Fatal(err)
	}
}
