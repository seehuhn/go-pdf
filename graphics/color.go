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
// The operators used here are defined in table 73 of
// ISO 32000-2:2020.

// Color represents a PDF color.
type Color interface {
	setStroke(w *Writer)
	setFill(w *Writer)
}

func (w *Writer) SetStrokeColor(c Color) {
	if !w.isValid("SetStrokeColor", objPage|objText) {
		return
	}

	minVersion := pdf.V1_0
	switch c.(type) {
	}
	if w.Version < minVersion {
		w.Err = &pdf.VersionError{Operation: fmt.Sprintf("%T colors", c), Earliest: minVersion}
		return
	}
	c.setStroke(w)
}

func (w *Writer) SetFillColor(c Color) {
	if !w.isValid("SetFillColor", objPage|objText) {
		return
	}

	minVersion := pdf.V1_0
	switch c.(type) {
	}
	if w.Version < minVersion {
		w.Err = &pdf.VersionError{Operation: fmt.Sprintf("%T colors", c), Earliest: minVersion}
		return
	}
	c.setFill(w)
}

// == DeviceGray =============================================================

type colorDeviceGray float64

// DeviceGrayNew returns a new DeviceGray color.
func DeviceGrayNew(gray float64) Color {
	return colorDeviceGray(gray)
}

// setStroke sets the current stroking color space to DeviceGray
// and sets the gray level for stroking.
//
// This implements the PDF graphics operator "G".
func (c colorDeviceGray) setStroke(w *Writer) {
	cur, ok := w.StrokeColor.(colorDeviceGray)
	if ok && w.isSet(StateStrokeColor) && cur == c {
		return
	}

	w.StrokeColor = c
	w.Set |= StateStrokeColor

	// TODO(voss): rounding
	_, w.Err = fmt.Fprintf(w.Content, "%f G\n", c)
}

// setFill sets the current fill color space to DeviceGray
// and sets the gray level for filling.
//
// This implements the PDF graphics operator "g".
func (c colorDeviceGray) setFill(w *Writer) {
	cur, ok := w.FillColor.(colorDeviceGray)
	if ok && w.isSet(StateFillColor) && cur == c {
		return
	}

	w.FillColor = c
	w.Set |= StateFillColor

	// TODO(voss): rounding
	_, w.Err = fmt.Fprintf(w.Content, "%f g\n", c)
}

// == DeviceRGB ==============================================================

type colorDeviceRGB [3]float64

// DeviceRGBNew returns a new DeviceRGB color.
func DeviceRGBNew(r, g, b float64) Color {
	return colorDeviceRGB{r, g, b}
}

// setStroke sets the current stroking color space to DeviceRGB
// and sets the red, green, and blue levels for stroking.
//
// This implements the PDF graphics operator "RG".
func (c colorDeviceRGB) setStroke(w *Writer) {
	cur, ok := w.StrokeColor.(colorDeviceRGB)
	if ok && w.isSet(StateStrokeColor) && cur == c {
		return
	}

	w.StrokeColor = c
	w.Set |= StateStrokeColor

	// TODO(voss): rounding
	_, w.Err = fmt.Fprintf(w.Content, "%f %f %f RG\n", c[0], c[1], c[2])
}

// setFill sets the current fill color space to DeviceRGB
// and sets the red, green, and blue levels for filling.
//
// This implements the PDF graphics operator "rg".
func (c colorDeviceRGB) setFill(w *Writer) {
	cur, ok := w.FillColor.(colorDeviceRGB)
	if ok && w.isSet(StateFillColor) && cur == c {
		return
	}

	w.FillColor = c
	w.Set |= StateFillColor

	// TODO(voss): rounding
	_, w.Err = fmt.Fprintf(w.Content, "%f %f %f rg\n", c[0], c[1], c[2])
}

// == DeviceCMYK =============================================================

type colorDeviceCYMK [4]float64

// DeviceCYMKNew returns a new DeviceCMYK color.
func DeviceCYMKNew(cyan, magenta, yellow, black float64) Color {
	return colorDeviceCYMK{cyan, magenta, yellow, black}
}

// setStroke sets the current stroking color space to DeviceCMYK
// and sets the cyan, magenta, yellow, and black levels for stroking.
//
// This implements the PDF graphics operator "K".
func (c colorDeviceCYMK) setStroke(w *Writer) {
	cur, ok := w.StrokeColor.(colorDeviceCYMK)
	if ok && w.isSet(StateStrokeColor) && cur == c {
		return
	}

	w.StrokeColor = c
	w.Set |= StateStrokeColor

	// TODO(voss): rounding
	_, w.Err = fmt.Fprintf(w.Content, "%f %f %f %f K\n", c[0], c[1], c[2], c[3])
}

// setFill sets the current fill color space to DeviceCMYK
// and sets the cyan, magenta, yellow, and black levels for filling.
//
// This implements the PDF graphics operator "k".
func (c colorDeviceCYMK) setFill(w *Writer) {
	cur, ok := w.FillColor.(colorDeviceCYMK)
	if ok && w.isSet(StateFillColor) && cur == c {
		return
	}

	w.FillColor = c
	w.Set |= StateFillColor

	// TODO(voss): rounding
	_, w.Err = fmt.Fprintf(w.Content, "%f %f %f %f k\n", c[0], c[1], c[2], c[3])
}

// == CalGray ================================================================

// ColorSpaceCalGray represents a CalGray color space.
type ColorSpaceCalGray struct {
	pdf.Res
	whitePoint []float64
	blackPoint []float64
	gamma      float64
}

// CalGray returns a new CalGray color space.
//
// The whitePoint and blackPoint parameters are arrays of three numbers
// each, representing the X, Y, and Z components of the points.
// The Y-value of the white point must equal 1.0.
//
// blackPoint is optional, the default value is [0 0 0].
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

// Embed embeds the color space in the PDF file.
// This saves space in case the color space is used in multiple content streams.
func (c *ColorSpaceCalGray) Embed(out *pdf.Writer) (*ColorSpaceCalGray, error) {
	if _, ok := c.Res.Ref.(pdf.Reference); ok {
		return c, nil
	}
	ref := out.Alloc()
	err := out.Put(ref, c.Res.Ref)
	if err != nil {
		return nil, err
	}

	res := clone(c)
	res.Res.Ref = ref
	return res, nil
}

// New returns a new CalGray color.
func (c *ColorSpaceCalGray) New(gray float64) Color {
	return colorCalGray{Space: c, Value: gray}
}

type colorCalGray struct {
	Space *ColorSpaceCalGray
	Value float64
}

func (c colorCalGray) setStroke(w *Writer) {
	if w.Version < pdf.V1_1 {
		w.Err = &pdf.VersionError{Operation: "CalGray colors", Earliest: pdf.V1_1}
		return
	}

	cur, ok := w.StrokeColor.(colorCalGray)
	// First set the color space, if needed.
	if !ok || !w.isSet(StateStrokeColor) || cur.Space != c.Space {
		name := w.getResourceName(catColorSpace, c.Space)
		w.Err = name.PDF(w.Content)
		if w.Err != nil {
			return
		}
		_, w.Err = fmt.Fprintln(w.Content, " CS")
		if w.Err != nil {
			return
		}

		cur.Value = 0
	}

	// Then set the color value.
	if cur.Value != c.Value {
		// TODO(voss): rounding
		_, w.Err = fmt.Fprintf(w.Content, "%f SC\n", c.Value)
		if w.Err != nil {
			return
		}
	}

	w.StrokeColor = c
	w.State.Set |= StateStrokeColor
}

func (c colorCalGray) setFill(w *Writer) {
	if w.Version < pdf.V1_1 {
		w.Err = &pdf.VersionError{Operation: "CalGray colors", Earliest: pdf.V1_1}
		return
	}

	cur, ok := w.FillColor.(colorCalGray)
	// First set the color space, if needed.
	if !ok || !w.isSet(StateFillColor) || cur.Space != c.Space {
		name := w.getResourceName(catColorSpace, c.Space)
		w.Err = name.PDF(w.Content)
		if w.Err != nil {
			return
		}
		_, w.Err = fmt.Fprintln(w.Content, " cs")
		if w.Err != nil {
			return
		}

		cur.Value = 0
	}

	// Then set the color value.
	if cur.Value != c.Value {
		// TODO(voss): rounding
		_, w.Err = fmt.Fprintf(w.Content, "%f sc\n", c.Value)
		if w.Err != nil {
			return
		}
	}

	w.FillColor = c
	w.State.Set |= StateFillColor
}

// == CalRGB =================================================================

// ColorSpaceCalRGB represents a CalRGB color space.
type ColorSpaceCalRGB struct {
	pdf.Res
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

// Embed embeds the color space in the PDF file.
// This saves space in case the color space is used in multiple content streams.
func (c *ColorSpaceCalRGB) Embed(out *pdf.Writer) (*ColorSpaceCalRGB, error) {
	if _, ok := c.Res.Ref.(pdf.Reference); ok {
		return c, nil
	}
	ref := out.Alloc()
	err := out.Put(ref, c.Res.Ref)
	if err != nil {
		return nil, err
	}

	res := clone(c)
	res.Res.Ref = ref
	return res, nil
}

// New returns a new CalRGB color.
func (c *ColorSpaceCalRGB) New(r, g, b float64) Color {
	return colorCalRGB{Space: c, R: r, G: g, B: b}
}

type colorCalRGB struct {
	Space   *ColorSpaceCalRGB
	R, G, B float64
}

func (c colorCalRGB) setStroke(w *Writer) {
	if w.Version < pdf.V1_1 {
		w.Err = &pdf.VersionError{Operation: "CalRGB colors", Earliest: pdf.V1_1}
		return
	}

	cur, ok := w.StrokeColor.(colorCalRGB)
	// First set the color space, if needed.
	if !ok || !w.isSet(StateStrokeColor) || cur.Space != c.Space {
		name := w.getResourceName(catColorSpace, c.Space)
		w.Err = name.PDF(w.Content)
		if w.Err != nil {
			return
		}
		_, w.Err = fmt.Fprintln(w.Content, " CS")
		if w.Err != nil {
			return
		}

		cur.R = 0
		cur.G = 0
		cur.B = 0
	}

	// Then set the color value.
	if cur.R != c.R || cur.G != c.G || cur.B != c.B {
		rString := float.Format(c.R, 3)
		gString := float.Format(c.G, 3)
		bString := float.Format(c.B, 3)
		_, w.Err = fmt.Fprintln(w.Content, rString, gString, bString, "SC")
		if w.Err != nil {
			return
		}
	}

	w.StrokeColor = c
	w.State.Set |= StateStrokeColor
}

func (c colorCalRGB) setFill(w *Writer) {
	if w.Version < pdf.V1_1 {
		w.Err = &pdf.VersionError{Operation: "CalRGB colors", Earliest: pdf.V1_1}
		return
	}

	cur, ok := w.FillColor.(colorCalRGB)
	// First set the color space, if needed.
	if !ok || !w.isSet(StateFillColor) || cur.Space != c.Space {
		name := w.getResourceName(catColorSpace, c.Space)
		w.Err = name.PDF(w.Content)
		if w.Err != nil {
			return
		}
		_, w.Err = fmt.Fprintln(w.Content, " cs")
		if w.Err != nil {
			return
		}

		cur.R = 0
		cur.G = 0
		cur.B = 0
	}

	// Then set the color value.
	if cur.R != c.R || cur.G != c.G || cur.B != c.B {
		rString := float.Format(c.R, 3)
		gString := float.Format(c.G, 3)
		bString := float.Format(c.B, 3)
		_, w.Err = fmt.Fprintln(w.Content, rString, gString, bString, "sc")
		if w.Err != nil {
			return
		}
	}

	w.FillColor = c
	w.State.Set |= StateFillColor
}

// == ... =================================================================

type colorSpaceLab struct {
	pdf.Res
}

func (c *colorSpaceLab) defaultColor() []float64 {
	panic("not implemented")
}

type colorSpaceICCBased struct {
	pdf.Res
}

func (c *colorSpaceICCBased) defaultColor() []float64 {
	panic("not implemented")
}

type colorSpaceIndexed struct {
	pdf.Res
}

func (c *colorSpaceIndexed) defaultColor() []float64 {
	panic("not implemented")
}

type colorSpacePattern struct {
	pdf.Res
}

func (c *colorSpacePattern) defaultColor() []float64 {
	panic("not implemented")
}

type colorSpaceSeparation struct {
	pdf.Res
}

func (c *colorSpaceSeparation) defaultColor() []float64 {
	panic("not implemented")
}

type colorSpaceDeviceN struct {
	pdf.Res
}

func (c *colorSpaceDeviceN) defaultColor() []float64 {
	panic("not implemented")
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

func clone[T any](x *T) *T {
	y := *x
	return &y
}

// WhitePointD65 represents the D65 whitepoint.
//
// https://en.wikipedia.org/wiki/Illuminant_D65
var WhitePointD65 = []float64{0.95047, 1.0, 1.08883}
