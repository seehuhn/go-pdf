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

package annotation

import (
	"fmt"

	"seehuhn.de/go/pdf"
)

// Border represents the characteristics of an annotation's border.
type Border struct {
	// HCornerRadius is the horizontal corner radius.
	HCornerRadius float64

	// VCornerRadius is the vertical corner radius.
	VCornerRadius float64

	// Width is the border width in default user space units.
	// If 0, no border is drawn.
	Width float64

	// DashArray (optional; PDF 1.1) defines a pattern of dashes and gaps
	// for drawing the border. If nil, a solid border is drawn.
	DashArray []float64

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

var _ pdf.Embedder[pdf.Unused] = (*Border)(nil)

// PDFDefaultBorder is the default border values within PDF files.
// Using this for [Common.Border] slightly reduces file size.
var PDFDefaultBorder = &Border{Width: 1}

// ExtractBorder extracts a Border from a PDF array.
// If no border entry exists, returns the PDF default (solid border with width 1).
// If no border is to be drawn, returns nil.
func ExtractBorder(r pdf.Getter, obj pdf.Object) (*Border, error) {
	border, err := pdf.Optional(pdf.GetArray(r, obj))
	if err != nil {
		return nil, err
	}

	if len(border) < 3 {
		return PDFDefaultBorder, nil
	}

	b := &Border{}

	if h, err := pdf.Optional(pdf.GetNumber(r, border[0])); err != nil {
		return nil, err
	} else {
		b.HCornerRadius = float64(h)
	}

	if v, err := pdf.Optional(pdf.GetNumber(r, border[1])); err != nil {
		return nil, err
	} else {
		b.VCornerRadius = float64(v)
	}

	if w, err := pdf.Optional(pdf.GetNumber(r, border[2])); err != nil {
		return nil, err
	} else {
		b.Width = float64(w)
	}

	if b.Width <= 0 {
		return nil, nil // no border
	}

	if len(border) > 3 {
		if dashArray, err := pdf.Optional(pdf.GetFloatArray(r, border[3])); err != nil {
			return nil, err
		} else {
			// filter out negative values
			var dashes []float64
			for _, num := range dashArray {
				if num > 0 {
					dashes = append(dashes, num)
				}
			}
			if len(dashes) > 0 {
				b.DashArray = dashes
			}
		}
	}

	return b, nil
}

func (b *Border) Embed(rm *pdf.EmbedHelper) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	// the Go default value is "no border"
	if b == nil {
		return pdf.Array{pdf.Number(0), pdf.Number(0), pdf.Number(0)}, zero, nil
	}

	// if we have the PDF default value, we don't need to store anything
	if b.isPDFDefault() {
		return nil, zero, nil
	}

	if b.Width <= 0 {
		return nil, zero, fmt.Errorf("invalid border width %f", b.Width)
	}
	for _, v := range b.DashArray {
		if v <= 0 {
			return nil, zero, fmt.Errorf("invalid dash value %f", v)
		}
	}

	borderArray := pdf.Array{
		pdf.Number(b.HCornerRadius),
		pdf.Number(b.VCornerRadius),
		pdf.Number(b.Width),
	}

	if b.DashArray != nil {
		if err := pdf.CheckVersion(rm.Out(), "border dash array", pdf.V1_1); err != nil {
			return nil, zero, err
		}
		dashArray := make(pdf.Array, len(b.DashArray))
		for i, v := range b.DashArray {
			if v < 0 {
				return nil, zero, fmt.Errorf("invalid dash value %f in border dash array", v)
			}
			dashArray[i] = pdf.Number(v)
		}
		borderArray = append(borderArray, dashArray)
	}

	if b.SingleUse {
		return borderArray, zero, nil
	}
	ref := rm.Alloc()
	err := rm.Out().Put(ref, borderArray)
	if err != nil {
		return nil, zero, err
	}
	return ref, zero, nil
}

func (b *Border) isPDFDefault() bool {
	return b != nil &&
		b.HCornerRadius == 0 &&
		b.VCornerRadius == 0 &&
		b.Width == 1 &&
		b.DashArray == nil
}
