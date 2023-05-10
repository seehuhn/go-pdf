// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

// Package image provides functions for embedding images in PDF files.
package image

import (
	"image"
	"image/draw"
	"image/jpeg"

	"seehuhn.de/go/pdf"
)

// EmbedAsJPEG writes the image src to the PDF file w, using lossy compression.
func EmbedAsJPEG(w *pdf.Writer, ref pdf.Reference, src image.Image, opts *jpeg.Options) error {
	// convert to NRGBA format
	// TODO(voss): needed????
	b := src.Bounds()
	img := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(img, img.Bounds(), src, b.Min, draw.Src)

	// TODO(voss): write a mask if there is an alpha channel
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
		return err
	}

	err = jpeg.Encode(stream, img, opts)
	if err != nil {
		return err
	}

	err = stream.Close()
	if err != nil {
		return err
	}

	return nil
}

// EmbedAsPNG writes the image img to the PDF file w, using a lossless representation
// very similar to the PNG format.
func EmbedAsPNG(w *pdf.Writer, ref pdf.Reference, src image.Image) error {
	width := src.Bounds().Dx()
	height := src.Bounds().Dy()
	filter := &pdf.FilterInfo{
		Name: pdf.FlateDecode,
		Parms: pdf.Dict{
			"Columns":   pdf.Integer(width),
			"Colors":    pdf.Integer(3),
			"Predictor": pdf.Integer(15),
		},
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
		return err
	}
	alpha := make([]byte, 0, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, a := src.At(x, y).RGBA()
			_, err = stream.Write([]byte{byte(r >> 8), byte(g >> 8), byte(b >> 8)})
			if err != nil {
				return err
			}
			alpha = append(alpha, byte(a>>8))
		}
	}
	err = stream.Close()
	if err != nil {
		return err
	}

	// TODO(voss): is there a more appropriate compression type for the mask?
	filter = &pdf.FilterInfo{
		Name: pdf.FlateDecode,
		Parms: pdf.Dict{
			"Columns":   pdf.Integer(width),
			"Predictor": pdf.Integer(15),
		},
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
		return err
	}
	_, err = stream.Write(alpha)
	if err != nil {
		return err
	}
	err = stream.Close()
	if err != nil {
		return err
	}

	return nil
}
