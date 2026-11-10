// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package lab

import (
	"io"

	"seehuhn.de/go/pdf"
	pdfcolor "seehuhn.de/go/pdf/graphics/color"
	pdfimage "seehuhn.de/go/pdf/graphics/image"
)

// PDF 2.0 sections: 8.9.5

// Lab8 represents an image in the Lab color space with 8 bits per component.
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

// ResourceName returns the empty string: Lab8 does not expose a Name
// field.  Callers who need a specific resource-dict key should wrap
// the image in a [seehuhn.de/go/pdf/graphics/image.Dict] and set its
// Name field.  See [graphics.XObject.ResourceName].
func (im *Lab8) ResourceName() pdf.Name {
	return ""
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
		Data: pdfimage.NewFlateSource(im.Width, cs, 8,
			func(w io.Writer) error {
				_, err := w.Write(im.PixData)
				return err
			}),
		Interpolate: true,
	}
	return dict.Embed(e)
}
