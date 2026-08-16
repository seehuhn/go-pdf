// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package fallback

import (
	"seehuhn.de/go/icc"
	"seehuhn.de/go/pdf/graphics/color"
)

// Lightness adjustment for the shades of a 3-D border.  A colour is converted
// to CIE L*a*b*, its L* component is moved, and it is converted back, which
// changes how light the colour is while leaving its hue and chroma alone.

// labSpace is the L*a*b* space the adjustment runs in.  Its white point is the
// one every conversion in [color] is relative to, so converting through it
// involves no chromatic adaptation.  The ranges are wide enough to hold the a*
// and b* values of any device colour.
var labSpace = newLabSpace()

func newLabSpace() *color.SpaceLab {
	s, err := color.Lab(icc.PCSWhitePoint[:], nil, []float64{-128, 127, -128, 127})
	if err != nil {
		panic(err) // the parameters are constants
	}
	return s
}

// rgbToLab converts a DeviceRGB colour to L*a*b*.
func rgbToLab(r, g, b float64) (L, A, B float64) {
	ws := &icc.Workspace{}
	X, Y, Z := color.SpaceDeviceRGB.ToXYZ([]float64{r, g, b}, ws)
	var lab [3]float64
	labSpace.FromXYZ(X, Y, Z, lab[:], ws)
	return lab[0], lab[1], lab[2]
}

// labToRGB is the inverse of [rgbToLab].  The components are already within
// [0, 1], because converting out of XYZ clamps them to the range DeviceRGB
// allows.
func labToRGB(L, A, B float64) (r, g, b float64) {
	ws := &icc.Workspace{}
	X, Y, Z := labSpace.ToXYZ([]float64{L, A, B}, ws)
	var rgb [3]float64
	color.SpaceDeviceRGB.FromXYZ(X, Y, Z, rgb[:], ws)
	return rgb[0], rgb[1], rgb[2]
}

// grayToL converts a DeviceGray value to an L* value.  The corresponding a*
// and b* are zero, since the colour is neutral.
func grayToL(gray float64) float64 {
	ws := &icc.Workspace{}
	X, Y, Z := color.SpaceDeviceGray.ToXYZ([]float64{gray}, ws)
	var lab [3]float64
	labSpace.FromXYZ(X, Y, Z, lab[:], ws)
	return lab[0]
}

// lToGray is the inverse of [grayToL].
func lToGray(L float64) float64 {
	ws := &icc.Workspace{}
	X, Y, Z := labSpace.ToXYZ([]float64{L, 0, 0}, ws)
	var gray [1]float64
	color.SpaceDeviceGray.FromXYZ(X, Y, Z, gray[:], ws)
	return gray[0]
}

// cmykToLab converts a DeviceCMYK colour to L*a*b*.
//
// The inks are mapped to RGB by the device formula rather than through the
// built-in CMYK profile the [color] package uses, so that [labToCMYK] returns
// an ink mixture this same formula reproduces.  Going through the profile
// instead would send the reverse direction through the profile's B2A tables,
// which need not land anywhere near the mixture the annotation started from.
func cmykToLab(c, m, y, k float64) (L, A, B float64) {
	return rgbToLab((1-c)*(1-k), (1-m)*(1-k), (1-y)*(1-k))
}

// labToCMYK is the inverse of [cmykToLab], up to the ink mixture chosen: the
// device formula is not injective, so the result is an equivalent mixture and
// not necessarily the one [cmykToLab] was given.
func labToCMYK(L, A, B float64) (c, m, y, k float64) {
	r, g, b := labToRGB(L, A, B)

	k = 1 - max(r, g, b)
	if k < 1 {
		c = clampFloat((1-r-k)/(1-k), 0, 1)
		m = clampFloat((1-g-k)/(1-k), 0, 1)
		y = clampFloat((1-b-k)/(1-k), 0, 1)
	}
	return c, m, y, clampFloat(k, 0, 1)
}
