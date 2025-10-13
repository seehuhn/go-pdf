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

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

// PDF 2.0 sections: 8.7.4.3 8.7.4.5.4

// Type3 represents a type 3 (radial) shading.
//
// This type implements the [seehuhn.de/go/pdf/graphics.Shading] interface.
type Type3 struct {
	// ColorSpace defines the color space for shading color values.
	ColorSpace color.Space

	// Center1, R1 specify the center and radius of the starting circle.
	Center1 vec.Vec2
	R1      float64

	// Center2, R2 specify the center and radius of the ending circle.
	Center2 vec.Vec2
	R2      float64

	// F is either 1->n function or an array of n 1->1 functions, where n is
	// the number of color components of the ColorSpace.
	F pdf.Function

	// TMin, TMax specify the limiting values of the parametric variable t.
	// Default: [0, 1].
	TMin, TMax float64

	// ExtendStart specifies whether to extend the shading beyond the starting circle.
	ExtendStart bool

	// ExtendEnd specifies whether to extend the shading beyond the ending circle.
	ExtendEnd bool

	// Background (optional) specifies the color for areas outside the
	// shading's bounds, when used in a shading pattern.
	Background []float64

	// BBox (optional) defines the shading's bounding box as a clipping boundary.
	BBox *pdf.Rectangle

	// AntiAlias controls whether to filter the shading function to prevent aliasing.
	AntiAlias bool

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

var _ graphics.Shading = (*Type3)(nil)

// ShadingType implements the [Shading] interface.
func (s *Type3) ShadingType() int {
	return 3
}

// extractType3 reads a Type 3 (radial) shading from a PDF dictionary.
func extractType3(x *pdf.Extractor, d pdf.Dict, wasReference bool) (*Type3, error) {
	s := &Type3{}

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
	if len(coords) != 6 {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("Coords must have 6 elements, got %d", len(coords)),
		}
	}
	s.Center1 = vec.Vec2{X: coords[0], Y: coords[1]}
	s.R1 = coords[2]
	s.Center2 = vec.Vec2{X: coords[3], Y: coords[4]}
	s.R2 = coords[5]

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

	// Read optional Domain (renamed to TMin/TMax for Type3)
	if domainObj, ok := d["Domain"]; ok {
		if domain, err := pdf.Optional(pdf.GetFloatArray(x.R, domainObj)); err != nil {
			return nil, err
		} else if len(domain) == 2 {
			s.TMin, s.TMax = domain[0], domain[1]
		} else {
			s.TMin, s.TMax = 0.0, 1.0
		}
	} else {
		s.TMin, s.TMax = 0.0, 1.0
	}

	// Read optional Extend
	if extendObj, ok := d["Extend"]; ok {
		if extendArray, err := pdf.Optional(x.GetArray(extendObj)); err != nil {
			return nil, err
		} else {
			if len(extendArray) >= 1 {
				if extendStart, err := pdf.Optional(x.GetBoolean(extendArray[0])); err != nil {
					return nil, err
				} else {
					s.ExtendStart = bool(extendStart)
				}
			}
			if len(extendArray) >= 2 {
				if extendEnd, err := pdf.Optional(x.GetBoolean(extendArray[1])); err != nil {
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
		} else if len(bg) == s.ColorSpace.Channels() {
			s.Background = bg // Only store if valid length
		}
		// Invalid lengths silently ignored for permissive reading
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
		if aa, err := pdf.Optional(x.GetBoolean(aaObj)); err != nil {
			return nil, err
		} else {
			s.AntiAlias = bool(aa)
		}
	}

	// obj is indirect if passed as a reference or accessed through one
	s.SingleUse = !wasReference && !x.IsIndirect

	return s, nil
}

// Embed implements the [Shading] interface.
func (s *Type3) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "Type 3 shadings", pdf.V1_3); err != nil {
		return nil, err
	}
	if s.ColorSpace == nil {
		return nil, errors.New("missing ColorSpace")
	} else if s.ColorSpace.Family() == color.FamilyPattern {
		return nil, errors.New("Pattern color space not allowed")
	} else if s.ColorSpace.Family() == color.FamilyIndexed {
		return nil, errors.New("Indexed color space not allowed")
	}
	if have := len(s.Background); have > 0 {
		want := s.ColorSpace.Channels()
		if have != want {
			err := fmt.Errorf("wrong number of background values: expected %d, got %d",
				want, have)
			return nil, err
		}
	}

	if s.R1 < 0 {
		return nil, fmt.Errorf("invalid radius: %f", s.R1)
	}
	if s.R2 < 0 {
		return nil, fmt.Errorf("invalid radius: %f", s.R2)
	}
	if s.F == nil {
		return nil, errors.New("missing function")
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
		"ShadingType": pdf.Integer(3),
		"ColorSpace":  csE,
		"Coords": pdf.Array{
			pdf.Number(s.Center1.X), pdf.Number(s.Center1.Y), pdf.Number(s.R1),
			pdf.Number(s.Center2.X), pdf.Number(s.Center2.Y), pdf.Number(s.R2),
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
