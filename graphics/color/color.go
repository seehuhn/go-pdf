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
	"fmt"

	"seehuhn.de/go/pdf"
)

// Color space families supported by PDF.
const (
	FamilyDeviceGray pdf.Name = "DeviceGray"
	FamilyDeviceRGB  pdf.Name = "DeviceRGB"
	FamilyDeviceCMYK pdf.Name = "DeviceCMYK"
	FamilyCalGray    pdf.Name = "CalGray"
	FamilyCalRGB     pdf.Name = "CalRGB"
	FamilyLab        pdf.Name = "Lab"
	FamilyICCBased   pdf.Name = "ICCBased"
	FamilyPattern    pdf.Name = "Pattern"
	FamilyIndexed    pdf.Name = "Indexed"
	FamilySeparation pdf.Name = "Separation"
	FamilyDeviceN    pdf.Name = "DeviceN"
)

// Space represents a PDF color space.
type Space interface {
	pdf.Resource

	ColorSpaceFamily() pdf.Name

	defaultColor() Color
}

// NumValues returns the number of color values for the given color space.
func NumValues(s Space) int {
	return len(s.defaultColor().values())
}

// IsPattern returns whether the given color space is a pattern color space.
func IsPattern(s Space) bool {
	switch s.(type) {
	case spacePatternColored, spacePatternUncolored:
		return true
	}
	return false
}

// IsIndexed returns whether the given color space is an indexed color space.
func IsIndexed(s Space) bool {
	switch s.(type) {
	case *SpaceIndexed:
		return true
	}
	return false
}

// Color represents a PDF color.
type Color interface {
	ColorSpace() Space
	values() []float64
}

// CheckVersion checks whether the given color space can be used in the given
// PDF version.
//
// See table 61 in ISO 32000-2:2020.
func CheckVersion(cs Space, v pdf.Version) error {
	fam := cs.ColorSpaceFamily()
	minVersion := pdf.V1_0
	switch fam {
	case FamilyDeviceGray, FamilyDeviceRGB, FamilyDeviceCMYK:
		// The concept of color spaces was only introduced in PDF 1.1.
		// We use pdf.V1_0 here to allow for use of the "G", "RG", and "K"
		// operators in PDF 1.0.
		minVersion = pdf.V1_0
	case FamilyCalGray, FamilyCalRGB, FamilyLab, FamilyIndexed:
		minVersion = pdf.V1_1
	case FamilyPattern, FamilySeparation:
		minVersion = pdf.V1_2
	case FamilyICCBased, FamilyDeviceN:
		minVersion = pdf.V1_3
	}
	if v < minVersion {
		return &pdf.VersionError{
			Operation: string(fam) + " colors",
			Earliest:  minVersion,
		}
	}
	return nil
}

// CheckCurrent checks whether the changing from the current color to the new
// color requires a color space change and/or a color change.
func CheckCurrent(cur, new Color) (needsColorSpace bool, needsColor bool) {
	needsColorSpace = false
	if cs := new.ColorSpace(); cs != DeviceGray && cs != DeviceRGB && cs != DeviceCMYK {
		if cur == nil || cur.ColorSpace() != cs {
			needsColorSpace = true
			cur = cs.defaultColor()
		}
	}

	return needsColorSpace, cur != new
}

// Operator returns the color values, the pattern resource, and the operator
// name for the given color.  The operator name is for stroking operations. The
// corresponding operator for filling operations is the operator name converted
// to lower case.
func Operator(c Color) ([]float64, pdf.Resource, string) {
	switch c := c.(type) {
	case colorDeviceGray:
		return c.values(), nil, "G"
	case colorDeviceRGB:
		return c.values(), nil, "RG"
	case colorDeviceCMYK:
		return c.values(), nil, "K"
	case colorCalGray, colorCalRGB, colorLab:
		return c.values(), nil, "SC"
	case colorIndexed:
		return c.values(), nil, "SC"
	case *PatternColored:
		return nil, c.Res, "SCN"
	case *colorPatternUncolored:
		return c.values(), c.Res, "SCN"
	default:
		panic(fmt.Sprintf("unknown color type %T", c))
	}
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

var (
	// WhitePointD65 represents the D65 whitepoint.
	// The given values are CIE 1931 XYZ coordinates.
	//
	// https://en.wikipedia.org/wiki/Illuminant_D65
	WhitePointD65 = []float64{0.95047, 1.0, 1.08883}

	// WhitePointD50 represents the D50 whitepoint.
	// The given values are CIE 1931 XYZ coordinates.
	//
	// https://en.wikipedia.org/wiki/Standard_illuminant#Illuminant_series_D
	WhitePointD50 = []float64{0.964212, 1.0, 0.8251883}
)
