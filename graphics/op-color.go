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
	"fmt"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/float"
)

// This file implements color-related PDF operators.
// The operators implemented here are defined in table 73 of
// ISO 32000-2:2020.

// SetStrokeColorSpace sets the current color space for nonstroking operations.
//
// This implements the PDF graphics operator "CS".
func (w *Writer) SetStrokeColorSpace(cs ColorSpace) {
	if !w.isValid("SetStrokeColorSpace", objPage|objText) {
		return
	}
	if w.Version < pdf.V1_1 {
		w.Err = &pdf.VersionError{Operation: "SetStrokeColorSpace", Earliest: pdf.V1_1}
		return
	}
	csFam := cs.Family()
	if minVersion, ok := colMinVersion[csFam]; ok && w.Version < minVersion {
		w.Err = &pdf.VersionError{Operation: csFam.String() + " color space", Earliest: minVersion}
		return
	}

	val := cs.PDFObject()

	var name pdf.Name
	if n, isName := val.(pdf.Name); isName {
		name = n
	} else {
		name = w.getResourceName(catColorSpace, cs)
	}

	if w.isSet(StateStrokeColor) && w.StrokeColorSpace == cs {
		return
	}
	w.StrokeColorSpace = cs
	w.StrokeColor = cs.DefaultColor()
	w.Set |= StateStrokeColor

	err := name.PDF(w.Content)
	if err != nil {
		w.Err = err
		return
	}
	_, w.Err = fmt.Fprintln(w.Content, " CS")
}

// SetFillColorSpace sets the current color space for nonstroking operations.
//
// This implements the PDF graphics operator "cs".
func (w *Writer) SetFillColorSpace(cs ColorSpace) {
	if !w.isValid("SetFillColorSpace", objPage|objText) {
		return
	}
	if w.Version < pdf.V1_1 {
		w.Err = &pdf.VersionError{Operation: "SetFillColorSpace", Earliest: pdf.V1_1}
		return
	}
	csFam := cs.Family()
	if minVersion, ok := colMinVersion[csFam]; ok && w.Version < minVersion {
		w.Err = &pdf.VersionError{Operation: csFam.String() + " color space", Earliest: minVersion}
		return
	}

	val := cs.PDFObject()

	var name pdf.Name
	if n, isName := val.(pdf.Name); isName {
		name = n
	} else {
		name = w.getResourceName(catColorSpace, cs)
	}

	if w.isSet(StateFillColor) && w.FillColorSpace == cs {
		return
	}
	w.FillColorSpace = cs
	w.FillColor = cs.DefaultColor()
	w.Set |= StateFillColor

	err := name.PDF(w.Content)
	if err != nil {
		w.Err = err
		return
	}
	_, w.Err = fmt.Fprintln(w.Content, " cs")
}

// SetStrokeColorValues sets the the color within the current color space.
//
// This implements the PDF graphics operators "SCN" (for Pattern, Separation,
// DeviceN and ICCBased color spaces) and "SC" (for all other color spaces).
func (w *Writer) SetStrokeColorValues(values []float64) {
	if !w.isValid("SetStrokeColorValues", objPage|objText) {
		return
	}
	if w.Version < pdf.V1_1 {
		w.Err = &pdf.VersionError{Operation: "SetStrokeColorValues", Earliest: pdf.V1_1}
		return
	}

	n := len(w.StrokeColorSpace.DefaultColor())
	if len(values) != n {
		w.Err = fmt.Errorf("%s: expected %d components, got %d",
			w.StrokeColorSpace.Family(), n, len(values))
	}

	if !w.isSet(StateStrokeColor) {
		w.Err = fmt.Errorf("SetStrokeColorValues: no color space set")
		return
	}
	if isEqual(w.StrokeColor, values) {
		return
	}

	w.StrokeColor = slices.Clone(values)
	w.Set |= StateStrokeColor

	for _, v := range values {
		_, w.Err = fmt.Fprint(w.Content, float.Format(v, 5), " ")
		if w.Err != nil {
			return
		}
	}
	if colNeedsScn[w.StrokeColorSpace.Family()] {
		_, w.Err = fmt.Fprintln(w.Content, "SCN")
	} else {
		_, w.Err = fmt.Fprintln(w.Content, "SC")
	}
}

// SetFillColorValues sets the the color within the current color space.
//
// This implements the PDF graphics operators "scn" (for Pattern, Separation,
// DeviceN and ICCBased color spaces) and "sc" (for all other color spaces).
func (w *Writer) SetFillColorValues(values []float64) {
	if !w.isValid("SetFillColorValues", objPage|objText) {
		return
	}
	if w.Version < pdf.V1_1 {
		w.Err = &pdf.VersionError{Operation: "SetFillColorValues", Earliest: pdf.V1_1}
		return
	}

	n := len(w.FillColorSpace.DefaultColor())
	if len(values) != n {
		w.Err = fmt.Errorf("%s: expected %d components, got %d",
			w.FillColorSpace.Family(), n, len(values))
	}

	if !w.isSet(StateFillColor) {
		w.Err = fmt.Errorf("SetFillColorValues: no color space set")
		return
	}
	if isEqual(w.FillColor, values) {
		return
	}

	w.FillColor = slices.Clone(values)

	for _, v := range values {
		_, w.Err = fmt.Fprint(w.Content, float.Format(v, 5), " ")
		if w.Err != nil {
			return
		}
	}
	if colNeedsScn[w.FillColorSpace.Family()] {
		_, w.Err = fmt.Fprintln(w.Content, "scn")
	} else {
		_, w.Err = fmt.Fprintln(w.Content, "sc")
	}
}

// SetStrokeColorValuesName sets the the color within the current color space.
//
// This implements the PDF graphics operator "SCN".
func (w *Writer) SetStrokeColorValuesName(values []float64, name pdf.Name) {
	if !w.isValid("SetStrokeColorValuesName", objPage|objText) {
		return
	}
	if w.Version < pdf.V1_2 {
		w.Err = &pdf.VersionError{Operation: "SetStrokeColorValuesName", Earliest: pdf.V1_2}
		return
	}
	panic("not implemented")
}

// SetFillColorValuesName sets the the color within the current color space.
//
// This implements the PDF graphics operator "SCN".
func (w *Writer) SetFillColorValuesName(values []float64, name pdf.Name) {
	if !w.isValid("SetFillColorValuesName", objPage|objText) {
		return
	}
	if w.Version < pdf.V1_2 {
		w.Err = &pdf.VersionError{Operation: "SetFillColorValuesName", Earliest: pdf.V1_2}
		return
	}
	panic("not implemented")
}

// SetStrokeColorDeviceGray sets the current stroking color space to DeviceGray
// and sets the gray level for stroking operations.
//
// This implements the PDF graphics operator "G".
func (w *Writer) SetStrokeColorDeviceGray(gray float64) {
	if !w.isValid("SetStrokeColorDeviceGray", objPage|objText) {
		return
	}
	if gray < 0 || gray > 1 {
		w.Err = fmt.Errorf("SetStrokeColorDeviceGray: expected value in [0, 1], got %f", gray)
		return
	}

	if w.isSet(StateStrokeColor) && w.StrokeColorSpace.Family() == ColorSpaceDeviceGray &&
		w.StrokeColor[0] == gray {
		return
	}

	w.StrokeColorSpace = DeviceGray
	w.StrokeColor = []float64{gray}
	w.Set |= StateStrokeColor

	// TODO(voss): rounding
	_, w.Err = fmt.Fprintf(w.Content, "%f G\n", gray)
}

// SetFillColorDeviceGray sets the current stroking color space to DeviceGray
// and sets the gray level for stroking operations.
//
// This implements the PDF graphics operator "g".
func (w *Writer) SetFillColorDeviceGray(gray float64) {
	if !w.isValid("SetFillColorDeviceGray", objPage|objText) {
		return
	}
	if gray < 0 || gray > 1 {
		w.Err = fmt.Errorf("SetFillColorDeviceGray: expected value in [0, 1], got %f", gray)
		return
	}

	if w.isSet(StateFillColor) && w.FillColorSpace.Family() == ColorSpaceDeviceGray &&
		w.FillColor[0] == gray {
		return
	}

	w.FillColorSpace = DeviceGray
	w.FillColor = []float64{gray}
	w.Set |= StateFillColor

	// TODO(voss): rounding
	_, w.Err = fmt.Fprintf(w.Content, "%f g\n", gray)
}

// SetStrokeColorDeviceRGB sets the current stroking color space to DeviceRGB
// and sets the color for stroking operations.
//
// This implements the PDF graphics operator "RG".
func (w *Writer) SetStrokeColorDeviceRGB(r, g, b float64) {
	if !w.isValid("SetStrokeColorDeviceRGB", objPage|objText) {
		return
	}
	if r < 0 || r > 1 || g < 0 || g > 1 || b < 0 || b > 1 {
		w.Err = fmt.Errorf("SetStrokeColorDeviceRGB: expected values in [0, 1], got %f, %f, %f", r, g, b)
		return
	}

	if w.isSet(StateStrokeColor) && w.StrokeColorSpace.Family() == ColorSpaceDeviceRGB &&
		w.StrokeColor[0] == r && w.StrokeColor[1] == g && w.StrokeColor[2] == b {
		return
	}

	w.StrokeColorSpace = DeviceRGB
	w.StrokeColor = []float64{r, g, b}
	w.Set |= StateStrokeColor

	// TODO(voss): rounding
	_, w.Err = fmt.Fprintf(w.Content, "%f %f %f RG\n", r, g, b)
}

// SetFillColorDeviceRGB sets the current stroking color space to DeviceRGB
// and sets the color for stroking operations.
//
// This implements the PDF graphics operator "rg".
func (w *Writer) SetFillColorDeviceRGB(r, g, b float64) {
	if !w.isValid("SetFillColorDeviceRGB", objPage|objText) {
		return
	}
	if r < 0 || r > 1 || g < 0 || g > 1 || b < 0 || b > 1 {
		w.Err = fmt.Errorf("SetFillColorDeviceRGB: expected values in [0, 1], got %f, %f, %f", r, g, b)
		return
	}

	if w.isSet(StateFillColor) && w.FillColorSpace.Family() == ColorSpaceDeviceRGB &&
		w.FillColor[0] == r && w.FillColor[1] == g && w.FillColor[2] == b {
		return
	}

	w.FillColorSpace = DeviceRGB
	w.FillColor = []float64{r, g, b}
	w.Set |= StateFillColor

	// TODO(voss): rounding
	_, w.Err = fmt.Fprintf(w.Content, "%f %f %f rg\n", r, g, b)
}

// SetStrokeColorDeviceCMYK sets the current stroking color space to DeviceCMYK
// and sets the color for stroking operations.
//
// This implements the PDF graphics operator "K".
func (w *Writer) SetStrokeColorDeviceCMYK(c, m, y, k float64) {
	if !w.isValid("SetStrokeColorDeviceCMYK", objPage|objText) {
		return
	}
	if c < 0 || c > 1 || m < 0 || m > 1 || y < 0 || y > 1 || k < 0 || k > 1 {
		w.Err = fmt.Errorf("SetStrokeColorDeviceCMYK: expected values in [0, 1], got %f, %f, %f, %f", c, m, y, k)
		return
	}

	if w.isSet(StateStrokeColor) && w.StrokeColorSpace.Family() == ColorSpaceDeviceCMYK &&
		w.StrokeColor[0] == c && w.StrokeColor[1] == m && w.StrokeColor[2] == y && w.StrokeColor[3] == k {
		return
	}

	w.StrokeColorSpace = DeviceCMYK
	w.StrokeColor = []float64{c, m, y, k}
	w.Set |= StateStrokeColor

	// TODO(voss): rounding
	_, w.Err = fmt.Fprintf(w.Content, "%f %f %f %f K\n", c, m, y, k)
}

// SetFillColorDeviceCMYK sets the current stroking color space to DeviceCMYK
// and sets the color for stroking operations.
//
// This implements the PDF graphics operator "k".
func (w *Writer) SetFillColorDeviceCMYK(c, m, y, k float64) {
	if !w.isValid("SetFillColorDeviceCMYK", objPage|objText) {
		return
	}
	if c < 0 || c > 1 || m < 0 || m > 1 || y < 0 || y > 1 || k < 0 || k > 1 {
		w.Err = fmt.Errorf("SetFillColorDeviceCMYK: expected values in [0, 1], got %f, %f, %f, %f", c, m, y, k)
		return
	}

	if w.isSet(StateFillColor) && w.FillColorSpace.Family() == ColorSpaceDeviceCMYK &&
		w.FillColor[0] == c && w.FillColor[1] == m && w.FillColor[2] == y && w.FillColor[3] == k {
		return
	}

	w.FillColorSpace = DeviceCMYK
	w.FillColor = []float64{c, m, y, k}
	w.Set |= StateFillColor

	// TODO(voss): rounding
	_, w.Err = fmt.Fprintf(w.Content, "%f %f %f %f k\n", c, m, y, k)
}
