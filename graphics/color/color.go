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
	"math"
)

// Color represents a PDF color.
type Color interface {
	ColorSpace() Space
}

// SCN returns a new color in the same color space as c, but with the given
// component values and pattern (if c is from a pattern color space).
func SCN(c Color, values []float64, pat Pattern) Color {
	switch d := c.(type) {
	case colorDeviceGray:
		if len(values) >= 1 {
			return colorDeviceGray(values[0])
		}
		return d
	case colorDeviceRGB:
		copy(d[:], values)
		return d
	case colorDeviceCMYK:
		copy(d[:], values)
		return d
	case colorCalGray:
		if len(values) >= 1 {
			d.Value = values[0]
		}
		return d
	case colorCalRGB:
		copy(d.Values[:], values)
		return d
	case colorLab:
		copy(d.Values[:], values)
		return d
	case colorICCBased:
		copy(d.Values[:], values)
		return d
	case colorSRGB:
		copy(d[:], values)
		return d
	case colorColoredPattern:
		d.Pat = pat
		return d
	case colorUncoloredPattern:
		d.Col = SCN(d.Col, values, nil)
		d.Pat = pat
		return d
	case colorIndexed:
		if len(values) >= 1 {
			d.Index = int(math.Round(values[0]))
		}
		return d
	case colorSeparation:
		if len(values) >= 1 {
			d.Tint = values[0]
		}
		return d
	case colorDeviceN:
		n := d.Space.Channels()
		if len(values) >= n {
			d.set(values[:n])
		}
		return d
	default:
		panic(fmt.Sprintf("unknown color type %T", d))
	}
}

// Operator returns the color values, the pattern resource, and the operator
// name for the given color.  The operator name is for stroking operations. The
// corresponding operator for filling operations is the operator name converted
// to lower case.
func Operator(c Color) ([]float64, Pattern, string) {
	v := values(c)
	switch c := c.(type) {
	case colorDeviceGray:
		return v, nil, "G"
	case colorDeviceRGB:
		return v, nil, "RG"
	case colorDeviceCMYK:
		return v, nil, "K"
	case colorCalGray:
		return v, nil, "SC"
	case colorCalRGB:
		return v, nil, "SC"
	case colorLab:
		return v, nil, "SC"
	case colorICCBased:
		return v, nil, "SCN"
	case colorSRGB:
		return v, nil, "SCN"
	case colorColoredPattern:
		return v, c.Pat, "SCN"
	case *colorUncoloredPattern:
		return v, c.Pat, "SCN"
	case colorIndexed:
		return v, nil, "SC"
	case colorSeparation:
		return v, nil, "SCN"
	case colorDeviceN:
		return v, nil, "SCN"
	default:
		panic(fmt.Sprintf("unknown color type %T", c))
	}
}

func values(c Color) []float64 {
	switch c := c.(type) {
	case colorDeviceGray:
		return []float64{float64(c)}
	case colorDeviceRGB:
		return c[:]
	case colorDeviceCMYK:
		return c[:]
	case colorCalGray:
		return []float64{c.Value}
	case colorCalRGB:
		return c.Values[:]
	case colorLab:
		return c.Values[:]
	case colorICCBased:
		return c.Values[:c.Space.N]
	case colorSRGB:
		return c[:]
	case *colorUncoloredPattern:
		return values(c)
	case colorIndexed:
		return []float64{float64(c.Index)}
	case colorSeparation:
		return []float64{c.Tint}
	case colorDeviceN:
		return c.get()
	default:
		return nil
	}
}

// Some commonly used white points.
// These vectors can be used for the white point argument of the
// [CalGray], [CalRGB], and [Lab] functions.
var (
	// WhitePointD50 represents the D50 whitepoint.
	// This is often used in the printing industry.
	// The given values are CIE 1931 XYZ coordinates.
	//
	// https://en.wikipedia.org/wiki/Standard_illuminant#Illuminant_series_D
	WhitePointD50 = []float64{0.964212, 1.0, 0.8251883}

	// WhitePointD65 represents the D65 whitepoint.
	// This is often used in the computer industry.
	// The given values are CIE 1931 XYZ coordinates.
	//
	// https://en.wikipedia.org/wiki/Illuminant_D65
	WhitePointD65 = []float64{0.95047, 1.0, 1.08883}
)
