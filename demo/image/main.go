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
	pdfimage "seehuhn.de/go/pdf/image"
	"seehuhn.de/go/pdf/simple"
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

type imageWrapper struct {
	ref *pdf.Reference
}

func (im *imageWrapper) Reference() *pdf.Reference {
	return im.ref
}

func (im *imageWrapper) ResourceName() pdf.Name {
	return "I"
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
	doc, err := simple.CreateSinglePage("test.pdf", width, height)
	if err != nil {
		log.Fatal(err)
	}

	imageRef, err := pdfimage.EmbedAsJPEG(doc.Out, img, nil, nil)
	if err != nil {
		log.Fatal(err)
	}

	doc.Scale(width, height)
	doc.DrawImage(&imageWrapper{imageRef})

	doc.Out.Catalog.ViewerPreferences = pdf.Dict{
		"FitWindow":    pdf.Bool(true),
		"HideWindowUI": pdf.Bool(true),
	}

	err = doc.Close()
	if err != nil {
		log.Fatal(err)
	}
}
