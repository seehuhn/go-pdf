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

// XYZToSRGB converts CIE XYZ (D50) to sRGB.
func XYZToSRGB(X, Y, Z float64) (r, g, b float64) {
	// adapt from D50 to D65
	X2, Y2, Z2 := bradfordAdapt(X, Y, Z, WhitePointD50, WhitePointD65)

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

// SRGBToXYZ converts sRGB [0,1] to CIE XYZ (D50).
func SRGBToXYZ(r, g, b float64) (X, Y, Z float64) {
	// sRGB to linear
	rLin := srgbGammaInv(r)
	gLin := srgbGammaInv(g)
	bLin := srgbGammaInv(b)

	// linear sRGB to XYZ (D65)
	X2 := 0.4124564*rLin + 0.3575761*gLin + 0.1804375*bLin
	Y2 := 0.2126729*rLin + 0.7151522*gLin + 0.0721750*bLin
	Z2 := 0.0193339*rLin + 0.1191920*gLin + 0.9503041*bLin

	// adapt from D65 to D50
	return bradfordAdapt(X2, Y2, Z2, WhitePointD65, WhitePointD50)
}

// ColorToXYZ extracts D50 XYZ from any Go color.Color.
// For PDF colors, this uses the ToXYZ method directly.
// For other colors, the input is assumed to be in sRGB space.
func ColorToXYZ(c stdcolor.Color) (X, Y, Z float64) {
	if cc, ok := c.(Color); ok {
		return cc.ToXYZ()
	}
	// sRGB fallback for non-PDF colors
	r32, g32, b32, _ := c.RGBA()
	r := float64(r32) / 65535.0
	g := float64(g32) / 65535.0
	b := float64(b32) / 65535.0
	return SRGBToXYZ(r, g, b)
}

// bradfordAdapt performs Bradford chromatic adaptation from srcWP to dstWP.
// Both whitepoints are CIE 1931 XYZ coordinates (e.g. WhitePointD50).
func bradfordAdapt(X, Y, Z float64, srcWP, dstWP []float64) (float64, float64, float64) {
	// short-circuit when whitepoints are (nearly) identical
	if math.Abs(srcWP[0]-dstWP[0]) < 1e-10 &&
		math.Abs(srcWP[1]-dstWP[1]) < 1e-10 &&
		math.Abs(srcWP[2]-dstWP[2]) < 1e-10 {
		return X, Y, Z
	}

	// Bradford cone-response matrix M
	const (
		m00 = 0.8951
		m01 = 0.2664
		m02 = -0.1614
		m10 = -0.7502
		m11 = 1.7135
		m12 = 0.0367
		m20 = 0.0389
		m21 = -0.0685
		m22 = 1.0296
	)

	// cone responses for source and destination whitepoints
	srcR := m00*srcWP[0] + m01*srcWP[1] + m02*srcWP[2]
	srcG := m10*srcWP[0] + m11*srcWP[1] + m12*srcWP[2]
	srcB := m20*srcWP[0] + m21*srcWP[1] + m22*srcWP[2]

	dstR := m00*dstWP[0] + m01*dstWP[1] + m02*dstWP[2]
	dstG := m10*dstWP[0] + m11*dstWP[1] + m12*dstWP[2]
	dstB := m20*dstWP[0] + m21*dstWP[1] + m22*dstWP[2]

	// cone response of input colour
	cR := m00*X + m01*Y + m02*Z
	cG := m10*X + m11*Y + m12*Z
	cB := m20*X + m21*Y + m22*Z

	// scale by destination/source ratio
	cR *= dstR / srcR
	cG *= dstG / srcG
	cB *= dstB / srcB

	// inverse Bradford matrix M^-1
	const (
		i00 = 0.9869929
		i01 = -0.1470543
		i02 = 0.1599627
		i10 = 0.4323053
		i11 = 0.5183603
		i12 = 0.0492912
		i20 = -0.0085287
		i21 = 0.0400428
		i22 = 0.9684867
	)

	Xo := i00*cR + i01*cG + i02*cB
	Yo := i10*cR + i11*cG + i12*cB
	Zo := i20*cR + i21*cG + i22*cB
	return Xo, Yo, Zo
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
