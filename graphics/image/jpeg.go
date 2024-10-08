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
	"image/draw"
	"image/jpeg"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
)

// JPEG creates a new PDF image from a JPEG image.
// The file is stored in the DCTDecode format in the PDF file.
func JPEG(src image.Image, opts *jpeg.Options) (Image, error) {
	// convert to NRGBA format
	b := src.Bounds()
	img := image.NewNRGBA(b)
	draw.Draw(img, img.Bounds(), src, b.Min, draw.Src)

	im := &jpegImage{
		im:   img,
		opts: opts,
	}
	return im, nil
}

type jpegImage struct {
	im   *image.NRGBA
	opts *jpeg.Options
}

// Subtype returns /Image.
// This implements the [graphics.XObject] interface.
func (im *jpegImage) Subtype() pdf.Name {
	return pdf.Name("Image")
}

// Bounds implements the [Image] interface.
func (im *jpegImage) Bounds() Rectangle {
	b := im.im.Bounds()
	return Rectangle{XMin: b.Min.X, YMin: b.Min.Y, XMax: b.Max.X, YMax: b.Max.Y}
}

// Embed ensures that the image is embedded in the PDF file.
// This implements the [Image] interface.
func (im *jpegImage) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	ref := rm.Out.Alloc()

	// TODO(voss): write a mask if there is an alpha channel

	img := im.im
	stream, err := rm.Out.OpenStream(ref, pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(img.Bounds().Dx()),
		"Height":           pdf.Integer(img.Bounds().Dy()),
		"ColorSpace":       pdf.Name(color.FamilyDeviceRGB),
		"BitsPerComponent": pdf.Integer(8),
		"Filter":           pdf.Name("DCTDecode"),
	})
	if err != nil {
		return nil, zero, err
	}

	err = jpeg.Encode(stream, img, im.opts)
	if err != nil {
		return nil, zero, err
	}

	err = stream.Close()
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}
