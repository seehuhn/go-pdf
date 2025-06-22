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
	"seehuhn.de/go/pdf/graphics/color"
)

// Type1 represents a type 1 (function-based) shading.
//
// This type implements the [seehuhn.de/go/pdf/graphics.Shading] interface.
type Type1 struct {
	// ColorSpace defines the color space for shading color values.
	ColorSpace color.Space

	// F is either 2->n function or an array of n 2->1 functions, where n is
	// the number of colour components of the ColorSpace.
	F pdf.Function

	// Domain (optional) specifies the rectangular coordinate domain [xmin xmax
	// ymin ymax]. The default is [0 1 0 1].
	Domain []float64

	// Matrix (optional) transforms domain coordinates to target coordinate
	// space. Default: identity matrix [1 0 0 1 0 0].
	Matrix []float64

	// Background (optional) specifies the color for areas outside the
	// transformed domain, when used in a shading pattern. The default is to
	// leave points outside the transformed domain unpainted.
	Background []float64

	// BBox (optional) defines the shading's bounding box as a clipping
	// boundary.
	BBox *pdf.Rectangle

	// AntiAlias controls whether to filter the shading function to prevent
	// aliasing. Default: false.
	AntiAlias bool

	// SingleUse determines if shading is returned as dictionary (true) or
	// reference (false).
	SingleUse bool
}

// ShadingType implements the [Shading] interface.
func (s *Type1) ShadingType() int {
	return 1
}

// Embed implements the [Shading] interface.
func (s *Type1) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
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

	fn, _, err := pdf.ResourceManagerEmbed(rm, s.F)
	if err != nil {
		return nil, zero, err
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
		"Function":    fn,
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
