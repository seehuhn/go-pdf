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
	stdcolor "image/color"
	"math"

	"seehuhn.de/go/pdf"
)

// == CalGray ================================================================

// PDF 2.0 sections: 8.6.5.2

// SpaceCalGray represents a CalGray color space.
// Use [CalGray] to create new CalGray color spaces.
type SpaceCalGray struct {
	// WhitePoint is the diffuse white point in CIE 1931 XYZ coordinates
	// (positive entries, Y=1).
	WhitePoint [3]float64

	// BlackPoint is the diffuse black point in CIE 1931 XYZ coordinates
	// (non-negative entries).
	BlackPoint [3]float64

	// Gamma is the gamma value (positive).
	Gamma float64
}

// CalGray returns a new CalGray color space.
//
// WhitePoint is the diffuse white point in CIE 1931 XYZ coordinates.  This
// must be a slice of length 3, with positive entries, and Y=1.
// This is typically one of [WhitePointD65] or [WhitePointD50].
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

	res := &SpaceCalGray{Gamma: gamma}
	copy(res.WhitePoint[:], whitePoint)
	copy(res.BlackPoint[:], blackPoint)
	return res, nil
}

// Family returns /CalGray.
// This implements the [Space] interface.
func (s *SpaceCalGray) Family() pdf.Name {
	return FamilyCalGray
}

// Channels returns 1.
// This implements the [Space] interface.
func (s *SpaceCalGray) Channels() int {
	return 1
}

// Default returns the black in the CalGray color space.
// This implements the [Space] interface.
func (s *SpaceCalGray) Default() Color {
	return colorCalGray{Space: s, Value: 0}
}

// New returns a new CalGray color.
// The parameter gray must be in the range from 0 (black) to 1 (white).
func (s *SpaceCalGray) New(gray float64) Color {
	return colorCalGray{Space: s, Value: gray}
}

// Convert converts a color to the CalGray color space.
// This implements the [stdcolor.Model] interface.
func (s *SpaceCalGray) Convert(c stdcolor.Color) stdcolor.Color {
	// fast path: already this CalGray space
	if cg, ok := c.(colorCalGray); ok && cg.Space == s {
		return cg
	}

	// convert via XYZ
	X, Y, Z := ColorToXYZ(c)
	return s.FromXYZ(X, Y, Z)
}

// FromXYZ converts D50-adapted CIE XYZ coordinates to a CalGray color.
// Only the Y component (luminance) is used after adaptation.
func (s *SpaceCalGray) FromXYZ(X, Y, Z float64) Color {
	_, Ya, _ := bradfordAdapt(X, Y, Z, WhitePointD50, s.WhitePoint[:])
	yNorm := Ya / s.WhitePoint[1]
	if yNorm <= 0 {
		return colorCalGray{Space: s, Value: 0}
	}
	if yNorm >= 1 {
		return colorCalGray{Space: s, Value: 1}
	}
	gray := math.Pow(yNorm, 1.0/s.Gamma)
	return colorCalGray{Space: s, Value: clamp(gray, 0, 1)}
}

// Embed adds the color space to a PDF file.
// This implements the [Space] interface.
func (s *SpaceCalGray) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "CalGray color space", pdf.V1_1); err != nil {
		return nil, err
	}

	dict := pdf.Dict{}
	dict["WhitePoint"] = toPDF(s.WhitePoint[:])
	if !isZero(s.BlackPoint[:]) {
		dict["BlackPoint"] = toPDF(s.BlackPoint[:])
	}
	if math.Abs(s.Gamma-1) >= ε {
		dict["Gamma"] = pdf.Number(s.Gamma)
	}

	return pdf.Array{pdf.Name("CalGray"), dict}, nil
}

type colorCalGray struct {
	Space *SpaceCalGray
	Value float64
}

// ColorSpace implements the [Color] interface.
func (c colorCalGray) ColorSpace() Space {
	return c.Space
}

// ToXYZ converts a CalGray color to CIE XYZ tristimulus values
// adapted to the D50 illuminant.
func (c colorCalGray) ToXYZ() (X, Y, Z float64) {
	A := math.Pow(c.Value, c.Space.Gamma)
	X = c.Space.WhitePoint[0] * A
	Y = c.Space.WhitePoint[1] * A
	Z = c.Space.WhitePoint[2] * A
	return bradfordAdapt(X, Y, Z, c.Space.WhitePoint[:], WhitePointD50)
}

// RGBA implements the color.Color interface.
func (c colorCalGray) RGBA() (r, g, b, a uint32) {
	X, Y, Z := c.ToXYZ()
	rf, gf, bf := XYZToSRGB(X, Y, Z)
	return toUint32(rf), toUint32(gf), toUint32(bf), 0xffff
}

// == CalRGB =================================================================

// PDF 2.0 sections: 8.6.5.3

// SpaceCalRGB represents a CalRGB color space.
// Use [CalRGB] to create new CalRGB color spaces.
type SpaceCalRGB struct {
	// WhitePoint is the diffuse white point in CIE 1931 XYZ coordinates
	// (positive entries, Y=1).
	WhitePoint [3]float64

	// BlackPoint is the diffuse black point in CIE 1931 XYZ coordinates
	// (non-negative entries).
	BlackPoint [3]float64

	// Gamma contains the gamma values for R, G, B (all positive).
	Gamma [3]float64

	// Matrix is a 3x3 matrix in column-major order that maps
	// decoded ABC values to CIE 1931 XYZ coordinates.
	Matrix [9]float64
}

// CalRGB returns a new CalRGB color space.
//
// WhitePoint is the diffuse white point in CIE 1931 XYZ coordinates.  This
// must be a slice of length 3, with positive entries, and Y=1.
// This is typically one of [WhitePointD65] or [WhitePointD50].
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

	res := &SpaceCalRGB{}
	copy(res.WhitePoint[:], whitePoint)
	copy(res.BlackPoint[:], blackPoint)
	copy(res.Gamma[:], gamma)
	copy(res.Matrix[:], matrix)
	return res, nil
}

// Embed adds the color space to a PDF file.
// This implements the [Space] interface.
func (s *SpaceCalRGB) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "CalRGB color space", pdf.V1_1); err != nil {
		return nil, err
	}

	dict := pdf.Dict{}
	dict["WhitePoint"] = toPDF(s.WhitePoint[:])
	if !isZero(s.BlackPoint[:]) {
		dict["BlackPoint"] = toPDF(s.BlackPoint[:])
	}
	if !isConst(s.Gamma[:], 1) {
		dict["Gamma"] = toPDF(s.Gamma[:])
	}
	if !isValues(s.Matrix[:], 1, 0, 0, 0, 1, 0, 0, 0, 1) {
		dict["Matrix"] = toPDF(s.Matrix[:])
	}

	return pdf.Array{pdf.Name("CalRGB"), dict}, nil
}

// New returns a new CalRGB color.
// The parameters r, g, and b must be in the range [0, 1].
func (s *SpaceCalRGB) New(r, g, b float64) Color {
	return colorCalRGB{Space: s, Values: [3]float64{r, g, b}}
}

// Convert converts a color to the CalRGB color space.
// This implements the [stdcolor.Model] interface.
func (s *SpaceCalRGB) Convert(c stdcolor.Color) stdcolor.Color {
	// fast path: already this CalRGB space
	if cr, ok := c.(colorCalRGB); ok && cr.Space == s {
		return cr
	}

	// convert via XYZ
	X, Y, Z := ColorToXYZ(c)
	return s.FromXYZ(X, Y, Z)
}

// FromXYZ converts D50-adapted CIE XYZ coordinates to a CalRGB color.
func (s *SpaceCalRGB) FromXYZ(X, Y, Z float64) Color {
	X, Y, Z = bradfordAdapt(X, Y, Z, WhitePointD50, s.WhitePoint[:])

	// invert the matrix (stored in column-major order in matrix field)
	m := s.Matrix
	det := m[0]*(m[4]*m[8]-m[5]*m[7]) - m[3]*(m[1]*m[8]-m[2]*m[7]) + m[6]*(m[1]*m[5]-m[2]*m[4])
	if det == 0 {
		return colorCalRGB{Space: s, Values: [3]float64{0, 0, 0}}
	}
	invDet := 1.0 / det

	// inverse matrix elements
	i00 := (m[4]*m[8] - m[5]*m[7]) * invDet
	i01 := (m[6]*m[5] - m[3]*m[8]) * invDet
	i02 := (m[3]*m[7] - m[6]*m[4]) * invDet
	i10 := (m[2]*m[7] - m[1]*m[8]) * invDet
	i11 := (m[0]*m[8] - m[6]*m[2]) * invDet
	i12 := (m[6]*m[1] - m[0]*m[7]) * invDet
	i20 := (m[1]*m[5] - m[2]*m[4]) * invDet
	i21 := (m[3]*m[2] - m[0]*m[5]) * invDet
	i22 := (m[0]*m[4] - m[3]*m[1]) * invDet

	// linear RGB values
	A := i00*X + i01*Y + i02*Z
	B := i10*X + i11*Y + i12*Z
	C := i20*X + i21*Y + i22*Z

	// apply inverse gamma
	r := invGamma(A, s.Gamma[0])
	g := invGamma(B, s.Gamma[1])
	b := invGamma(C, s.Gamma[2])

	return colorCalRGB{Space: s, Values: [3]float64{
		clamp(r, 0, 1),
		clamp(g, 0, 1),
		clamp(b, 0, 1),
	}}
}

func invGamma(v, gamma float64) float64 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 1
	}
	return math.Pow(v, 1.0/gamma)
}

// Channels returns 3.
// This implements the [Space] interface.
func (s *SpaceCalRGB) Channels() int {
	return 3
}

// Default returns the black in the CalRGB color space.
// This implements the [Space] interface.
func (s *SpaceCalRGB) Default() Color {
	return colorCalRGB{Space: s}
}

// Family returns /CalRGB.
// This implements the [Space] interface.
func (s *SpaceCalRGB) Family() pdf.Name {
	return FamilyCalRGB
}

type colorCalRGB struct {
	Space  *SpaceCalRGB
	Values [3]float64 // R, G, B
}

// ColorSpace implements the [Color] interface.
func (c colorCalRGB) ColorSpace() Space {
	return c.Space
}

// ToXYZ converts a CalRGB color to CIE XYZ tristimulus values
// adapted to the D50 illuminant.
func (c colorCalRGB) ToXYZ() (X, Y, Z float64) {
	// apply gamma to each component
	A := math.Pow(c.Values[0], c.Space.Gamma[0])
	B := math.Pow(c.Values[1], c.Space.Gamma[1])
	C := math.Pow(c.Values[2], c.Space.Gamma[2])

	// apply the 3x3 matrix (stored in column-major order)
	m := c.Space.Matrix
	X = m[0]*A + m[3]*B + m[6]*C
	Y = m[1]*A + m[4]*B + m[7]*C
	Z = m[2]*A + m[5]*B + m[8]*C
	return bradfordAdapt(X, Y, Z, c.Space.WhitePoint[:], WhitePointD50)
}

// RGBA implements the color.Color interface.
func (c colorCalRGB) RGBA() (r, g, b, a uint32) {
	X, Y, Z := c.ToXYZ()
	rf, gf, bf := XYZToSRGB(X, Y, Z)
	return toUint32(rf), toUint32(gf), toUint32(bf), 0xffff
}

// == Lab ====================================================================
//
// PDF Lab is a CIE-based color space defined in ISO 32000-1:2008, section
// 8.6.5.4. It represents colors using CIE 1976 L*a*b* coordinates: L*
// (lightness, 0–100) and a*, b* (chromaticity, with configurable ranges).
//
// The color space is parameterized by a WhitePoint (required) and BlackPoint
// (optional). The L*a*b* ↔ XYZ conversion uses only the WhitePoint; BlackPoint
// does not affect this transformation. BlackPoint is instead used for black
// point compensation during rendering, as specified in ISO 18619 and
// controlled by the UseBlackPtComp entry in the graphics state (see ISO
// 32000-1:2008, section 8.6.5.9).
//
// For further details on CIE-based color conversion in PDF, see ISO
// 32000-1:2008, sections 8.6.5 (CIE-Based Colour Spaces) and 10.3 (CIE-Based
// Colour to Device Colour).

// PDF 2.0 sections: 8.6.5.4

// SpaceLab represents a CIE 1976 L*a*b* color space.
type SpaceLab struct {
	// WhitePoint is the diffuse white point in CIE 1931 XYZ coordinates
	// (positive entries, Y=1).
	WhitePoint [3]float64

	// BlackPoint is the diffuse black point in CIE 1931 XYZ coordinates
	// (non-negative entries).
	BlackPoint [3]float64

	// Ranges is [aMin, aMax, bMin, bMax] for the a* and b* components.
	Ranges [4]float64
}

// Lab returns a new CIE 1976 L*a*b* color space.
//
// WhitePoint is the diffuse white point in CIE 1931 XYZ coordinates.  This
// must be a slice of length 3, with positive entries, and Y=1.
// This is typically one of [WhitePointD65] or [WhitePointD50].
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

	res := &SpaceLab{}
	copy(res.WhitePoint[:], whitePoint)
	copy(res.BlackPoint[:], blackPoint)
	copy(res.Ranges[:], ranges)
	return res, nil
}

// Family returns /Lab.
// This implements the [Space] interface.
func (s *SpaceLab) Family() pdf.Name {
	return FamilyLab
}

// New returns a new Lab color.
// The parameter l must be in the range [0, 100].
// The parameters a and b must be in the range [aMin, aMax] and [bMin, bMax],
func (s *SpaceLab) New(l, a, b float64) (Color, error) {
	if l < 0 || l > 100 {
		return nil, fmt.Errorf("Lab: invalid L* value %g∉[0,100]", l)
	}
	if a < s.Ranges[0] || a > s.Ranges[1] {
		return nil, fmt.Errorf("Lab: invalid a* value %g∉[%g,%g]",
			a, s.Ranges[0], s.Ranges[1])
	}
	if b < s.Ranges[2] || b > s.Ranges[3] {
		return nil, fmt.Errorf("Lab: invalid b* value %g∉[%g,%g]",
			b, s.Ranges[2], s.Ranges[3])
	}

	return colorLab{Space: s, Values: [3]float64{l, a, b}}, nil
}

// Channels returns 3.
// This implements the [Space] interface.
func (s *SpaceLab) Channels() int {
	return 3
}

// Convert converts a color to the Lab color space.
// This implements the [stdcolor.Model] interface.
func (s *SpaceLab) Convert(c stdcolor.Color) stdcolor.Color {
	// fast path: already this Lab space
	if cl, ok := c.(colorLab); ok && cl.Space == s {
		return cl
	}

	// convert via XYZ
	X, Y, Z := ColorToXYZ(c)
	return s.FromXYZ(X, Y, Z)
}

// Default returns the black (or the closest representable color) in an Lab
// color space.
// This implements the [Space] interface.
func (s *SpaceLab) Default() Color {
	a := 0.0
	if a < s.Ranges[0] {
		a = s.Ranges[0]
	} else if a > s.Ranges[1] {
		a = s.Ranges[1]
	}
	b := 0.0
	if b < s.Ranges[2] {
		b = s.Ranges[2]
	} else if b > s.Ranges[3] {
		b = s.Ranges[3]
	}

	return colorLab{Space: s, Values: [3]float64{0, a, b}}
}

// Embed adds the color space to a PDF file.
// This implements the [Space] interface.
func (s *SpaceLab) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "Lab color space", pdf.V1_1); err != nil {
		return nil, err
	}

	dict := pdf.Dict{}
	dict["WhitePoint"] = toPDF(s.WhitePoint[:])
	if !isZero(s.BlackPoint[:]) {
		dict["BlackPoint"] = toPDF(s.BlackPoint[:])
	}
	if !isValues(s.Ranges[:], -100, 100, -100, 100) {
		dict["Range"] = toPDF(s.Ranges[:])
	}

	return pdf.Array{FamilyLab, dict}, nil
}

type colorLab struct {
	Space  *SpaceLab
	Values [3]float64 // L, a, b
}

// ColorSpace implements the [Color] interface.
func (c colorLab) ColorSpace() Space {
	return c.Space
}

// ToXYZ converts a Lab color to CIE XYZ tristimulus values
// adapted to the D50 illuminant.
func (c colorLab) ToXYZ() (X, Y, Z float64) {
	LStar, aStar, bStar := c.Values[0], c.Values[1], c.Values[2]
	XW, YW, ZW := c.Space.WhitePoint[0], c.Space.WhitePoint[1], c.Space.WhitePoint[2]

	// Stage 1: L*a*b* to intermediate (L, M, N)
	common := (LStar + 16) / 116
	L := common + aStar/500
	M := common
	N := common - bStar/200

	// Stage 2: Intermediate to XYZ
	X = XW * labG(L)
	Y = YW * labG(M)
	Z = ZW * labG(N)
	return bradfordAdapt(X, Y, Z, c.Space.WhitePoint[:], WhitePointD50)
}

// RGBA implements the color.Color interface.
func (c colorLab) RGBA() (r, g, b, a uint32) {
	X, Y, Z := c.ToXYZ()
	rf, gf, bf := XYZToSRGB(X, Y, Z)
	return toUint32(rf), toUint32(gf), toUint32(bf), 0xffff
}

// FromXYZ converts D50-adapted CIE XYZ coordinates to a Lab color.
// Values outside the valid range are clamped.
func (s *SpaceLab) FromXYZ(X, Y, Z float64) Color {
	X, Y, Z = bradfordAdapt(X, Y, Z, WhitePointD50, s.WhitePoint[:])

	XW, YW, ZW := s.WhitePoint[0], s.WhitePoint[1], s.WhitePoint[2]

	// Inverse of stage 2
	L := labF(X / XW)
	M := labF(Y / YW)
	N := labF(Z / ZW)

	// Inverse of stage 1
	LStar := 116*M - 16
	aStar := 500 * (L - M)
	bStar := 200 * (M - N)

	// Clamp to valid ranges
	LStar = clamp(LStar, 0, 100)
	aStar = clamp(aStar, s.Ranges[0], s.Ranges[1])
	bStar = clamp(bStar, s.Ranges[2], s.Ranges[3])

	return colorLab{Space: s, Values: [3]float64{LStar, aStar, bStar}}
}

// labG is the forward transfer function (used in ToXYZ).
func labG(x float64) float64 {
	if x >= 6.0/29.0 {
		return x * x * x
	}
	return (108.0 / 841.0) * (x - 4.0/29.0)
}

// labF is the inverse transfer function (used in FromXYZ).
func labF(t float64) float64 {
	const delta3 = (6.0 / 29.0) * (6.0 / 29.0) * (6.0 / 29.0) // 216/24389
	if t >= delta3 {
		return math.Pow(t, 1.0/3.0)
	}
	return (841.0/108.0)*t + 4.0/29.0
}

func clamp(x, min, max float64) float64 {
	if x < min {
		return min
	}
	if x > max {
		return max
	}
	return x
}
