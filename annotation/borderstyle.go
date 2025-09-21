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

import "seehuhn.de/go/pdf"

// PDF 2.0 sections: 12.5.4

type BorderStyle struct {
	// Width is the border width in points.
	// If 0, no border is drawn.
	Width float64

	// Style is the border style.
	//  - "S" (Solid) is the default.
	//  - "D" (Dashed) specifies a dashed line.
	//  - "B" (Beveled) specifies a beveled line.
	//  - "I" (Inset) specifies an inset line.
	//  - "U" (Underline) specifies an underline.
	Style pdf.Name

	// DashArray (optional) defines a pattern of dashes and gaps for drawing
	// the border when Style is "D".
	DashArray []float64

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

var _ pdf.Embedder[pdf.Unused] = (*BorderStyle)(nil)

func ExtractBorderStyle(x *pdf.Extractor, obj pdf.Object) (*BorderStyle, error) {
	dict, err := pdf.GetDictTyped(x.R, obj, "Border")
	if dict == nil {
		return nil, err
	}

	style := &BorderStyle{}

	style.Width = 1 // default width
	if w, ok := dict["W"]; ok {
		if w, err := pdf.Optional(pdf.GetNumber(x.R, w)); err != nil {
			return nil, err
		} else if w >= 0 {
			style.Width = float64(w)
		}
	}

	if s, err := pdf.Optional(pdf.GetName(x.R, dict["S"])); err != nil {
		return nil, err
	} else if s != "" {
		style.Style = s
	} else {
		style.Style = "S" // default to solid line
	}

	if style.Style == "D" {
		a, err := pdf.Optional(pdf.GetFloatArray(x.R, dict["D"]))
		if err != nil {
			return nil, err
		}
		for _, ai := range a {
			if ai < 0 {
				a = nil
				break
			}
		}
		if len(a) > 0 {
			style.DashArray = a
		} else {
			style.DashArray = borderStyleDefaultDash
		}
	}

	return style, nil
}

var borderStyleDefaultDash = []float64{3}

func (b *BorderStyle) Embed(rm *pdf.EmbedHelper) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	d := pdf.Dict{}
	if rm.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		d["Type"] = pdf.Name("Border")
	}

	if b.Width < 0 {
		return nil, zero, pdf.Error("negative border width")
	}
	if b.Width != 1 {
		d["W"] = pdf.Number(b.Width)
	}

	if b.Style != "S" && b.Style != "" {
		d["S"] = b.Style
	}

	if b.Style == "D" {
		if len(b.DashArray) == 0 {
			return nil, zero, pdf.Error("missing dash array")
		}
		defaultDash := len(b.DashArray) == 1 && b.DashArray[0] == 3
		if b.DashArray != nil && !defaultDash {
			a := make(pdf.Array, len(b.DashArray))
			for i, d := range b.DashArray {
				if d < 0 {
					return nil, zero, pdf.Error("negative dash value")
				}
				a[i] = pdf.Number(d)
			}
			d["D"] = a
		}
	} else if b.DashArray != nil {
		return nil, zero, pdf.Error("unexpected dash array")
	}

	if b.SingleUse {
		return d, zero, nil
	}
	ref := rm.Alloc()
	err := rm.Out().Put(ref, d)
	if err != nil {
		return nil, zero, err
	}
	return ref, zero, nil
}
