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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

// Type2 represents a type 2 (axial) shading.
//
// This type implements the [seehuhn.de/go/pdf/graphics.Shading] interface.
type Type2 struct {
	// ColorSpace defines the color space for shading color values.
	ColorSpace color.Space

	// X0, Y0, X1, Y1 specify the starting and ending coordinates of the axis,
	// expressed in the shading's target coordinate space.
	X0, Y0, X1, Y1 float64

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

// Embed implements the [Shading] interface.
func (s *Type2) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	if s.ColorSpace == nil {
		return nil, zero, errors.New("missing ColorSpace")
	} else if s.ColorSpace.Family() == color.FamilyPattern || s.ColorSpace.Family() == color.FamilyIndexed {
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

	// validate that starting and ending coordinates are not coincident
	if s.X0 == s.X1 && s.Y0 == s.Y1 {
		return nil, zero, errors.New("starting and ending coordinates must not be coincident")
	}

	if s.F == nil {
		return nil, zero, errors.New("missing function")
	}
	if m, _ := s.F.Shape(); m != 1 {
		return nil, zero, fmt.Errorf("function must have 1 input, not %d", m)
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
		"ShadingType": pdf.Integer(2),
		"ColorSpace":  csE,
		"Coords": pdf.Array{
			pdf.Number(s.X0), pdf.Number(s.Y0),
			pdf.Number(s.X1), pdf.Number(s.Y1),
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
