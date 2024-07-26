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

// Type1 represents a type 1 (function-based) shading.
//
// This type implements the [seehuhn.de/go/pdf/graphics.Shading] interface.
type Type1 struct {
	ColorSpace color.Space

	// F is either 2->n function or an array of n 2->1 functions, where n is
	// the number of colour components of the ColorSpace.
	F function.Func

	// Domain (optional)
	Domain []float64

	Matrix     []float64
	Background []float64
	BBox       *pdf.Rectangle
	AntiAlias  bool

	SingleUse bool
}

// ShadingType implements the [Shading] interface.
func (s *Type1) ShadingType() int {
	return 1
}

// Embed implements the [Shading] interface.
func (s *Type1) Embed(rm *pdf.ResourceManager) (pdf.Object, pdf.Unused, error) {
	var zero pdf.Unused
	if s.ColorSpace == nil {
		return nil, zero, errors.New("missing ColorSpace")
	} else if s.ColorSpace.ColorSpaceFamily() == color.FamilyPattern {
		return nil, zero, errors.New("invalid ColorSpace")
	}
	if have := len(s.Background); have > 0 {
		want := color.NumValues(s.ColorSpace)
		if have != want {
			err := fmt.Errorf("wrong number of background values: expected %d, got %d",
				want, have)
			return nil, zero, err
		}
	}
	switch F := s.F.(type) {
	case nil:
		return nil, zero, errors.New("missing Function")
	case pdf.Array:
		if len(F) != color.NumValues(s.ColorSpace) {
			return nil, zero, errors.New("invalid Function")
		}
	case pdf.Dict, pdf.Reference:
		// pass
	default:
		return nil, zero, fmt.Errorf("invalid Function: %T", s.F)
	}

	if len(s.Domain) > 0 && (len(s.Domain) != 4 || s.Domain[0] > s.Domain[1] || s.Domain[2] > s.Domain[3]) {
		return nil, zero, fmt.Errorf("invalid Domain: %v", s.Domain)
	}
	if len(s.Matrix) > 0 && len(s.Matrix) != 6 {
		return nil, zero, errors.New("invalid Matrix")
	}

	csE, _, err := pdf.ResourceManagerEmbed(rm, s.ColorSpace)
	if err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{
		"ShadingType": pdf.Integer(1),
		"ColorSpace":  csE,
		"Function":    s.F,
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
	if len(s.Domain) > 0 && !isValues(s.Domain, 0, 1, 0, 1) {
		dict["Domain"] = toPDF(s.Domain)
	}
	if len(s.Matrix) > 0 && !isValues(s.Matrix, 1, 0, 0, 1, 0, 0) {
		dict["Matrix"] = toPDF(s.Matrix)
	}

	var data pdf.Object
	if s.SingleUse {
		data = dict
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
