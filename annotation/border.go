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
}

func (b *Border) isDefault() bool {
	return b.HCornerRadius == 0 &&
		b.VCornerRadius == 0 &&
		b.Width == 1 &&
		b.DashArray == nil
}

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

func ExtractBorderStyle(r pdf.Getter, obj pdf.Object) (*BorderStyle, error) {
	dict, err := pdf.GetDictTyped(r, obj, "Border")
	if dict == nil {
		return nil, err
	}

	style := &BorderStyle{}

	if w, ok := dict["W"]; ok {
		x, _ := pdf.GetNumber(r, w)
		style.Width = float64(x)
	} else {
		style.Width = 1 // default width
	}

	style.Style, _ = pdf.GetName(r, dict["S"])
	if style.Style == "" {
		style.Style = "S" // default to solid line
	}

	a, _ := pdf.GetArray(r, dict["D"])
	if len(a) > 0 {
		style.DashArray = make([]float64, len(a))
		for i, d := range a {
			if num, err := pdf.GetNumber(r, d); err == nil {
				style.DashArray[i] = float64(num)
			}
		}
	} else {
		style.DashArray = []float64{3} // default dash pattern
	}

	return style, nil
}

func (b *BorderStyle) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	d := pdf.Dict{}
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		d["Type"] = pdf.Name("Border")
	}

	if b.Width != 1 {
		d["W"] = pdf.Number(b.Width)
	}
	if b.Style != "S" && b.Style != "" {
		d["S"] = b.Style
	}

	defaultDash := len(b.DashArray) == 1 && b.DashArray[0] == 3
	if b.DashArray != nil && !defaultDash {
		a := make(pdf.Array, len(b.DashArray))
		for i, d := range b.DashArray {
			a[i] = pdf.Number(d)
		}
		d["D"] = a
	}

	if b.SingleUse {
		return d, zero, nil
	}
	ref := rm.Out.Alloc()
	err := rm.Out.Put(ref, d)
	if err != nil {
		return nil, zero, err
	}
	return ref, zero, nil
}

// BorderEffect represents a border effect dictionary that specifies
// an effect applied to an annotation's border.
type BorderEffect struct {
	// Style is the border effect style.
	//  - "S" (Solid): no effect.
	//  - "C" (Cloudy): border effect.
	//
	// When writing annotations, and empty Style value can be used
	// as a shorthand for "S".
	Style pdf.Name

	// Intensity (meaningful only when Style is "C") specifies
	// the intensity of the cloudy border effect.
	// Valid range is 0.0 to 2.0.
	Intensity float64

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

var _ pdf.Embedder[pdf.Unused] = (*BorderEffect)(nil)

func ExtractBorderEffect(r pdf.Getter, obj pdf.Object) (*BorderEffect, error) {
	dict, err := pdf.GetDict(r, obj)
	if err != nil {
		return nil, err
	}
	if dict == nil {
		return nil, pdf.Error("missing border effect dictionary")
	}

	effect := &BorderEffect{}

	if style, err := pdf.Optional(pdf.GetName(r, dict["S"])); err != nil {
		return nil, err
	} else if style != "" {
		effect.Style = style
	} else { // default to solid
		effect.Style = "S"
	}

	if intensity, err := pdf.Optional(pdf.GetNumber(r, dict["I"])); err != nil {
		return nil, err
	} else if effect.Style == "C" {
		effect.Intensity = float64(intensity)
	}

	_, isIndirect := obj.(pdf.Reference)
	effect.SingleUse = !isIndirect

	return effect, nil
}

func (be *BorderEffect) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "border effect dictionary", pdf.V1_5); err != nil {
		return nil, zero, err
	}

	d := pdf.Dict{}

	if be.Style != "S" && be.Style != "" {
		d["S"] = be.Style
	}

	if be.Style == "C" && be.Intensity != 0 {
		d["I"] = pdf.Number(be.Intensity)
	}

	if be.SingleUse {
		return d, zero, nil
	}
	ref := rm.Out.Alloc()
	err := rm.Out.Put(ref, d)
	if err != nil {
		return nil, zero, err
	}
	return ref, zero, nil
}
