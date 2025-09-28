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

package shading

import (
	"errors"
	"fmt"

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

// PDF 2.0 sections: 8.7.4.3 8.7.4.5.3

// Type2 represents a type 2 (axial) shading.
//
// This type implements the [seehuhn.de/go/pdf/graphics.Shading] interface.
type Type2 struct {
	// ColorSpace defines the color space for shading color values.
	ColorSpace color.Space

	// P0, P1 specify the starting and ending coordinates of the axis,
	// expressed in the shading's target coordinate space.
	P0, P1 vec.Vec2

	// F is a 1->n function where n is the number of color components of the
	// ColorSpace. The function is called with values of the parametric
	// variable t in the domain defined by TMin and TMax.
	//
	// TODO: Add support for array of n 1->1 functions as alternative to
	// single 1->n function.
	F pdf.Function

	// TMin, TMax specify the limiting values of the parametric variable t.
	// The variable is considered to vary linearly between these two values
	// as the color gradient varies between the starting and ending points
	// of the axis. Default: [0.0, 1.0].
	TMin, TMax float64

	// ExtendStart specifies whether to extend the shading beyond the starting
	// point of the axis. Default: false.
	ExtendStart bool

	// ExtendEnd specifies whether to extend the shading beyond the ending
	// point of the axis. Default: false.
	ExtendEnd bool

	// Background (optional) specifies the color for areas outside the
	// shading's bounds, when used in a shading pattern. The default is to
	// leave such points unpainted.
	Background []float64

	// BBox (optional) defines the shading's bounding box as a clipping
	// boundary.
	BBox *pdf.Rectangle

	// AntiAlias controls whether to filter the shading function to prevent
	// aliasing.
	AntiAlias bool

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

var _ graphics.Shading = (*Type2)(nil)

// ShadingType implements the [Shading] interface.
func (s *Type2) ShadingType() int {
	return 2
}

// extractType2 reads a Type 2 (axial) shading from a PDF dictionary.
func extractType2(x *pdf.Extractor, d pdf.Dict, isIndirect bool) (*Type2, error) {
	s := &Type2{}

	// Read required ColorSpace
	csObj, ok := d["ColorSpace"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /ColorSpace entry"),
		}
	}
	cs, err := color.ExtractSpace(x, csObj)
	if err != nil {
		return nil, err
	}
	s.ColorSpace = cs

	// Read required Coords
	coordsObj, ok := d["Coords"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /Coords entry"),
		}
	}
	coords, err := pdf.GetFloatArray(x.R, coordsObj)
	if err != nil {
		return nil, err
	}
	if len(coords) != 4 {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("Coords must have 4 elements, got %d", len(coords)),
		}
	}
	s.P0 = vec.Vec2{X: coords[0], Y: coords[1]}
	s.P1 = vec.Vec2{X: coords[2], Y: coords[3]}

	// Read required Function
	fnObj, ok := d["Function"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /Function entry"),
		}
	}
	fn, err := pdf.ExtractorGet(x, fnObj, function.Extract)
	if err != nil {
		return nil, err
	}
	s.F = fn

	// Read optional Domain (renamed to TMin/TMax for Type2)
	if domainObj, ok := d["Domain"]; ok {
		if domain, err := pdf.Optional(pdf.GetFloatArray(x.R, domainObj)); err != nil {
			return nil, err
		} else if len(domain) >= 2 {
			s.TMin, s.TMax = domain[0], domain[1]
		} else {
			s.TMin, s.TMax = 0.0, 1.0
		}
	} else {
		s.TMin, s.TMax = 0.0, 1.0
	}

	// Read optional Extend
	if extendObj, ok := d["Extend"]; ok {
		if extendArray, err := pdf.Optional(pdf.GetArray(x.R, extendObj)); err != nil {
			return nil, err
		} else {
			if len(extendArray) >= 1 {
				if extendStart, err := pdf.Optional(pdf.GetBoolean(x.R, extendArray[0])); err != nil {
					return nil, err
				} else {
					s.ExtendStart = bool(extendStart)
				}
			}
			if len(extendArray) >= 2 {
				if extendEnd, err := pdf.Optional(pdf.GetBoolean(x.R, extendArray[1])); err != nil {
					return nil, err
				} else {
					s.ExtendEnd = bool(extendEnd)
				}
			}
		}
	}

	// Read optional Background
	if bgObj, ok := d["Background"]; ok {
		if bg, err := pdf.Optional(pdf.GetFloatArray(x.R, bgObj)); err != nil {
			return nil, err
		} else {
			s.Background = bg
		}
	}

	// Read optional BBox
	if bboxObj, ok := d["BBox"]; ok {
		if bbox, err := pdf.Optional(pdf.GetRectangle(x.R, bboxObj)); err != nil {
			return nil, err
		} else {
			s.BBox = bbox
		}
	}

	// Read optional AntiAlias
	if aaObj, ok := d["AntiAlias"]; ok {
		if aa, err := pdf.Optional(pdf.GetBoolean(x.R, aaObj)); err != nil {
			return nil, err
		} else {
			s.AntiAlias = bool(aa)
		}
	}

	// Set SingleUse based on whether the original object was a reference,
	// true for direct dictionaries, false for references.
	s.SingleUse = !isIndirect

	return s, nil
}

// Embed implements the [Shading] interface.
func (s *Type2) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {

	// Version check
	if err := pdf.CheckVersion(rm.Out(), "Type 2 shading", pdf.V1_3); err != nil {
		return nil, err
	}

	if s.ColorSpace == nil {
		return nil, errors.New("missing ColorSpace")
	} else if s.ColorSpace.Family() == color.FamilyPattern || s.ColorSpace.Family() == color.FamilyIndexed {
		return nil, errors.New("invalid ColorSpace")
	}
	if have := len(s.Background); have > 0 {
		want := s.ColorSpace.Channels()
		if have != want {
			err := fmt.Errorf("wrong number of background values: expected %d, got %d",
				want, have)
			return nil, err
		}
	}

	// validate that starting and ending coordinates are not coincident
	if s.P0 == s.P1 {
		return nil, errors.New("starting and ending coordinates must not be coincident")
	}

	// validate domain relationship
	if s.TMin > s.TMax {
		return nil, fmt.Errorf("TMin (%g) must be less than or equal to TMax (%g)", s.TMin, s.TMax)
	}

	if s.F == nil {
		return nil, errors.New("missing function")
	}
	if m, n := s.F.Shape(); m != 1 {
		return nil, fmt.Errorf("function must have 1 input, not %d", m)
	} else if n != s.ColorSpace.Channels() {
		return nil, fmt.Errorf("function outputs (%d) must match color space channels (%d)", n, s.ColorSpace.Channels())
	}

	// Validate function domain contains shading domain
	shadingDomain := []float64{s.TMin, s.TMax}
	functionDomain := s.F.GetDomain()
	if !domainContains(functionDomain, shadingDomain) {
		return nil, fmt.Errorf("function domain %v must contain shading domain %v", functionDomain, shadingDomain)
	}

	fn, err := rm.Embed(s.F)
	if err != nil {
		return nil, err
	}

	csE, err := rm.Embed(s.ColorSpace)
	if err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"ShadingType": pdf.Integer(2),
		"ColorSpace":  csE,
		"Coords": pdf.Array{
			pdf.Number(s.P0.X), pdf.Number(s.P0.Y),
			pdf.Number(s.P1.X), pdf.Number(s.P1.Y),
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
	if s.TMin != 0 || s.TMax != 1 {
		dict["Domain"] = pdf.Array{pdf.Number(s.TMin), pdf.Number(s.TMax)}
	}
	if s.ExtendStart || s.ExtendEnd {
		dict["Extend"] = pdf.Array{pdf.Boolean(s.ExtendStart), pdf.Boolean(s.ExtendEnd)}
	}

	var data pdf.Native
	if s.SingleUse {
		data = dict
	} else {
		ref := rm.Alloc()
		err := rm.Out().Put(ref, dict)
		if err != nil {
			return nil, err
		}
		data = ref
	}

	return data, nil
}
