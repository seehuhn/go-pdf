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

package cie

import "math"

// srgbWhitePoint is the white point of the sRGB primaries, the D65 illuminant.
//
// These are the tabulated D65 tristimulus values, which differ from the
// D65 white point built from the chromaticity pair (0.3127, 0.3290) by
// 1.4e-5 in X and 2.3e-4 in Z.  The difference is deliberate: the sRGB
// standard states its white point as that chromaticity pair, but the
// published sRGB conversion matrices are all derived from the values below.
// Using them keeps srgbToPCS comparable with that reference data.  The choice
// does not affect neutral colours, since the matrix is scaled to map white
// onto [PCSWhitePoint] either way.
var srgbWhitePoint = []float64{0.95047, 1.0, 1.08883}

// CIE 1931 chromaticity coordinates of the sRGB primaries.
var srgbPrimaries = [3][2]float64{
	{0.6400, 0.3300}, // red
	{0.3000, 0.6000}, // green
	{0.1500, 0.0600}, // blue
}

// srgbToPCS converts linear sRGB to PCS XYZ, and pcsToSRGB inverts it.  Each
// folds the sRGB primaries and the chromatic adaptation into a single matrix.
//
// Deriving the pair rather than tabulating it is what makes the conversion
// exact: the rows of srgbToPCS sum to [PCSWhitePoint], so white maps onto the
// PCS white point, and pcsToSRGB is a true inverse, so a colour survives the
// round trip.  Compositing relies on both, because blend modes such as colour
// burn test their operands against 0 and 1 exactly.
var srgbToPCS, pcsToSRGB = func() (Mat3, Mat3) {
	toWhite := primaryMatrix(srgbPrimaries, srgbWhitePoint)
	adapt := BradfordMatrix(srgbWhitePoint, PCSWhitePoint)
	fwd := adapt.Mul(toWhite)
	return fwd, fwd.Inverse()
}()

// XYZToSRGB converts PCS XYZ to sRGB.  The result is clipped to [0, 1];
// a NaN component becomes 0.
func XYZToSRGB(X, Y, Z float64) (r, g, b float64) {
	rLin, gLin, bLin := pcsToSRGB.Apply(X, Y, Z)
	r = srgbGamma(rLin)
	g = srgbGamma(gLin)
	b = srgbGamma(bLin)
	return clip01(r), clip01(g), clip01(b)
}

// clip01 clips v to [0, 1]; a NaN becomes 0.
func clip01(v float64) float64 {
	if !(v >= 0) {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
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

// SRGBToXYZ converts sRGB [0,1] to PCS XYZ.
func SRGBToXYZ(r, g, b float64) (X, Y, Z float64) {
	return srgbToPCS.Apply(srgbGammaInv(r), srgbGammaInv(g), srgbGammaInv(b))
}

// ExtendedSRGBToXYZ converts extended-range sRGB to PCS XYZ.
// The transfer function is extended to negative values by odd symmetry, and
// no clipping is applied, so components outside [0, 1] map to colours outside
// the sRGB gamut.
func ExtendedSRGBToXYZ(r, g, b float64) (X, Y, Z float64) {
	return srgbToPCS.Apply(extendedSRGBGammaInv(r), extendedSRGBGammaInv(g), extendedSRGBGammaInv(b))
}

func extendedSRGBGammaInv(v float64) float64 {
	if v < 0 {
		return -srgbGammaInv(-v)
	}
	return srgbGammaInv(v)
}
