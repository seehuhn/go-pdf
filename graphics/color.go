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

package graphics

import (
	"errors"
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/float"
)

// This file implements functions to set the stroke and fill colors.
// The operators used here are defined in table 73 of ISO 32000-2:2020.

// ColorSpace represents a PDF color space.
type ColorSpace interface {
	pdf.Resource

	ColorSpaceFamily() string

	defaultColor() Color
}

// The following types implement the ColorSpace interface:
var (
	_ ColorSpace = ColorSpaceDeviceGray{}
	_ ColorSpace = ColorSpaceDeviceRGB{}
	_ ColorSpace = ColorSpaceDeviceCMYK{}
	_ ColorSpace = (*ColorSpaceCalGray)(nil)
	_ ColorSpace = (*ColorSpaceCalRGB)(nil)
	_ ColorSpace = (*ColorSpaceLab)(nil)
	_ ColorSpace = colorSpacePattern{}
	_ ColorSpace = colorSpacePatternUncolored{}
	// TODO(voss): ICCBased
	// TODO(voss): indexed
	// TODO(voss): Separation colour spaces
	// TODO(voss): DeviceN colour spaces
)

// Color represents a PDF color.
type Color interface {
	ColorSpace() ColorSpace
	values() []float64
}

// The following types implement the Color interface.
var (
	_ Color = colorDeviceGray(0)
	_ Color = colorDeviceRGB{0, 0, 0}
	_ Color = colorDeviceCMYK{0, 0, 0, 1}
	_ Color = colorCalGray{}
	_ Color = colorCalRGB{}
	_ Color = colorLab{}
	// TODO(voss): colored tiling patterns
	_ Color = colorTilingPattern{}
	// TODO(voss): shading patterns
	// TODO(voss): ICCBased
	// TODO(voss): indexed
	// TODO(voss): Separation colour spaces
	// TODO(voss): DeviceN colour spaces
)

// SetStrokeColor sets the color to use for stroking operations.
func (w *Writer) SetStrokeColor(c Color) {
	if !w.isValid("SetStrokeColor", objPage|objText) {
		return
	}

	cs := c.ColorSpace()
	if cs != DeviceGray && cs != DeviceRGB && cs != DeviceCMYK {
		minVersion := pdf.V1_1
		switch cs.(type) {
		case colorSpacePattern, colorSpacePatternUncolored:
			minVersion = pdf.V1_2
		}
		// TODO(voss): Separation: minVersion = pdf.V1_2
		// TODO(voss): ICCBased, DeviceN: minVersion = pdf.V1_3
		if w.Version < minVersion {
			w.Err = &pdf.VersionError{
				Operation: cs.ColorSpaceFamily() + " colors",
				Earliest:  minVersion,
			}
			return
		}

		if !w.isSet(StateStrokeColor) || w.StrokeColor.ColorSpace() != cs {
			w.StrokeColor = cs.defaultColor()
			w.Set |= StateStrokeColor

			name := w.getResourceName(catColorSpace, cs)
			w.Err = name.PDF(w.Content)
			if w.Err != nil {
				return
			}
			_, w.Err = fmt.Fprintln(w.Content, " CS")
			if w.Err != nil {
				return
			}
		}
	}

	if w.isSet(StateStrokeColor) && w.StrokeColor == c {
		return
	}

	w.StrokeColor = c
	w.Set |= StateStrokeColor

	for _, val := range c.values() {
		valString := float.Format(val, 3)
		_, w.Err = fmt.Fprint(w.Content, valString, " ")
		if w.Err != nil {
			return
		}
	}
	switch c := c.(type) {
	case colorDeviceGray:
		_, w.Err = fmt.Fprintln(w.Content, "G")
	case colorDeviceRGB:
		_, w.Err = fmt.Fprintln(w.Content, "RG")
	case colorDeviceCMYK:
		_, w.Err = fmt.Fprintln(w.Content, "K")
	case colorCalGray, colorCalRGB, colorLab:
		_, w.Err = fmt.Fprintln(w.Content, "SC")
	case colorTilingPattern:
		name := w.getResourceName(catPattern, c.pattern)
		w.Err = name.PDF(w.Content)
		if w.Err != nil {
			return
		}
		_, w.Err = fmt.Fprintln(w.Content, " SCN")
	default:
		panic("unreachable")
	}
}

// SetFillColor sets the color to use for non-stroking operations.
func (w *Writer) SetFillColor(c Color) {
	if !w.isValid("SetFillColor", objPage|objText) {
		return
	}

	cs := c.ColorSpace()
	if cs != DeviceGray && cs != DeviceRGB && cs != DeviceCMYK {
		minVersion := pdf.V1_1
		switch cs.(type) {
		case colorSpacePattern, colorSpacePatternUncolored:
			minVersion = pdf.V1_2
		}
		// TODO(voss): Separation: minVersion = pdf.V1_2
		// TODO(voss): ICCBased, DeviceN: minVersion = pdf.V1_3
		if w.Version < minVersion {
			w.Err = &pdf.VersionError{
				Operation: cs.ColorSpaceFamily() + " colors",
				Earliest:  minVersion,
			}
			return
		}

		if !w.isSet(StateFillColor) || w.FillColor.ColorSpace() != cs {
			w.FillColor = cs.defaultColor()
			w.Set |= StateFillColor

			name := w.getResourceName(catColorSpace, cs)
			w.Err = name.PDF(w.Content)
			if w.Err != nil {
				return
			}
			_, w.Err = fmt.Fprintln(w.Content, " cs")
			if w.Err != nil {
				return
			}
		}
	}

	if w.isSet(StateFillColor) && w.FillColor == c {
		return
	}

	w.FillColor = c
	w.Set |= StateFillColor

	for _, val := range c.values() {
		valString := float.Format(val, 3)
		_, w.Err = fmt.Fprint(w.Content, valString, " ")
		if w.Err != nil {
			return
		}
	}
	switch c := c.(type) {
	case colorDeviceGray:
		_, w.Err = fmt.Fprintln(w.Content, "g")
	case colorDeviceRGB:
		_, w.Err = fmt.Fprintln(w.Content, "rg")
	case colorDeviceCMYK:
		_, w.Err = fmt.Fprintln(w.Content, "k")
	case colorCalGray, colorCalRGB, colorLab:
		_, w.Err = fmt.Fprintln(w.Content, "sc")
	case colorTilingPattern:
		name := w.getResourceName(catPattern, c.pattern)
		w.Err = name.PDF(w.Content)
		if w.Err != nil {
			return
		}
		_, w.Err = fmt.Fprintln(w.Content, " scn")
	default:
		panic("unreachable")
	}
}

// == DeviceGray =============================================================

// ColorSpaceDeviceGray represents the DeviceGray color space.
// Use [DeviceGray] to access this color space.
type ColorSpaceDeviceGray struct{}

// DefaultName implements the [ColorSpace] interface.
func (s ColorSpaceDeviceGray) DefaultName() pdf.Name {
	return ""
}

// PDFObject implements the [ColorSpace] interface.
func (s ColorSpaceDeviceGray) PDFObject() pdf.Object {
	return pdf.Name("DeviceGray")
}

// ColorSpaceFamily implements the [ColorSpace] interface.
func (s ColorSpaceDeviceGray) ColorSpaceFamily() string {
	return "DeviceGray"
}

// defaultColor implements the [ColorSpace] interface.
func (s ColorSpaceDeviceGray) defaultColor() Color {
	return colorDeviceGray(0)
}

// New returns a color in the DeviceGray color space.
func (s ColorSpaceDeviceGray) New(gray float64) Color {
	return colorDeviceGray(gray)
}

// DeviceGray is the DeviceGray color space.
var DeviceGray = ColorSpaceDeviceGray{}

type colorDeviceGray float64

// ColorSpace implements the [Color] interface.
func (c colorDeviceGray) ColorSpace() ColorSpace {
	return DeviceGray
}

// values implements the [Color] interface.
func (c colorDeviceGray) values() []float64 {
	return []float64{float64(c)}
}

// == DeviceRGB ==============================================================

// ColorSpaceDeviceRGB represents the DeviceRGB color space.
// Use [DeviceRGB] to access this color space.
type ColorSpaceDeviceRGB struct{}

// DefaultName implements the [ColorSpace] interface.
func (s ColorSpaceDeviceRGB) DefaultName() pdf.Name {
	return ""
}

// PDFObject implements the [ColorSpace] interface.
func (s ColorSpaceDeviceRGB) PDFObject() pdf.Object {
	return pdf.Name("DeviceRGB")
}

// ColorSpaceFamily implements the [ColorSpace] interface.
func (s ColorSpaceDeviceRGB) ColorSpaceFamily() string {
	return "DeviceRGB"
}

// defaultColor implements the [ColorSpace] interface.
func (s ColorSpaceDeviceRGB) defaultColor() Color {
	return colorDeviceRGB{0, 0, 0}
}

// New returns a color in the DeviceRGB color space.
func (s ColorSpaceDeviceRGB) New(r, g, b float64) Color {
	return colorDeviceRGB{r, g, b}
}

// DeviceRGB is the DeviceRGB color space.
var DeviceRGB = ColorSpaceDeviceRGB{}

type colorDeviceRGB [3]float64

// ColorSpace implements the [Color] interface.
func (c colorDeviceRGB) ColorSpace() ColorSpace {
	return DeviceRGB
}

// values implements the [Color] interface.
func (c colorDeviceRGB) values() []float64 {
	return c[:]
}

// == DeviceCMYK =============================================================

// ColorSpaceDeviceCMYK represents the DeviceCMYK color space.
// Use [DeviceCMYK] to access this color space.
type ColorSpaceDeviceCMYK struct{}

// DefaultName implements the [ColorSpace] interface.
func (s ColorSpaceDeviceCMYK) DefaultName() pdf.Name {
	return ""
}

// PDFObject implements the [ColorSpace] interface.
func (s ColorSpaceDeviceCMYK) PDFObject() pdf.Object {
	return pdf.Name("DeviceCMYK")
}

// ColorSpaceFamily implements the [ColorSpace] interface.
func (s ColorSpaceDeviceCMYK) ColorSpaceFamily() string {
	return "DeviceCMYK"
}

// defaultColor implements the [ColorSpace] interface.
func (s ColorSpaceDeviceCMYK) defaultColor() Color {
	return colorDeviceCMYK{0, 0, 0, 1}
}

// New returns a color in the DeviceCMYK color space.
func (s ColorSpaceDeviceCMYK) New(c, m, y, k float64) Color {
	return colorDeviceCMYK{c, m, y, k}
}

// DeviceCMYK is the DeviceCMYK color space.
var DeviceCMYK = ColorSpaceDeviceCMYK{}

type colorDeviceCMYK [4]float64

// ColorSpace implements the [Color] interface.
func (c colorDeviceCMYK) ColorSpace() ColorSpace {
	return DeviceCMYK
}

// values implements the [Color] interface.
func (c colorDeviceCMYK) values() []float64 {
	return c[:]
}

// == CalGray ================================================================

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
func CalGray(whitePoint, blackPoint []float64, gamma float64, defName pdf.Name) (*ColorSpaceCalGray, error) {
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

	dict := pdf.Dict{}
	dict["WhitePoint"] = toPDF(whitePoint)
	if !isZero(blackPoint) {
		dict["BlackPoint"] = toPDF(blackPoint)
	}
	if gamma != 1 {
		dict["Gamma"] = pdf.Number(gamma)
	}

	return &ColorSpaceCalGray{
		Res: pdf.Res{
			DefName: defName,
			Ref:     pdf.Array{pdf.Name("CalGray"), dict},
		},
		whitePoint: whitePoint,
		blackPoint: blackPoint,
		gamma:      gamma}, nil
}

// ColorSpaceCalGray represents a CalGray color space.
type ColorSpaceCalGray struct {
	pdf.Res
	whitePoint []float64
	blackPoint []float64
	gamma      float64
}

// Embed embeds the color space in the PDF file.
// This saves space in case the color space is used in multiple content streams.
func (s *ColorSpaceCalGray) Embed(out *pdf.Writer) (*ColorSpaceCalGray, error) {
	if _, ok := s.Res.Ref.(pdf.Reference); ok {
		return s, nil
	}
	ref := out.Alloc()
	err := out.Put(ref, s.Res.Ref)
	if err != nil {
		return nil, err
	}

	res := clone(s)
	res.Res.Ref = ref
	return res, nil
}

// ColorSpaceFamily implements the [ColorSpace] interface.
func (s *ColorSpaceCalGray) ColorSpaceFamily() string {
	return "CalGray"
}

// defaultColor implements the [ColorSpace] interface.
func (s *ColorSpaceCalGray) defaultColor() Color {
	return colorCalGray{Space: s, Value: 0}
}

// New returns a new CalGray color.
func (s *ColorSpaceCalGray) New(gray float64) Color {
	return colorCalGray{Space: s, Value: gray}
}

type colorCalGray struct {
	Space *ColorSpaceCalGray
	Value float64
}

// ColorSpace implements the [Color] interface.
func (c colorCalGray) ColorSpace() ColorSpace {
	return c.Space
}

// values implements the [Color] interface.
func (c colorCalGray) values() []float64 {
	return []float64{c.Value}
}

// == CalRGB =================================================================

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
func CalRGB(whitePoint, blackPoint, gamma, matrix []float64, defName pdf.Name) (*ColorSpaceCalRGB, error) {
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

	dict := pdf.Dict{}
	dict["WhitePoint"] = toPDF(whitePoint)
	if !isZero(blackPoint) {
		dict["BlackPoint"] = toPDF(blackPoint)
	}
	if !isConst(gamma, 1) {
		dict["Gamma"] = toPDF(gamma)
	}
	if !isValues(matrix, 1, 0, 0, 0, 1, 0, 0, 0, 1) {
		dict["Matrix"] = toPDF(matrix)
	}

	return &ColorSpaceCalRGB{
		Res: pdf.Res{
			DefName: defName,
			Ref:     pdf.Array{pdf.Name("CalRGB"), dict},
		},
		whitePoint: whitePoint,
		blackPoint: blackPoint,
		gamma:      gamma,
		matrix:     matrix,
	}, nil
}

// ColorSpaceCalRGB represents a CalRGB color space.
type ColorSpaceCalRGB struct {
	pdf.Res
	whitePoint []float64
	blackPoint []float64
	gamma      []float64
	matrix     []float64
}

// Embed embeds the color space in the PDF file.
// This saves space in case the color space is used in multiple content streams.
func (s *ColorSpaceCalRGB) Embed(out *pdf.Writer) (*ColorSpaceCalRGB, error) {
	if _, ok := s.Res.Ref.(pdf.Reference); ok {
		return s, nil
	}
	ref := out.Alloc()
	err := out.Put(ref, s.Res.Ref)
	if err != nil {
		return nil, err
	}

	embedded := clone(s)
	embedded.Res.Ref = ref
	return embedded, nil
}

// ColorSpaceFamily implements the [ColorSpace] interface.
func (s *ColorSpaceCalRGB) ColorSpaceFamily() string {
	return "CalRGB"
}

// defaultColor implements the [ColorSpace] interface.
func (s *ColorSpaceCalRGB) defaultColor() Color {
	return colorCalRGB{Space: s, R: 0, G: 0, B: 0}
}

// New returns a new CalRGB color.
func (s *ColorSpaceCalRGB) New(r, g, b float64) Color {
	return colorCalRGB{Space: s, R: r, G: g, B: b}
}

type colorCalRGB struct {
	Space   *ColorSpaceCalRGB
	R, G, B float64
}

// ColorSpace implements the [Color] interface.
func (c colorCalRGB) ColorSpace() ColorSpace {
	return c.Space
}

// values implements the [Color] interface.
func (c colorCalRGB) values() []float64 {
	return []float64{c.R, c.G, c.B}
}

// == Lab ====================================================================

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
//
// DefName is the default resource name to use within content streams.
// This can be left empty to allocate names automatically.
func Lab(whitePoint, blackPoint, ranges []float64, defName pdf.Name) (*ColorSpaceLab, error) {
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

	dict := pdf.Dict{}
	dict["WhitePoint"] = toPDF(whitePoint)
	if !isZero(blackPoint) {
		dict["BlackPoint"] = toPDF(blackPoint)
	}
	if !isValues(ranges, -100, 100, -100, 100) {
		dict["Range"] = toPDF(ranges)
	}

	return &ColorSpaceLab{
		Res: pdf.Res{
			DefName: defName,
			Ref:     pdf.Array{pdf.Name("Lab"), dict},
		},
		whitePoint: whitePoint,
		blackPoint: blackPoint,
		ranges:     ranges,
	}, nil
}

// ColorSpaceLab represents a CIE 1976 L*a*b* color space.
type ColorSpaceLab struct {
	pdf.Res
	whitePoint []float64
	blackPoint []float64
	ranges     []float64
}

func (s *ColorSpaceLab) Embed(out *pdf.Writer) (*ColorSpaceLab, error) {
	if _, ok := s.Res.Ref.(pdf.Reference); ok {
		return s, nil
	}
	ref := out.Alloc()
	err := out.Put(ref, s.Res.Ref)
	if err != nil {
		return nil, err
	}

	embedded := clone(s)
	embedded.Res.Ref = ref
	return embedded, nil
}

// ColorSpaceFamily implements the [ColorSpace] interface.
func (s *ColorSpaceLab) ColorSpaceFamily() string {
	return "Lab"
}

// defaultColor implements the [ColorSpace] interface.
func (s *ColorSpaceLab) defaultColor() Color {
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
	return colorLab{Space: s, L: 0, A: a, B: b}
}

// New returns a new Lab color.
func (s *ColorSpaceLab) New(l, a, b float64) (Color, error) {
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

type colorLab struct {
	Space   *ColorSpaceLab
	L, A, B float64
}

// ColorSpace implements the [Color] interface.
func (c colorLab) ColorSpace() ColorSpace {
	return c.Space
}

// values implements the [Color] interface.
func (c colorLab) values() []float64 {
	return []float64{c.L, c.A, c.B}
}

// == ICCBased ===============================================================

// TODO(voss): implement this

// == Indexed ================================================================

// TODO(voss): implement this

// == Separation =============================================================

// TODO(voss): implement this

// == DeviceN ================================================================

// TODO(voss): implement this

// ===========================================================================

func toPDF(x []float64) pdf.Array {
	res := make(pdf.Array, len(x))
	for i, xi := range x {
		res[i] = pdf.Number(xi)
	}
	return res
}

func isConst(x []float64, value float64) bool {
	for _, xi := range x {
		if xi != value {
			return false
		}
	}
	return true
}

func isZero(x []float64) bool {
	return isConst(x, 0)
}

func isPosVec3(x []float64) bool {
	if len(x) != 3 {
		return false
	}
	for _, v := range x {
		if v < 0 {
			return false
		}
	}
	return true
}

func isEqual(x, y []float64) bool {
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

func isValues(x []float64, y ...float64) bool {
	return isEqual(x, y)
}

func clone[T any](x *T) *T {
	y := *x
	return &y
}

// WhitePointD65 represents the D65 whitepoint.
// The given values are CIE 1931 XYZ coordinates.
//
// https://en.wikipedia.org/wiki/Illuminant_D65
var WhitePointD65 = []float64{0.95047, 1.0, 1.08883}

// WhitePointD50 represents the D50 whitepoint.
// The given values are CIE 1931 XYZ coordinates.
//
// https://en.wikipedia.org/wiki/Standard_illuminant#Illuminant_series_D
var WhitePointD50 = []float64{0.964212, 1.0, 0.8251883}
