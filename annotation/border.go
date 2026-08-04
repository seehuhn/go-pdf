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

// PDF 2.0 sections: 12.5.2

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

var _ pdf.Embedder = (*Border)(nil)

// PDFDefaultBorder is the default border values within PDF files.
// Using this for [Common.Border] slightly reduces file size.
var PDFDefaultBorder = &Border{Width: 1}

// borderStyled is implemented by the annotation types which can carry a border
// style dictionary, so that a rule reading the style can be written once for
// all of them.  Each type's style is reachable through its exported
// BorderStyle field; this interface exists to reach it from an [Annotation],
// and adding a case to it means implementing the method on the new type.
type borderStyled interface {
	Annotation
	getBorderStyle() *BorderStyle
}

var (
	_ borderStyled = (*Circle)(nil)
	_ borderStyled = (*FreeText)(nil)
	_ borderStyled = (*Ink)(nil)
	_ borderStyled = (*Line)(nil)
	_ borderStyled = (*Link)(nil)
	_ borderStyled = (*Polygon)(nil)
	_ borderStyled = (*PolyLine)(nil)
	_ borderStyled = (*Square)(nil)
	_ borderStyled = (*Widget)(nil)
)

// EffectiveBorderWidth returns the width of the border drawn around the
// annotation, in default user space units.  A width of 0 means no border is
// drawn.
//
// A border style takes precedence over the [Common.Border] array, which is
// ignored whenever one is present.  An annotation with neither has no border:
// reading an annotation whose border is left out supplies the width the
// absent entry stands for, so a border missing at this point is one the file
// asked to be left undrawn.
func EffectiveBorderWidth(a Annotation) float64 {
	if bs, ok := a.(borderStyled); ok {
		if style := bs.getBorderStyle(); style != nil {
			return style.Width
		}
	}
	if border := a.GetCommon().Border; border != nil {
		return border.Width
	}
	return 0
}

// EffectiveBorderStyle returns the style the annotation's border is drawn
// with: "S" (solid), "D" (dashed), "B" (beveled), "I" (inset) or "U"
// (underline).  An annotation which asks for none is drawn solid.
//
// A border style takes precedence over the [Common.Border] array, as it does
// for [EffectiveBorderWidth].  A border array has no style entry of its own,
// so it can only ask for a solid or a dashed border, and it asks for a dashed
// one by carrying a dash pattern.
func EffectiveBorderStyle(a Annotation) pdf.Name {
	if bs, ok := a.(borderStyled); ok {
		if style := bs.getBorderStyle(); style != nil {
			if style.Style == "" {
				return "S"
			}
			return style.Style
		}
	}
	if border := a.GetCommon().Border; border != nil && len(border.DashArray) > 0 {
		return "D"
	}
	return "S"
}

// EffectiveBorderDash returns the pattern of dashes and gaps the annotation's
// border is drawn with, in default user space units, or nil for an unbroken
// border.  The dash phase is always 0.
//
// A border style takes precedence over the [Common.Border] array, as it does
// for [EffectiveBorderWidth].  A style holds a dash pattern only when it asks
// for a dashed border, so no check against [EffectiveBorderStyle] is needed
// here.
func EffectiveBorderDash(a Annotation) []float64 {
	if bs, ok := a.(borderStyled); ok {
		if style := bs.getBorderStyle(); style != nil {
			return style.DashArray
		}
	}
	if border := a.GetCommon().Border; border != nil {
		return border.DashArray
	}
	return nil
}

// ExtractBorder extracts a Border from a PDF array.
// If no border entry exists, returns the PDF default (solid border with width 1).
// If no border is to be drawn, returns nil.
func ExtractBorder(c pdf.Cursor, obj pdf.Object, isDirect bool) (*Border, error) {

	border, err := pdf.Optional(c.Array(obj))
	if err != nil {
		return nil, err
	}

	if len(border) < 3 {
		return PDFDefaultBorder, nil
	}

	b := &Border{}

	if h, err := pdf.Optional(c.Number(border[0])); err != nil {
		return nil, err
	} else {
		b.HCornerRadius = h
	}

	if v, err := pdf.Optional(c.Number(border[1])); err != nil {
		return nil, err
	} else {
		b.VCornerRadius = v
	}

	if w, err := pdf.Optional(c.Number(border[2])); err != nil {
		return nil, err
	} else {
		b.Width = w
	}

	if b.Width <= 0 {
		return nil, nil // no border
	}

	if len(border) > 3 {
		if dashArray, err := pdf.Optional(c.FloatArray(border[3])); err != nil {
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

	b.SingleUse = isDirect

	return b, nil
}

func (b *Border) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	// the Go default value is "no border"
	if b == nil {
		return pdf.Array{pdf.Number(0), pdf.Number(0), pdf.Number(0)}, nil
	}

	// if we have the PDF default value, we don't need to store anything
	if b.isPDFDefault() {
		return nil, nil
	}

	if b.Width <= 0 {
		return nil, fmt.Errorf("invalid border width %f", b.Width)
	}
	for _, v := range b.DashArray {
		if v <= 0 {
			return nil, fmt.Errorf("invalid dash value %f", v)
		}
	}

	borderArray := pdf.Array{
		pdf.Number(b.HCornerRadius),
		pdf.Number(b.VCornerRadius),
		pdf.Number(b.Width),
	}

	if b.DashArray != nil {
		if err := pdf.CheckVersion(rm.Out(), "border dash array", pdf.V1_1); err != nil {
			return nil, err
		}
		dashArray := make(pdf.Array, len(b.DashArray))
		for i, v := range b.DashArray {
			if v < 0 {
				return nil, fmt.Errorf("invalid dash value %f in border dash array", v)
			}
			dashArray[i] = pdf.Number(v)
		}
		borderArray = append(borderArray, dashArray)
	}

	if b.SingleUse {
		return borderArray, nil
	}
	ref := rm.Alloc()
	err := rm.Out().Put(ref, borderArray)
	if err != nil {
		return nil, err
	}
	return ref, nil
}

func (b *Border) isPDFDefault() bool {
	return b != nil &&
		b.HCornerRadius == 0 &&
		b.VCornerRadius == 0 &&
		b.Width == 1 &&
		b.DashArray == nil
}
