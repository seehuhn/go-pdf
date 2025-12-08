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

package lab

import (
	"io"

	"seehuhn.de/go/pdf"
	pdfcolor "seehuhn.de/go/pdf/graphics/color"
	pdfimage "seehuhn.de/go/pdf/graphics/image"
)

type Lab8 struct {
	Width  int
	Height int

	// PixData holds the image pixel data in Lab color space.
	// Each pixel is represented by 3 consecutive uint8 values: L, a, and b.
	PixData []uint8
}

func NewLab8(width, height int) *Lab8 {
	return &Lab8{
		Width:   width,
		Height:  height,
		PixData: make([]uint8, width*height*3),
	}
}

// Subtype returns the PDF XObject subtype for images.
func (im *Lab8) Subtype() pdf.Name {
	return pdf.Name("Image")
}

// Embed converts the Go representation of the object into a PDF object,
// corresponding to the PDF version of the output file.
//
// The return value is the PDF representation of the object.
// If the object is embedded in the PDF file, this may be a [Reference].
func (im *Lab8) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	cs, err := pdfcolor.Lab(pdfcolor.WhitePointD65, nil, nil)
	if err != nil {
		return nil, err
	}
	dict := &pdfimage.Dict{
		Width:            im.Width,
		Height:           im.Height,
		ColorSpace:       cs,
		BitsPerComponent: 8,
		Decode:           []float64{0, 100, -100, 100, -100, 100},
		WriteData: func(w io.Writer) error {
			_, err := w.Write(im.PixData)
			return err
		},
		Interpolate: true,
	}
	return dict.Embed(e)
}
