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

package color

import (
	stdcolor "image/color"
	"math"
)

// This file contains helper functions for converting between colour spaces,
// used by the RGBA() and Convert() methods throughout the color package.

// xyzToSRGB converts CIE XYZ (D50) to sRGB.
func xyzToSRGB(X, Y, Z float64) (r, g, b float64) {
	// Bradford chromatic adaptation D50 to D65
	X2 := 0.9555766*X - 0.0230393*Y + 0.0631636*Z
	Y2 := -0.0282895*X + 1.0099416*Y + 0.0210077*Z
	Z2 := 0.0122982*X - 0.0204830*Y + 1.3299098*Z

	// XYZ (D65) to linear sRGB
	rLin := 3.2404542*X2 - 1.5371385*Y2 - 0.4985314*Z2
	gLin := -0.9692660*X2 + 1.8760108*Y2 + 0.0415560*Z2
	bLin := 0.0556434*X2 - 0.2040259*Y2 + 1.0572252*Z2

	// sRGB gamma
	r = srgbGamma(rLin)
	g = srgbGamma(gLin)
	b = srgbGamma(bLin)
	return clamp01(r), clamp01(g), clamp01(b)
}

func srgbGamma(v float64) float64 {
	if v <= 0.0031308 {
		return 12.92 * v
	}
	return 1.055*math.Pow(v, 1/2.4) - 0.055
}

// srgbGammaInv converts sRGB gamma-encoded value to linear.
func srgbGammaInv(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

// srgbToXYZ converts sRGB [0,1] to CIE XYZ (D50).
func srgbToXYZ(r, g, b float64) (X, Y, Z float64) {
	// sRGB to linear
	rLin := srgbGammaInv(r)
	gLin := srgbGammaInv(g)
	bLin := srgbGammaInv(b)

	// linear sRGB to XYZ (D65)
	X2 := 0.4124564*rLin + 0.3575761*gLin + 0.1804375*bLin
	Y2 := 0.2126729*rLin + 0.7151522*gLin + 0.0721750*bLin
	Z2 := 0.0193339*rLin + 0.1191920*gLin + 0.9503041*bLin

	// Bradford chromatic adaptation D65 to D50
	X = 1.0478112*X2 + 0.0228866*Y2 - 0.0501270*Z2
	Y = 0.0295424*X2 + 0.9904844*Y2 - 0.0170491*Z2
	Z = -0.0092345*X2 + 0.0150436*Y2 + 0.7521316*Z2

	return X, Y, Z
}

// colorToXYZ extracts D50 XYZ from any Go color.Color.
// This assumes the input color is in sRGB space (as is typical for Go colors).
func colorToXYZ(c stdcolor.Color) (X, Y, Z float64) {
	r32, g32, b32, _ := c.RGBA()
	r := float64(r32) / 65535.0
	g := float64(g32) / 65535.0
	b := float64(b32) / 65535.0
	return srgbToXYZ(r, g, b)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// toUint32 converts a float64 in [0,1] to uint32 in [0,0xffff].
func toUint32(v float64) uint32 {
	return uint32(clamp01(v)*0xffff + 0.5)
}
