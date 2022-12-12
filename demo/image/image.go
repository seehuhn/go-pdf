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
	_ "image/png"
	"log"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	pdfimage "seehuhn.de/go/pdf/image"
	"seehuhn.de/go/pdf/pages2"
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

func imagePage(img *image.NRGBA) error {
	out, err := pdf.Create("test.pdf")
	if err != nil {
		return err
	}

	imageRef, err := pdfimage.EmbedAsJPEG(out, img, nil, nil)
	if err != nil {
		return err
	}

	pageTree := pages2.NewTree(out, nil)

	b := img.Bounds()
	pageBox := &pdf.Rectangle{
		URx: float64(b.Dx()) / dpi * 72,
		URy: float64(b.Dy()) / dpi * 72,
	}

	page, err := graphics.NewPage(out)
	if err != nil {
		return err
	}

	page.Scale(pageBox.URx, pageBox.URy)
	page.DrawImage(imageRef)

	dict, err := page.Close()
	if err != nil {
		return err
	}
	dict["MediaBox"] = pageBox

	_, err = pageTree.AppendPage(dict)
	if err != nil {
		return err
	}

	rootRef, err := pageTree.Close()
	if err != nil {
		return err
	}

	out.Catalog.Pages = rootRef
	out.Catalog.ViewerPreferences = pdf.Dict{
		"FitWindow":    pdf.Bool(true),
		"HideWindowUI": pdf.Bool(true),
	}
	return out.Close()
}

func main() {
	img, err := readImage(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	err = imagePage(img)
	if err != nil {
		log.Fatal(err)
	}
}
