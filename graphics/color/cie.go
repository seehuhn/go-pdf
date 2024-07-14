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
	if !isPosVec3(whitePoint) || whitePoint[1] != 1 {
		return nil, errors.New("CalGray: invalid white point")
	}
	if blackPoint == nil {
		blackPoint = []float64{0, 0, 0}
	} else if !isPosVec3(blackPoint) {
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
	return "CalGray"
}

// defaultValues implements the [Space] interface.
func (s *SpaceCalGray) defaultValues() []float64 {
	return []float64{0}
}

// Embed implements the [Space] interface.
func (s *SpaceCalGray) Embed(*pdf.ResourceManager) (pdf.Resource, error) {
	dict := pdf.Dict{}
	dict["WhitePoint"] = toPDF(s.whitePoint)
	if !isZero(s.blackPoint) {
		dict["BlackPoint"] = toPDF(s.blackPoint)
	}
	if s.gamma != 1 {
		dict["Gamma"] = pdf.Number(s.gamma)
	}

	return pdf.Res{
		Data: pdf.Array{pdf.Name("CalGray"), dict},
	}, nil
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
	if !isPosVec3(whitePoint) || whitePoint[1] != 1 {
		return nil, errors.New("CalRGB: invalid white point")
	}
	if blackPoint == nil {
		blackPoint = []float64{0, 0, 0}
	} else if !isPosVec3(blackPoint) {
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
func (s *SpaceCalRGB) Embed(*pdf.ResourceManager) (pdf.Resource, error) {
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

	return pdf.Res{
		Data: pdf.Array{pdf.Name("CalRGB"), dict},
	}, nil
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
	if !isPosVec3(whitePoint) || whitePoint[1] != 1 {
		return nil, errors.New("Lab: invalid white point")
	}
	if blackPoint == nil {
		blackPoint = []float64{0, 0, 0}
	} else if !isPosVec3(blackPoint) {
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
func (s *SpaceLab) Embed(*pdf.ResourceManager) (pdf.Resource, error) {
	dict := pdf.Dict{}
	dict["WhitePoint"] = toPDF(s.whitePoint)
	if !isZero(s.blackPoint) {
		dict["BlackPoint"] = toPDF(s.blackPoint)
	}
	if !isValues(s.ranges, -100, 100, -100, 100) {
		dict["Range"] = toPDF(s.ranges)
	}

	return pdf.Res{
		Data: pdf.Array{pdf.Name("Lab"), dict},
	}, nil
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

// == ICCBased ===============================================================

// SpaceICCBased represents an ICC-based color space.
type SpaceICCBased struct {
	n        int
	ranges   []float64
	metadata *pdf.Stream
	profile  []byte
}

// ICCBased returns a new ICC-based color space.
func ICCBased(n int, profile []byte, ranges []float64, metadata *pdf.Stream) (*SpaceICCBased, error) {
	if n != 1 && n != 3 && n != 4 {
		return nil, fmt.Errorf("ICCBased: invalid number of components %d", n)
	}
	if len(profile) == 0 {
		return nil, errors.New("ICCBased: missing profile")
	}
	if ranges == nil {
		ranges = make([]float64, 2*n)
		for i := range ranges {
			ranges[i] = float64(i % 2)
		}
	} else {
		if len(ranges) != 2*n {
			return nil, fmt.Errorf("ICCBased: invalid ranges")
		}
		for i := 0; i < 2*n; i += 2 {
			if ranges[i] > ranges[i+1] {
				return nil, fmt.Errorf("ICCBased: invalid ranges")
			}
		}
	}

	res := &SpaceICCBased{
		n:        n,
		ranges:   ranges,
		metadata: metadata,
		profile:  profile,
	}
	return res, nil
}

// ColorSpaceFamily implements the [Space] interface.
func (s *SpaceICCBased) ColorSpaceFamily() pdf.Name {
	return "ICCBased"
}

// Embed implements the [Space] interface.
func (s *SpaceICCBased) Embed(*pdf.ResourceManager) (pdf.Resource, error) {
	panic("not implemented") // TODO: Implement
}

func (s *SpaceICCBased) defaultValues() []float64 {
	panic("not implemented") // TODO: Implement
}
