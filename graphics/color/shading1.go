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

package color

import (
	"errors"
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
)

// ShadingType1 represents a type 1 (function-based) shading.
type ShadingType1 struct {
	ColorSpace Space

	// F is either 2->n function or an array of n 2->1 functions, where n is
	// the number of colour components of the ColorSpace.
	F function.Func

	// Domain (optional)
	Domain []float64

	Matrix     []float64
	Background []float64
	BBox       *pdf.Rectangle
	AntiAlias  bool
}

// Embed implements the [Shading] interface.
func (s *ShadingType1) Embed(w pdf.Putter, singleUse bool, defName pdf.Name) (*EmbeddedShading, error) {
	if s.ColorSpace == nil {
		return nil, errors.New("missing ColorSpace")
	} else if isPattern(s.ColorSpace) {
		return nil, errors.New("invalid ColorSpace")
	}
	if have := len(s.Background); have > 0 {
		want := len(s.ColorSpace.defaultColor().values())
		if have != want {
			err := fmt.Errorf("wrong number of background values: expected %d, got %d",
				want, have)
			return nil, err
		}
	}
	switch F := s.F.(type) {
	case nil:
		return nil, errors.New("missing Function")
	case pdf.Array:
		if len(F) != len(s.ColorSpace.defaultColor().values()) {
			return nil, errors.New("invalid Function")
		}
	case pdf.Dict, pdf.Reference:
		// pass
	default:
		return nil, fmt.Errorf("invalid Function: %T", s.F)
	}

	if len(s.Domain) > 0 && (len(s.Domain) != 4 || s.Domain[0] > s.Domain[1] || s.Domain[2] > s.Domain[3]) {
		return nil, fmt.Errorf("invalid Domain: %v", s.Domain)
	}
	if len(s.Matrix) > 0 && len(s.Matrix) != 6 {
		return nil, errors.New("invalid Matrix")
	}

	dict := pdf.Dict{
		"ShadingType": pdf.Integer(1),
		"ColorSpace":  s.ColorSpace.PDFObject(),
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
	if singleUse {
		data = dict
	} else {
		ref := w.Alloc()
		err := w.Put(ref, dict)
		if err != nil {
			return nil, err
		}
		data = ref
	}

	return &EmbeddedShading{pdf.Res{DefName: defName, Data: data}}, nil
}
