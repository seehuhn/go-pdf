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

package image

import (
	"image"

	"seehuhn.de/go/pdf"
)

// EmbedPNG writes the image `src` to the PDF file w, using a lossless
// representation very similar to the PNG format.
func EmbedPNG(w pdf.Putter, src image.Image, resName pdf.Name) (*Embedded, error) {
	im, err := PNG(src)
	if err != nil {
		return nil, err
	}
	return im.Embed(w, resName)
}

func PNG(src image.Image) (Image, error) {
	return &pngImage{src}, nil
}

type pngImage struct {
	im image.Image
}

// Bounds implements the [Image] interface.
func (im *pngImage) Bounds() Rectangle {
	b := im.im.Bounds()
	return Rectangle{XMin: b.Min.X, YMin: b.Min.Y, XMax: b.Max.X, YMax: b.Max.Y}
}

func (im *pngImage) Embed(w pdf.Putter, defaultName pdf.Name) (*Embedded, error) {
	ref := w.Alloc()
	src := im.im

	width := src.Bounds().Dx()
	height := src.Bounds().Dy()
	filter := pdf.FilterFlate{
		"Columns":   pdf.Integer(width),
		"Colors":    pdf.Integer(3),
		"Predictor": pdf.Integer(15),
	}
	// TODO(voss): only write the mask if there is an alpha channel
	maskRef := w.Alloc()
	stream, err := w.OpenStream(ref, pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(width),
		"Height":           pdf.Integer(height),
		"ColorSpace":       pdf.Name("DeviceRGB"),
		"BitsPerComponent": pdf.Integer(8),
		"SMask":            maskRef,
	}, filter)
	if err != nil {
		return nil, err
	}
	alpha := make([]byte, 0, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, a := src.At(x, y).RGBA()
			_, err = stream.Write([]byte{byte(r >> 8), byte(g >> 8), byte(b >> 8)})
			if err != nil {
				return nil, err
			}
			alpha = append(alpha, byte(a>>8))
		}
	}
	err = stream.Close()
	if err != nil {
		return nil, err
	}

	// TODO(voss): is there a more appropriate compression type for the mask?
	filter = pdf.FilterFlate{
		"Columns":   pdf.Integer(width),
		"Predictor": pdf.Integer(15),
	}
	stream, err = w.OpenStream(maskRef, pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(width),
		"Height":           pdf.Integer(height),
		"ColorSpace":       pdf.Name("DeviceGray"),
		"BitsPerComponent": pdf.Integer(8),
	}, filter)
	if err != nil {
		return nil, err
	}
	_, err = stream.Write(alpha)
	if err != nil {
		return nil, err
	}
	err = stream.Close()
	if err != nil {
		return nil, err
	}

	return &Embedded{
		Res: pdf.Res{
			DefName: defaultName,
			Data:    ref,
		},
	}, nil
}
