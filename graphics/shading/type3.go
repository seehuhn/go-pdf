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
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

// Type3 represents a type 3 (radial) shading.
//
// This type implements the [seehuhn.de/go/pdf/graphics.Shading] interface.
type Type3 struct {
	// ColorSpace defines the color space for shading color values.
	ColorSpace color.Space

	// X1, Y1, R1 specify the center and radius of the starting circle.
	X1, Y1, R1 float64

	// X2, Y2, R2 specify the center and radius of the ending circle.
	X2, Y2, R2 float64

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
func extractType3(r pdf.Getter, d pdf.Dict, wasReference bool) (*Type3, error) {
	s := &Type3{}

	// Read required ColorSpace
	csObj, ok := d["ColorSpace"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /ColorSpace entry"),
		}
	}
	cs, err := color.ExtractSpace(r, csObj)
	if err != nil {
		return nil, fmt.Errorf("failed to read ColorSpace: %w", err)
	}
	s.ColorSpace = cs

	// Read required Coords
	coordsObj, ok := d["Coords"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /Coords entry"),
		}
	}
	coords, err := pdf.GetFloatArray(r, coordsObj)
	if err != nil {
		return nil, fmt.Errorf("failed to read Coords: %w", err)
	}
	if len(coords) != 6 {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("Coords must have 6 elements, got %d", len(coords)),
		}
	}
	s.X1, s.Y1, s.R1, s.X2, s.Y2, s.R2 = coords[0], coords[1], coords[2], coords[3], coords[4], coords[5]

	// Read required Function
	fnObj, ok := d["Function"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /Function entry"),
		}
	}
	fn, err := function.Extract(r, fnObj)
	if err != nil {
		return nil, fmt.Errorf("failed to read Function: %w", err)
	}
	s.F = fn

	// Read optional Domain (renamed to TMin/TMax for Type3)
	if domainObj, ok := d["Domain"]; ok {
		domain, err := pdf.GetFloatArray(r, domainObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read Domain: %w", err)
		}
		if len(domain) >= 2 {
			s.TMin, s.TMax = domain[0], domain[1]
		}
	} else {
		s.TMin, s.TMax = 0.0, 1.0
	}

	// Read optional Extend
	if extendObj, ok := d["Extend"]; ok {
		extendArray, err := pdf.GetArray(r, extendObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read Extend: %w", err)
		}
		if len(extendArray) >= 1 {
			extendStart, err := pdf.GetBoolean(r, extendArray[0])
			if err != nil {
				return nil, fmt.Errorf("failed to read Extend[0]: %w", err)
			}
			s.ExtendStart = bool(extendStart)
		}
		if len(extendArray) >= 2 {
			extendEnd, err := pdf.GetBoolean(r, extendArray[1])
			if err != nil {
				return nil, fmt.Errorf("failed to read Extend[1]: %w", err)
			}
			s.ExtendEnd = bool(extendEnd)
		}
	}

	// Read optional Background
	if bgObj, ok := d["Background"]; ok {
		bg, err := pdf.GetFloatArray(r, bgObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read Background: %w", err)
		}
		s.Background = bg
	}

	// Read optional BBox
	if bboxObj, ok := d["BBox"]; ok {
		bbox, err := pdf.GetRectangle(r, bboxObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read BBox: %w", err)
		}
		s.BBox = bbox
	}

	// Read optional AntiAlias
	if aaObj, ok := d["AntiAlias"]; ok {
		aa, err := pdf.GetBoolean(r, aaObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read AntiAlias: %w", err)
		}
		s.AntiAlias = bool(aa)
	}

	// Set SingleUse based on whether the original object was a reference
	// True for direct dictionaries, false for references
	s.SingleUse = !wasReference

	return s, nil
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
		ref := rm.Out.Alloc()
		err := rm.Out.Put(ref, dict)
		if err != nil {
			return nil, zero, err
		}
		data = ref
	}

	return data, zero, nil
}
