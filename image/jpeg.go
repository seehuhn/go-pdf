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
)

// EmbedJPEG writes the image src to the PDF file w, using lossy compression.
func EmbedJPEG(w pdf.Putter, src image.Image, opts *jpeg.Options, resName pdf.Name) (Embedded, error) {
	im, err := JPEG(src, opts)
	if err != nil {
		return nil, err
	}
	return im.Embed(w, resName)
}

func JPEG(src image.Image, opts *jpeg.Options) (Image, error) {
	// convert to NRGBA format
	b := src.Bounds()
	img := image.NewNRGBA(b)
	draw.Draw(img, img.Bounds(), src, b.Min, draw.Src)

	im := &jpegImage{
		NRGBA: &image.NRGBA{},
		opts:  opts,
	}
	return im, nil
}

type jpegImage struct {
	*image.NRGBA
	opts *jpeg.Options
}

func (im *jpegImage) Embed(w pdf.Putter, resName pdf.Name) (Embedded, error) {
	ref := w.Alloc()

	// TODO(voss): write a mask if there is an alpha channel

	img := im.NRGBA
	stream, err := w.OpenStream(ref, pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(img.Bounds().Dx()),
		"Height":           pdf.Integer(img.Bounds().Dy()),
		"ColorSpace":       pdf.Name("DeviceRGB"),
		"BitsPerComponent": pdf.Integer(8),
		"Filter":           pdf.Name("DCTDecode"),
	})
	if err != nil {
		return nil, err
	}

	err = jpeg.Encode(stream, img, im.opts)
	if err != nil {
		return nil, err
	}

	err = stream.Close()
	if err != nil {
		return nil, err
	}

	return &jpegEmbedded{
		jpegImage: im,
		ref:       ref,
		resName:   resName,
	}, nil
}

type jpegEmbedded struct {
	*jpegImage
	ref     pdf.Reference
	resName pdf.Name
}

func (e *jpegEmbedded) Reference() pdf.Reference {
	return e.ref
}

func (e *jpegEmbedded) ResourceName() pdf.Name {
	return e.resName
}
