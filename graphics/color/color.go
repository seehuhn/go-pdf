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
	gocolor "image/color"
	"math"
)

// Color represents a PDF color.
//
// This interface extends the standard library's [gocolor.Color] interface,
// allowing to use PDF colors wherever Go colors are expected.
type Color interface {
	ColorSpace() Space

	// ToXYZ returns the colour as CIE XYZ tristimulus values
	// adapted to the D50 illuminant (the ICC Profile Connection Space).
	ToXYZ() (X, Y, Z float64)

	gocolor.Color
}

// FromValues returns a new color in the given color space with the specified
// component values and pattern (if cs is a pattern color space).
func FromValues(cs Space, values []float64, pat Pattern) Color {
	switch cs := cs.(type) {
	case spaceDeviceGray:
		if len(values) >= 1 {
			return DeviceGray(values[0])
		}
		return cs.Default()
	case spaceDeviceRGB:
		var c DeviceRGB
		copy(c[:], values)
		return c
	case spaceDeviceCMYK:
		var c DeviceCMYK
		copy(c[:], values)
		return c
	case *SpaceCalGray:
		if len(values) >= 1 {
			return cs.New(values[0])
		}
		return cs.Default()
	case *SpaceCalRGB:
		if len(values) >= 3 {
			return cs.New(values[0], values[1], values[2])
		}
		return cs.Default()
	case *SpaceLab:
		if len(values) >= 3 {
			c, _ := cs.New(values[0], values[1], values[2])
			if c != nil {
				return c
			}
		}
		return cs.Default()
	case *SpaceICCBased:
		d := colorICCBased{Space: cs}
		copy(d.Values[:], values)
		return d
	case spaceSRGB:
		var c colorSRGB
		copy(c[:], values)
		return c
	case spacePatternColored:
		return colorColoredPattern{Pat: pat}
	case spacePatternUncolored:
		col := FromValues(cs.base, values, nil)
		return colorUncoloredPattern{Col: col, Pat: pat}
	case *SpaceIndexed:
		if len(values) >= 1 {
			return cs.New(int(math.Round(values[0])))
		}
		return cs.Default()
	case *SpaceSeparation:
		if len(values) >= 1 {
			return cs.New(values[0])
		}
		return cs.Default()
	case *SpaceDeviceN:
		n := cs.Channels()
		if len(values) >= n {
			return cs.New(values[:n])
		}
		return cs.Default()
	default:
		panic(fmt.Sprintf("unknown color space type %T", cs))
	}
}

// Values returns the color component values and the pattern resource (if any)
// for the given color. The returned slice must not be modified.
func Values(c Color) ([]float64, Pattern) {
	switch c := c.(type) {
	case DeviceGray:
		return []float64{float64(c)}, nil
	case DeviceRGB:
		return c[:], nil
	case DeviceCMYK:
		return c[:], nil
	case colorCalGray:
		return []float64{c.Value}, nil
	case colorCalRGB:
		return c.Values[:], nil
	case colorLab:
		return c.Values[:], nil
	case colorICCBased:
		return c.Values[:c.Space.N], nil
	case colorSRGB:
		return c[:], nil
	case colorColoredPattern:
		return nil, c.Pat
	case colorUncoloredPattern:
		v, _ := Values(c.Col)
		return v, c.Pat
	case colorIndexed:
		return []float64{float64(c.Index)}, nil
	case colorSeparation:
		return []float64{c.Tint}, nil
	case colorDeviceN:
		return c.get(), nil
	default:
		return nil, nil
	}
}

// Operator returns the PDF operator name for setting the given color.
// The operator name is for stroking operations. The corresponding operator
// for filling operations is the operator name converted to lower case.
func Operator(c Color) string {
	switch c.(type) {
	case DeviceGray:
		return "G"
	case DeviceRGB:
		return "RG"
	case DeviceCMYK:
		return "K"
	case colorCalGray:
		return "SC"
	case colorCalRGB:
		return "SC"
	case colorLab:
		return "SC"
	case colorICCBased:
		return "SCN"
	case colorSRGB:
		return "SCN"
	case colorColoredPattern:
		return "SCN"
	case colorUncoloredPattern:
		return "SCN"
	case colorIndexed:
		return "SC"
	case colorSeparation:
		return "SCN"
	case colorDeviceN:
		return "SCN"
	default:
		panic(fmt.Sprintf("unknown color type %T", c))
	}
}

var (
	// Black represents the black color in the DeviceGray color space.
	Black = DeviceGray(0)

	// White represents the white color in the DeviceGray color space.
	White = DeviceGray(1)

	// Red represents the red color in the DeviceRGB color space.
	Red = DeviceRGB{1, 0, 0}

	// Green represents the green color in the DeviceRGB color space.
	Green = DeviceRGB{0, 1, 0}

	// Blue represents the blue color in the DeviceRGB color space.
	Blue = DeviceRGB{0, 0, 1}

	// Cyan represents the cyan color in the DeviceCMYK color space.
	Cyan = DeviceCMYK{1, 0, 0, 0}

	// Magenta represents the magenta color in the DeviceCMYK color space.
	Magenta = DeviceCMYK{0, 1, 0, 0}

	// Yellow represents the yellow color in the DeviceCMYK color space.
	Yellow = DeviceCMYK{0, 0, 1, 0}
)

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
