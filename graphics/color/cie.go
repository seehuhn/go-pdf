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
	"math"

	"seehuhn.de/go/pdf"
)

// == CalGray ================================================================

// SpaceCalGray represents a CalGray color space.
type SpaceCalGray struct {
	whitePoint []float64
	blackPoint []float64
	gamma      float64
}

// CalGray returns a new CalGray color space.
//
// WhitePoint is the diffuse white point in CIE 1931 XYZ coordinates.  This
// must be a slice of length 3, with positive entries, and Y=1.
//
// BlackPoint (optional) is the diffuse black point in the CIE 1931 XYZ
// coordinates.  If non-nil, this must be a slice of three non-negative
// numbers.  The default is [0 0 0].
//
// The gamma parameter is a positive number (usually greater than or equal to 1).
//
// DefName is the default resource name to use within content streams.
// This can be left empty to allocate names automatically.
func CalGray(whitePoint, blackPoint []float64, gamma float64) (*SpaceCalGray, error) {
	if !isValidWhitePoint(whitePoint) {
		return nil, errors.New("CalGray: invalid white point")
	}
	if blackPoint == nil {
		blackPoint = []float64{0, 0, 0}
	} else if !isValidBlackPoint(blackPoint) {
		return nil, errors.New("CalGray: invalid black point")
	}
	if gamma <= 0 {
		return nil, fmt.Errorf("CalGray: expected gamma > 0, got %f", gamma)
	}

	return &SpaceCalGray{
		whitePoint: whitePoint,
		blackPoint: blackPoint,
		gamma:      gamma,
	}, nil
}

// New returns a new CalGray color.
func (s *SpaceCalGray) New(gray float64) Color {
	return colorCalGray{Space: s, Value: gray}
}

// ColorSpaceFamily implements the [SpaceEmbedded] interface.
func (s *SpaceCalGray) ColorSpaceFamily() pdf.Name {
	return FamilyCalGray
}

// defaultValues implements the [Space] interface.
func (s *SpaceCalGray) defaultValues() []float64 {
	return []float64{0}
}

// Embed implements the [Space] interface.
func (s *SpaceCalGray) Embed(rm *pdf.ResourceManager) (pdf.Object, pdf.Unused, error) {
	var zero pdf.Unused
	if err := pdf.CheckVersion(rm.Out, "CalGray color space", pdf.V1_1); err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{}
	dict["WhitePoint"] = toPDF(s.whitePoint)
	if !isZero(s.blackPoint) {
		dict["BlackPoint"] = toPDF(s.blackPoint)
	}
	if math.Abs(s.gamma-1) >= ε {
		dict["Gamma"] = pdf.Number(s.gamma)
	}

	return pdf.Array{pdf.Name("CalGray"), dict}, zero, nil
}

func readSpaceCalGray(r pdf.Getter, a pdf.Array) (s *SpaceCalGray, err error) {
	defer func() {
		if err != nil {
			err = pdf.Wrap(err, "CalGray color space")
		}
	}()

	if len(a) != 2 {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("invalid array length"),
		}
	}
	param, err := pdf.GetDict(r, a[1])
	if err != nil {
		return nil, err
	}

	s = &SpaceCalGray{}

	s.whitePoint, _ = getNumbers(r, param["WhitePoint"], 3)
	if !isValidWhitePoint(s.whitePoint) {
		s.whitePoint = WhitePointD65
	}

	s.blackPoint, _ = getNumbers(r, param["BlackPoint"], 3)
	if len(s.blackPoint) != 3 {
		s.blackPoint = []float64{0, 0, 0}
	}

	gamma, _ := pdf.GetNumber(r, param["Gamma"])
	if err != nil || gamma <= 0 {
		gamma = 1
	}
	s.gamma = float64(gamma)

	return s, nil
}

type colorCalGray struct {
	Space *SpaceCalGray
	Value float64
}

// ColorSpace implements the [Color] interface.
func (c colorCalGray) ColorSpace() Space {
	return c.Space
}

// values implements the [Color] interface.
func (c colorCalGray) values() []float64 {
	return []float64{c.Value}
}

// == CalRGB =================================================================

// SpaceCalRGB represents a CalRGB color space.
type SpaceCalRGB struct {
	whitePoint []float64
	blackPoint []float64
	gamma      []float64
	matrix     []float64
}

// CalRGB returns a new CalRGB color space.
//
// WhitePoint is the diffuse white point in CIE 1931 XYZ coordinates.  This
// must be a slice of length 3, with positive entries, and Y=1.
//
// BlackPoint (optional) is the diffuse black point in the CIE 1931 XYZ
// coordinates.  If non-nil, this must be a slice of three non-negative
// numbers.  The default is [0 0 0].
//
// Gamma (optional) gives the gamma values for the red, green and blue
// components.  If non-nil, this must be a slice of three numbers.  The default
// is [1 1 1].
//
// Matrix (optional) is a 3x3 matrix.  The default is [1 0 0 0 1 0 0 0 1].
//
// DefName is the default resource name to use within content streams.
// This can be left empty to allocate names automatically.
func CalRGB(whitePoint, blackPoint, gamma, matrix []float64) (*SpaceCalRGB, error) {
	if !isValidWhitePoint(whitePoint) {
		return nil, errors.New("CalRGB: invalid white point")
	}
	if blackPoint == nil {
		blackPoint = []float64{0, 0, 0}
	} else if !isValidBlackPoint(blackPoint) {
		return nil, errors.New("CalRGB: invalid black point")
	}
	if gamma == nil {
		gamma = []float64{1, 1, 1}
	} else if len(gamma) != 3 {
		return nil, errors.New("CalRGB: invalid gamma")
	}
	if matrix == nil {
		matrix = []float64{1, 0, 0, 0, 1, 0, 0, 0, 1}
	} else if len(matrix) != 9 {
		return nil, errors.New("CalRGB: invalid matrix")
	}

	return &SpaceCalRGB{
		whitePoint: whitePoint,
		blackPoint: blackPoint,
		gamma:      gamma,
		matrix:     matrix,
	}, nil
}

// New returns a new CalRGB color.
func (s *SpaceCalRGB) New(r, g, b float64) Color {
	if r < 0 || r > 1 || g < 0 || g > 1 || b < 0 || b > 1 {
		// TODO(voss): clamp the values instead?
		return nil
	}
	return colorCalRGB{Space: s, R: r, G: g, B: b}
}

// Embed implements the [Space] interface.
func (s *SpaceCalRGB) Embed(rm *pdf.ResourceManager) (pdf.Object, pdf.Unused, error) {
	var zero pdf.Unused
	if err := pdf.CheckVersion(rm.Out, "CalRGB color space", pdf.V1_1); err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{}
	dict["WhitePoint"] = toPDF(s.whitePoint)
	if !isZero(s.blackPoint) {
		dict["BlackPoint"] = toPDF(s.blackPoint)
	}
	if !isConst(s.gamma, 1) {
		dict["Gamma"] = toPDF(s.gamma)
	}
	if !isValues(s.matrix, 1, 0, 0, 0, 1, 0, 0, 0, 1) {
		dict["Matrix"] = toPDF(s.matrix)
	}

	return pdf.Array{pdf.Name("CalRGB"), dict}, zero, nil
}

// defaultValues implements the [Space] interface.
func (s *SpaceCalRGB) defaultValues() []float64 {
	return []float64{0, 0, 0}
}

// ColorSpaceFamily implements the [Space] interface.
func (s *SpaceCalRGB) ColorSpaceFamily() pdf.Name {
	return "CalRGB"
}

type colorCalRGB struct {
	Space   *SpaceCalRGB
	R, G, B float64
}

// ColorSpace implements the [Color] interface.
func (c colorCalRGB) ColorSpace() Space {
	return c.Space
}

// values implements the [Color] interface.
func (c colorCalRGB) values() []float64 {
	return []float64{c.R, c.G, c.B}
}

// == Lab ====================================================================

// SpaceLab represents a CIE 1976 L*a*b* color space.
type SpaceLab struct {
	whitePoint []float64
	blackPoint []float64
	ranges     []float64
}

// Lab returns a new CIE 1976 L*a*b* color space.
//
// WhitePoint is the diffuse white point in CIE 1931 XYZ coordinates.  This
// must be a slice of length 3, with positive entries, and Y=1.
//
// BlackPoint (optional) is the diffuse black point in the CIE 1931 XYZ
// coordinates.  If non-nil, this must be a slice of three non-negative
// numbers.  The default is [0 0 0].
//
// Ranges (optional) is a slice of four numbers, [aMin, aMax, bMin, bMax],
// which define the valid range of the a* and b* components.
// The default is [-100 100 -100 100].
func Lab(whitePoint, blackPoint, ranges []float64) (*SpaceLab, error) {
	if !isValidWhitePoint(whitePoint) {
		return nil, errors.New("Lab: invalid white point")
	}
	if blackPoint == nil {
		blackPoint = []float64{0, 0, 0}
	} else if !isValidBlackPoint(blackPoint) {
		return nil, errors.New("Lab: invalid black point")
	}
	if ranges == nil {
		ranges = []float64{-100, 100, -100, 100}
	} else if len(ranges) != 4 || ranges[0] >= ranges[1] || ranges[2] >= ranges[3] {
		return nil, errors.New("Lab: invalid ranges")
	}

	return &SpaceLab{
		whitePoint: whitePoint,
		blackPoint: blackPoint,
		ranges:     ranges,
	}, nil
}

// New returns a new Lab color.
func (s *SpaceLab) New(l, a, b float64) (Color, error) {
	if l < 0 || l > 100 {
		return nil, fmt.Errorf("Lab: invalid L* value %g∉[0,100]", l)
	}
	if a < s.ranges[0] || a > s.ranges[1] {
		return nil, fmt.Errorf("Lab: invalid a* value %g∉[%g,%g]",
			a, s.ranges[0], s.ranges[1])
	}
	if b < s.ranges[2] || b > s.ranges[3] {
		return nil, fmt.Errorf("Lab: invalid b* value %g∉[%g,%g]",
			b, s.ranges[2], s.ranges[3])
	}

	return colorLab{Space: s, L: l, A: a, B: b}, nil
}

// ColorSpaceFamily implements the [Space] interface.
func (s *SpaceLab) ColorSpaceFamily() pdf.Name {
	return "Lab"
}

// Embed implements the [Space] interface.
func (s *SpaceLab) Embed(rm *pdf.ResourceManager) (pdf.Object, pdf.Unused, error) {
	var zero pdf.Unused
	if err := pdf.CheckVersion(rm.Out, "Lab color space", pdf.V1_1); err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{}
	dict["WhitePoint"] = toPDF(s.whitePoint)
	if !isZero(s.blackPoint) {
		dict["BlackPoint"] = toPDF(s.blackPoint)
	}
	if !isValues(s.ranges, -100, 100, -100, 100) {
		dict["Range"] = toPDF(s.ranges)
	}

	return pdf.Array{FamilyLab, dict}, zero, nil
}

// defaultValues implements the [Space] interface.
func (s *SpaceLab) defaultValues() []float64 {
	a := 0.0
	if a < s.ranges[0] {
		a = s.ranges[0]
	} else if a > s.ranges[1] {
		a = s.ranges[1]
	}
	b := 0.0
	if b < s.ranges[2] {
		b = s.ranges[2]
	} else if b > s.ranges[3] {
		b = s.ranges[3]
	}
	return []float64{0, a, b}
}

type colorLab struct {
	Space   *SpaceLab
	L, A, B float64
}

// ColorSpace implements the [Color] interface.
func (c colorLab) ColorSpace() Space {
	return c.Space
}

// values implements the [Color] interface.
func (c colorLab) values() []float64 {
	return []float64{c.L, c.A, c.B}
}
