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
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

// PDF 2.0 sections: 8.7.4.3 8.7.4.5.2

// Type1 represents a type 1 (function-based) shading.
//
// This type implements the [seehuhn.de/go/pdf/graphics.Shading] interface.
type Type1 struct {
	// ColorSpace defines the color space for shading color values.
	ColorSpace color.Space

	// F is either a 2->n function or an array of n 2->1 functions, where n is
	// the number of color components of the ColorSpace.
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

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

var _ graphics.Shading = (*Type1)(nil)

// ShadingType implements the [Shading] interface.
func (s *Type1) ShadingType() int {
	return 1
}

// Equal implements the [Shading] interface.
func (s *Type1) Equal(other graphics.Shading) bool {
	if s == nil || other == nil {
		return s == nil && other == nil
	}
	o, ok := other.(*Type1)
	if !ok {
		return false
	}
	return color.SpacesEqual(s.ColorSpace, o.ColorSpace) &&
		function.Equal(s.F, o.F) &&
		slices.Equal(s.Domain, o.Domain) &&
		slices.Equal(s.Matrix, o.Matrix) &&
		slices.Equal(s.Background, o.Background) &&
		s.BBox.Equal(o.BBox) &&
		s.AntiAlias == o.AntiAlias
}

// extractType1 reads a Type 1 (function-based) shading from a PDF dictionary.
func extractType1(x *pdf.Extractor, d pdf.Dict, singleUse bool) (*Type1, error) {
	s := &Type1{}

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

	// validate function has correct number of inputs
	m, n := fn.Shape()
	if m != 2 {
		return nil, pdf.Errorf("function must have 2 inputs, not %d", m)
	}

	// validate function outputs match color space channels
	if n != cs.Channels() {
		return nil, pdf.Errorf("function outputs (%d) must match color space channels (%d)", n, cs.Channels())
	}

	// validate function domain is well-formed
	functionDomain := fn.GetDomain()
	if len(functionDomain) != 4 {
		return nil, pdf.Errorf("function domain must have 4 values, not %d", len(functionDomain))
	}
	if functionDomain[0] > functionDomain[1] || functionDomain[2] > functionDomain[3] {
		return nil, pdf.Errorf("function domain %v has invalid ranges", functionDomain)
	}

	s.F = fn

	// Read optional Domain
	if domainObj, ok := d["Domain"]; ok {
		if domain, err := pdf.Optional(pdf.GetFloatArray(x.R, domainObj)); err != nil {
			return nil, err
		} else if len(domain) == 4 && domain[0] <= domain[1] && domain[2] <= domain[3] {
			s.Domain = domain
		}
		// Invalid domain values are ignored, using zero value
	}

	// validate function domain contains shading domain
	shadingDomain := []float64{0, 1, 0, 1} // default
	if len(s.Domain) == 4 {
		shadingDomain = s.Domain
	}
	if !domainContains(functionDomain, shadingDomain) {
		return nil, pdf.Errorf("function domain %v must contain shading domain %v", functionDomain, shadingDomain)
	}

	// Read optional Matrix
	if matrixObj, ok := d["Matrix"]; ok {
		if matrix, err := pdf.Optional(pdf.GetFloatArray(x.R, matrixObj)); err != nil {
			return nil, err
		} else if len(matrix) == 6 {
			s.Matrix = matrix
		}
		// Invalid matrix values are ignored, using zero value
	}

	// Read optional Background
	if bgObj, ok := d["Background"]; ok {
		if bg, err := pdf.Optional(pdf.GetFloatArray(x.R, bgObj)); err != nil {
			return nil, err
		} else if len(bg) > 0 {
			if len(bg) != cs.Channels() {
				return nil, pdf.Errorf("wrong number of background values: expected %d, got %d", cs.Channels(), len(bg))
			}
			s.Background = bg
		}
		// Invalid background values are ignored, using zero value
	}

	// Read optional BBox
	if bboxObj, ok := d["BBox"]; ok {
		if bbox, err := pdf.Optional(pdf.GetRectangle(x.R, bboxObj)); err != nil {
			return nil, err
		} else if bbox != nil {
			s.BBox = bbox
		}
		// Invalid bbox values are ignored, using zero value
	}

	// Read optional AntiAlias
	if aaObj, ok := d["AntiAlias"]; ok {
		if aa, err := pdf.Optional(x.GetBoolean(aaObj)); err != nil {
			return nil, err
		} else {
			s.AntiAlias = bool(aa)
		}
		// Invalid antiAlias values are ignored, using zero value (false)
	}

	s.SingleUse = singleUse

	return s, nil
}

// Embed implements the [Shading] interface.
func (s *Type1) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	// Version check
	if err := pdf.CheckVersion(rm.Out(), "Type 1 shading", pdf.V1_3); err != nil {
		return nil, err
	}

	if s.ColorSpace == nil {
		return nil, errors.New("missing ColorSpace")
	} else if s.ColorSpace.Family() == color.FamilyPattern {
		return nil, errors.New("invalid ColorSpace")
	} else if s.ColorSpace.Family() == color.FamilyIndexed {
		return nil, errors.New("Type 1 shading cannot use Indexed color space")
	}
	if have := len(s.Background); have > 0 {
		want := s.ColorSpace.Channels()
		if have != want {
			err := fmt.Errorf("wrong number of background values: expected %d, got %d",
				want, have)
			return nil, err
		}
	}
	if m, n := s.F.Shape(); m != 2 {
		return nil, fmt.Errorf("function must have 2 inputs, not %d", m)
	} else if n != s.ColorSpace.Channels() {
		return nil, fmt.Errorf("function outputs (%d) must match color space channels (%d)", n, s.ColorSpace.Channels())
	}

	// Validate function domain contains shading domain
	shadingDomain := []float64{0, 1, 0, 1} // default domain
	if len(s.Domain) == 4 {
		shadingDomain = s.Domain
	}
	functionDomain := s.F.GetDomain()
	if !domainContains(functionDomain, shadingDomain) {
		return nil, fmt.Errorf("function domain %v must contain shading domain %v", functionDomain, shadingDomain)
	}

	fn, err := rm.Embed(s.F)
	if err != nil {
		return nil, err
	}

	if len(s.Domain) > 0 && (len(s.Domain) != 4 || s.Domain[0] > s.Domain[1] || s.Domain[2] > s.Domain[3]) {
		return nil, fmt.Errorf("invalid Domain: %v", s.Domain)
	}
	if len(s.Matrix) > 0 && len(s.Matrix) != 6 {
		return nil, errors.New("invalid Matrix")
	}

	csE, err := rm.Embed(s.ColorSpace)
	if err != nil {
		return nil, err
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
