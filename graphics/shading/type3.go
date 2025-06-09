// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package shading

import (
	"errors"
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics/color"
)

// Type3 represents a type 3 (radial) shading.
//
// This type implements the [seehuhn.de/go/pdf/graphics.Shading] interface.
type Type3 struct {
	ColorSpace color.Space
	X1, Y1, R1 float64
	X2, Y2, R2 float64

	// F is either 1->n function or an array of n 1->1 functions, where n is
	// the number of colour components of the ColorSpace.
	F function.Func

	TMin, TMax  float64
	ExtendStart bool
	ExtendEnd   bool
	Background  []float64
	BBox        *pdf.Rectangle
	AntiAlias   bool

	SingleUse bool
}

// ShadingType implements the [Shading] interface.
func (s *Type3) ShadingType() int {
	return 3
}

// Embed implements the [Shading] interface.
func (s *Type3) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	if s.ColorSpace == nil {
		return nil, zero, errors.New("missing ColorSpace")
	} else if s.ColorSpace.Family() == color.FamilyPattern {
		return nil, zero, errors.New("invalid ColorSpace")
	}
	if have := len(s.Background); have > 0 {
		want := s.ColorSpace.Channels()
		if have != want {
			err := fmt.Errorf("wrong number of background values: expected %d, got %d",
				want, have)
			return nil, zero, err
		}
	}

	if s.R1 < 0 {
		return nil, zero, fmt.Errorf("invalid radius: %f", s.R1)
	}
	if s.R2 < 0 {
		return nil, zero, fmt.Errorf("invalid radius: %f", s.R2)
	}
	if s.F == nil {
		return nil, zero, errors.New("missing function")
	}
	fn, _, err := pdf.ResourceManagerEmbed(rm, s.F)
	if err != nil {
		return nil, zero, err
	}

	csE, _, err := pdf.ResourceManagerEmbed(rm, s.ColorSpace)
	if err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{
		"ShadingType": pdf.Integer(3),
		"ColorSpace":  csE,
		"Coords": pdf.Array{
			pdf.Number(s.X1), pdf.Number(s.Y1), pdf.Number(s.R1),
			pdf.Number(s.X2), pdf.Number(s.Y2), pdf.Number(s.R2),
		},
		"Function": fn,
	}
	if len(s.Background) > 0 {
		dict["Background"] = toPDF(s.Background)
	}
	if s.BBox != nil {
		dict["BBox"] = s.BBox
	}
	if s.AntiAlias {
		dict["AntiAlias"] = pdf.Boolean(true)
	}
	if s.TMin != 0 || (s.TMax != 0 && s.TMax != 1) {
		dict["Domain"] = pdf.Array{pdf.Number(s.TMin), pdf.Number(s.TMax)}
	}
	if s.ExtendStart || s.ExtendEnd {
		dict["Extend"] = pdf.Array{pdf.Boolean(s.ExtendStart), pdf.Boolean(s.ExtendEnd)}
	}

	var data pdf.Native
	if s.SingleUse {
		data = dict.AsPDF(rm.Out.GetOptions())
	} else {
		ref := rm.Out.Alloc()
		err := rm.Out.Put(ref, dict)
		if err != nil {
			return nil, zero, err
		}
		data = ref
	}

	return data, zero, nil
}
