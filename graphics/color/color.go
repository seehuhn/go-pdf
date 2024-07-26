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
	"slices"

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

// Space represents a PDF color space which can be embedded in a PDF file.
type Space interface {
	Embed(*pdf.ResourceManager) (pdf.Object, pdf.Unused, error)
	ColorSpaceFamily() pdf.Name
	defaultValues() []float64
}

// NumValues returns the number of color values for the given color space.
func NumValues(s Space) int {
	return len(s.defaultValues())
}

// Color represents a PDF color.
type Color interface {
	ColorSpace() Space
	values() []float64
}

// Pattern represents a PDF pattern dictionary.
type Pattern interface {
	// IsColored returns true if the pattern is colored.
	// This is the case for colored tiling patterns and shading patterns.
	IsColored() bool

	// Embed embeds the pattern in the PDF file.
	Embed(*pdf.ResourceManager) (pdf.Object, pdf.Unused, error)
}

// CheckCurrent checks whether the changing from the current color to the new
// color requires a color space change and/or a color change.
func CheckCurrent(cur, new Color) (needsColorSpace bool, needsColor bool) {
	var curCS Space
	if cur != nil {
		curCS = cur.ColorSpace()
	}
	newCS := new.ColorSpace()

	var currentPattern Pattern
	switch cur := cur.(type) {
	case colorColoredPattern:
		currentPattern = cur.Pat
	case *colorUncoloredPattern:
		currentPattern = cur.Pat
	}

	var currentValues []float64
	if curCS != newCS {
		needsColorSpace = true
		currentPattern = nil
		currentValues = newCS.defaultValues()
	} else {
		currentValues = cur.values()
	}

	switch new := new.(type) {
	case colorDeviceGray, colorDeviceRGB, colorDeviceCMYK:
		// We use the "g", "rg" and "k" operators without setting the
		// color space separately.
		return false, needsColorSpace || !slices.Equal(currentValues, new.values())
	case colorColoredPattern:
		return needsColorSpace, currentPattern != new.Pat
	case *colorUncoloredPattern:
		return needsColorSpace, currentPattern != new.Pat || !slices.Equal(currentValues, new.values())
	default:
		return needsColorSpace, !slices.Equal(currentValues, new.values())
	}
}

// Operator returns the color values, the pattern resource, and the operator
// name for the given color.  The operator name is for stroking operations. The
// corresponding operator for filling operations is the operator name converted
// to lower case.
func Operator(c Color) ([]float64, Pattern, string) {
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
	case colorColoredPattern:
		return nil, c.Pat, "SCN"
	case *colorUncoloredPattern:
		return c.values(), c.Pat, "SCN"
	default:
		panic(fmt.Sprintf("unknown color type %T", c))
	}
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
