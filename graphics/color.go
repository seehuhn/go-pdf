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
	"seehuhn.de/go/pdf/color"
)

// WhitePointD65 represents the D65 whitepoint.
//
// https://en.wikipedia.org/wiki/Illuminant_D65
var WhitePointD65 = []float64{0.95047, 1.0, 1.08883}

type Color struct {
	CS     ColorSpace
	Values []float64
	Name   pdf.Name
}

// ColorSpace represents a PDF color space.
type ColorSpace interface {
	pdf.Resource
	Family() ColorSpaceFamily
	DefaultColor() []float64
}

func (w *Writer) SetStrokeColor(c Color) {
	if !w.isValid("SetStrokeColor", objPage|objText) {
		return
	}

	switch c.CS.Family() {
	case ColorSpaceDeviceGray:
		if len(c.Values) != 1 {
			w.Err = fmt.Errorf("SetStrokeColor: DeviceGray: expected 1 value, got %d", len(c.Values))
			return
		}
		w.SetStrokeColorDeviceGray(c.Values[0])
	case ColorSpaceDeviceRGB:
		if len(c.Values) != 3 {
			w.Err = fmt.Errorf("SetStrokeColor: DeviceRGB: expected 3 values, got %d", len(c.Values))
			return
		}
		w.SetStrokeColorDeviceRGB(c.Values[0], c.Values[1], c.Values[2])
	case ColorSpaceDeviceCMYK:
		if len(c.Values) != 4 {
			w.Err = fmt.Errorf("SetStrokeColor: DeviceCMYK: expected 4 values, got %d", len(c.Values))
			return
		}
		w.SetStrokeColorDeviceCMYK(c.Values[0], c.Values[1], c.Values[2], c.Values[3])
	default:
		w.SetStrokeColorSpace(c.CS)
		if c.Name == "" {
			w.SetStrokeColorValues(c.Values)
		} else {
			w.SetStrokeColorValuesName(c.Values, c.Name)
		}
	}
}

func (w *Writer) SetFillColor(c Color) {
	if !w.isValid("SetFillColor", objPage|objText) {
		return
	}

	switch c.CS.Family() {
	case ColorSpaceDeviceGray:
		if len(c.Values) != 1 {
			w.Err = fmt.Errorf("SetFillColor: DeviceGray: expected 1 value, got %d", len(c.Values))
			return
		}
		w.SetFillColorDeviceGray(c.Values[0])
	case ColorSpaceDeviceRGB:
		if len(c.Values) != 3 {
			w.Err = fmt.Errorf("SetFillColor: DeviceRGB: expected 3 values, got %d", len(c.Values))
			return
		}
		w.SetFillColorDeviceRGB(c.Values[0], c.Values[1], c.Values[2])
	case ColorSpaceDeviceCMYK:
		if len(c.Values) != 4 {
			w.Err = fmt.Errorf("SetFillColor: DeviceCMYK: expected 4 values, got %d", len(c.Values))
			return
		}
		w.SetFillColorDeviceCMYK(c.Values[0], c.Values[1], c.Values[2], c.Values[3])
	default:
		w.SetFillColorSpace(c.CS)
		if c.Name == "" {
			w.SetFillColorValues(c.Values)
		} else {
			w.SetFillColorValuesName(c.Values, c.Name)
		}
	}
}

// ColorSpaceFamily is an enumeration of the color space families defined in PDF-2.0.
type ColorSpaceFamily int

func (cs ColorSpaceFamily) String() string {
	switch cs {
	case ColorSpaceDeviceGray:
		return "DeviceGray"
	case ColorSpaceDeviceRGB:
		return "DeviceRGB"
	case ColorSpaceDeviceCMYK:
		return "DeviceCMYK"
	case ColorSpaceCalGray:
		return "CalGray"
	case ColorSpaceCalRGB:
		return "CalRGB"
	case ColorSpaceLab:
		return "Lab"
	case ColorSpaceICCBased:
		return "ICCBased"
	case ColorSpaceIndexed:
		return "Indexed"
	case ColorSpacePattern:
		return "Pattern"
	case ColorSpaceSeparation:
		return "Separation"
	case ColorSpaceDeviceN:
		return "DeviceN"
	default:
		return fmt.Sprintf("ColorSpaceFamily(%d)", cs)
	}
}

// These are the color space families defined in PDF-2.0.
const (
	ColorSpaceDeviceGray ColorSpaceFamily = iota + 1
	ColorSpaceDeviceRGB
	ColorSpaceDeviceCMYK
	ColorSpaceCalGray
	ColorSpaceCalRGB
	ColorSpaceLab
	ColorSpaceICCBased
	ColorSpaceIndexed
	ColorSpacePattern
	ColorSpaceSeparation
	ColorSpaceDeviceN
)

var colMinVersion = map[ColorSpaceFamily]pdf.Version{
	ColorSpaceICCBased:   pdf.V1_3,
	ColorSpacePattern:    pdf.V1_2,
	ColorSpaceSeparation: pdf.V1_2,
	ColorSpaceDeviceN:    pdf.V1_3,
}

var colNeedsScn = map[ColorSpaceFamily]bool{
	ColorSpacePattern:    true,
	ColorSpaceSeparation: true,
	ColorSpaceDeviceN:    true,
	ColorSpaceICCBased:   true,
}

type simpleColorSpace struct {
	Name     pdf.Name
	Fam      ColorSpaceFamily
	DefColor []float64
}

// DefaultName implements the [ColorSpace] interface.
func (c simpleColorSpace) DefaultName() pdf.Name {
	return ""
}

// PDFObject implements the [ColorSpace] interface.
func (c simpleColorSpace) PDFObject() pdf.Object {
	return c.Name
}

// Family implements the [ColorSpace] interface.
func (c simpleColorSpace) Family() ColorSpaceFamily {
	return c.Fam
}

// DefaultColor implements the [ColorSpace] interface.
func (c simpleColorSpace) DefaultColor() []float64 {
	return c.DefColor
}

var deviceGray = &simpleColorSpace{
	Name:     "DeviceGray",
	Fam:      ColorSpaceDeviceGray,
	DefColor: []float64{0},
}
var deviceRGB = &simpleColorSpace{
	Name:     "DeviceRGB",
	Fam:      ColorSpaceDeviceRGB,
	DefColor: []float64{0, 0, 0},
}
var deviceCMYK = &simpleColorSpace{
	Name:     "DeviceCMYK",
	Fam:      ColorSpaceDeviceCMYK,
	DefColor: []float64{0, 0, 0, 1},
}

// These are the color space families which have no parameters and can
// be represented by singleton objects.
var (
	DeviceGray = deviceGray
	DeviceRGB  = deviceRGB
	DeviceCMYK = deviceCMYK
)

// CalGray returns a new CalGray color space.
//
// The whitePoint and blackPoint parameters are arrays of three numbers
// each, representing the X, Y, and Z components of the points.
// The Y-value of the white point must equal 1.0.
//
// blackPoint is optional, the default value is [0 0 0].
//
// The gamma parameter is a positive number (usually greater than or equal to 1).
func CalGray(whitePoint, blackPoint []float64, gamma float64) (ColorSpace, error) {
	if !isPosVec3(whitePoint) || whitePoint[1] != 1 {
		return nil, errors.New("CalGray: invalid white point")
	}
	if blackPoint != nil && !isPosVec3(blackPoint) {
		return nil, errors.New("CalGray: invalid black point")
	}
	if gamma <= 0 {
		return nil, fmt.Errorf("CalGray: expected gamma > 0, got %f", gamma)
	}

	dict := pdf.Dict{}
	dict["WhitePoint"] = toPDF(whitePoint)
	if len(blackPoint) == 3 && !isZero(blackPoint) {
		dict["BlackPoint"] = toPDF(blackPoint)
	}
	if gamma != 1 {
		dict["Gamma"] = pdf.Number(gamma)
	}
	data := pdf.Array{pdf.Name("CalGray"), dict}

	// TODO(voss): fill in the default values in the struct?

	return &calGray{
		data:       data,
		whitePoint: whitePoint,
		blackPoint: blackPoint,
		gamma:      gamma,
	}, nil
}

type calGray struct {
	data       pdf.Object
	whitePoint []float64
	blackPoint []float64
	gamma      float64
}

func (cs *calGray) DefaultName() pdf.Name {
	return ""
}

func (cs *calGray) PDFObject() pdf.Object {
	return cs.data
}

func (cs *calGray) Family() ColorSpaceFamily {
	return ColorSpaceCalGray
}

func (cs *calGray) DefaultColor() []float64 {
	return []float64{0}
}

// CalRGB allocates a new CalRGB color space.
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
func CalRGB(whitePoint, blackPoint, gamma []float64, matrix []float64) (ColorSpace, error) {
	if !isPosVec3(whitePoint) || whitePoint[1] != 1 {
		return nil, errors.New("CalRGB: invalid white point")
	}
	if blackPoint != nil && !isPosVec3(blackPoint) {
		return nil, errors.New("CalRGB: invalid black point")
	}
	if gamma != nil && len(gamma) != 3 {
		return nil, fmt.Errorf("CalRGB: invalid gamma")
	}
	if matrix != nil && len(matrix) != 9 {
		return nil, fmt.Errorf("CalRGB: invalid matrix")
	}

	dict := pdf.Dict{}
	dict["WhitePoint"] = toPDF(whitePoint)
	if len(blackPoint) == 3 && !isZero(blackPoint) {
		dict["BlackPoint"] = toPDF(blackPoint)
	}
	if len(gamma) == 3 && !isConst(gamma, 1) {
		dict["Gamma"] = toPDF(gamma)
	}
	if len(matrix) == 9 && !isValues(matrix, 1, 0, 0, 0, 1, 0, 0, 0, 1) {
		dict["Matrix"] = toPDF(matrix)
	}
	data := pdf.Array{pdf.Name("CalRGB"), dict}

	// TODO(voss): fill in the default values in the struct?

	return &calRGB{
		data:       data,
		whitePoint: whitePoint,
		blackPoint: blackPoint,
		gamma:      gamma,
		matrix:     matrix,
	}, nil
}

type calRGB struct {
	data       pdf.Object
	whitePoint []float64
	blackPoint []float64
	gamma      []float64
	matrix     []float64
}

func (cs *calRGB) DefaultName() pdf.Name {
	return ""
}

func (cs *calRGB) PDFObject() pdf.Object {
	return cs.data
}

func (cs *calRGB) Family() ColorSpaceFamily {
	return ColorSpaceCalGray
}

func (cs *calRGB) DefaultColor() []float64 {
	return []float64{0, 0, 0}
}

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

// SetStrokeColorOld sets the stroke color in the graphics state.
// If col is nil, the stroke color is not changed.
func (w *Writer) SetStrokeColorOld(col color.Color) {
	if !w.isValid("SetStrokeColor", objPage|objText) {
		return
	}
	if w.isSet(StateStrokeColor) && col == w.StrokeColorOld {
		return
	}
	w.StrokeColorOld = col
	w.Set |= StateStrokeColor
	w.Err = col.SetStroke(w.Content)
}

// SetFillColorOld sets the fill color in the graphics state.
// If col is nil, the fill color is not changed.
func (w *Writer) SetFillColorOld(col color.Color) {
	if !w.isValid("SetFillColor", objPage|objText) {
		return
	}
	if w.isSet(StateFillColor) && col == w.FillColorOld {
		return
	}
	w.FillColorOld = col
	w.Set |= StateFillColor
	w.Err = col.SetFill(w.Content)
}
