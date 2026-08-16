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
	"slices"
	"sync/atomic"

	"seehuhn.de/go/icc"
)

// This file contains helper functions for converting between colour spaces,
// used by the RGBA() and Convert() methods throughout the color package.

// pcsWhitePoint is the white point every XYZ value exchanged between the
// colour spaces in this package is relative to.  It is the white point of the
// ICC Profile Connection Space, which is close to but not the same as
// [WhitePointD50], the CIE illuminant a PDF file can name.
var pcsWhitePoint = slices.Clone(icc.PCSWhitePoint[:])

// srgbWhitePoint is the white point of the sRGB primaries, the D65 illuminant.
//
// These are the tabulated D65 tristimulus values, which differ from
// [WhitePointD65] by 1.4e-5 in X and 2.3e-4 in Z.  The difference is
// deliberate: the sRGB standard states its white point as the chromaticity
// pair (0.3127, 0.3290), which is what [WhitePointD65] is built from, but the
// published sRGB conversion matrices are all derived from the values below.
// Using them keeps srgbToPCS comparable with that reference data.  The choice
// does not affect neutral colours, since the matrix is scaled to map white
// onto [pcsWhitePoint] either way.
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
// exact: the rows of srgbToPCS sum to [pcsWhitePoint], so white maps onto the
// PCS white point, and pcsToSRGB is a true inverse, so a colour survives the
// round trip.  Compositing relies on both, because blend modes such as colour
// burn test their operands against 0 and 1 exactly.
var srgbToPCS, pcsToSRGB = func() (mat3, mat3) {
	toWhite := primaryMatrix(srgbPrimaries, srgbWhitePoint)
	adapt := bradfordMatrix(srgbWhitePoint, pcsWhitePoint)
	fwd := adapt.mul(toWhite)
	return fwd, fwd.inverse()
}()

// xyzToSRGB converts PCS XYZ to sRGB.
func xyzToSRGB(X, Y, Z float64) (r, g, b float64) {
	rLin, gLin, bLin := pcsToSRGB.apply(X, Y, Z)
	r = srgbGamma(rLin)
	g = srgbGamma(gLin)
	b = srgbGamma(bLin)
	return clip01(r), clip01(g), clip01(b)
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

// srgbToXYZ converts sRGB [0,1] to PCS XYZ.
func srgbToXYZ(r, g, b float64) (X, Y, Z float64) {
	return srgbToPCS.apply(srgbGammaInv(r), srgbGammaInv(g), srgbGammaInv(b))
}

// ColorToXYZ extracts PCS-adapted CIE XYZ from any Go color.Color.
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
	return srgbToXYZ(r, g, b)
}

// bradford is the Bradford cone-response matrix, and bradfordInv its inverse.
// The inverse is computed rather than tabulated: rounding it to the seven
// decimals usually quoted moves the largest element by 4.8e-8, which is enough
// to keep an adaptation from returning the destination white point exactly.
var (
	bradford = mat3{
		0.8951, 0.2664, -0.1614,
		-0.7502, 1.7135, 0.0367,
		0.0389, -0.0685, 1.0296,
	}
	bradfordInv = bradford.inverse()
)

// bradfordMatrix returns the Bradford chromatic adaptation from srcWP to dstWP
// as a single matrix.  Both white points are CIE 1931 XYZ coordinates.
func bradfordMatrix(srcWP, dstWP []float64) mat3 {
	src := bradford.apply3(srcWP)
	dst := bradford.apply3(dstWP)

	// scale each cone response by the destination/source ratio
	var scaled mat3
	for i := range 3 {
		ratio := dst[i] / src[i]
		for j := range 3 {
			scaled[i*3+j] = ratio * bradford[i*3+j]
		}
	}
	return bradfordInv.mul(scaled)
}

// whitePointsEqual reports whether two white points are close enough that
// adapting between them can be skipped.
func whitePointsEqual(a, b []float64) bool {
	return math.Abs(a[0]-b[0]) < 1e-10 &&
		math.Abs(a[1]-b[1]) < 1e-10 &&
		math.Abs(a[2]-b[2]) < 1e-10
}

// paramCache memoises a value derived from one colour space parameter.  The
// zero value is ready to use, and the cache is safe for concurrent use.
//
// The parameters of a colour space are exported fields, so the cached value is
// checked against the current parameter on every conversion rather than
// derived once when the space is built.  A space assembled as a struct
// literal, or one whose parameter is changed later, therefore still converts
// correctly.
type paramCache[K comparable, V any] struct {
	cached atomic.Pointer[derived[K, V]]
}

// derived is a value together with the parameter it was derived from.
type derived[K comparable, V any] struct {
	key   K
	value V
}

// get returns the value derived from key, calling derive if the cached value
// belongs to a different key.  derive must be a pure function of its argument,
// so that concurrent callers agree on the result.
func (c *paramCache[K, V]) get(key K, derive func(K) V) *derived[K, V] {
	d := c.cached.Load()
	if d == nil || d.key != key {
		d = &derived[K, V]{key: key, value: derive(key)}
		c.cached.Store(d)
	}
	return d
}

// adaptation holds the Bradford matrices between a colour space's white point
// and [pcsWhitePoint], in both directions.
type adaptation struct {
	toPCS   mat3
	fromPCS mat3
}

// newAdaptation derives the matrices adapting between wp and [pcsWhitePoint].
// A white point matching the PCS gives the identity in both directions, so
// that such a colour space converts without any rounding at all.
func newAdaptation(wp [3]float64) adaptation {
	if whitePointsEqual(wp[:], pcsWhitePoint) {
		return adaptation{toPCS: identity3, fromPCS: identity3}
	}
	return adaptation{
		toPCS:   bradfordMatrix(wp[:], pcsWhitePoint),
		fromPCS: bradfordMatrix(pcsWhitePoint, wp[:]),
	}
}

// matrixInverse holds the inverse of a CalRGB matrix, in the column-major
// order the colour space stores it in.  ok is false when the matrix cannot be
// inverted, in which case there is no colour to return.
type matrixInverse struct {
	inverse mat3
	ok      bool
}

// newMatrixInverse inverts a 3x3 matrix held in column-major order.
//
// mat3 reads the entries in row-major order, which is the transpose of what
// the colour space stores.  Since the inverse of a transpose is the transpose
// of the inverse, inverting in that orientation yields the inverse in
// column-major order, with no transposing at either end.
func newMatrixInverse(m [9]float64) matrixInverse {
	inv := mat3(m).inverse()
	return matrixInverse{inverse: inv, ok: allFinite(inv[:])}
}

// mat3 is a 3x3 matrix in row-major order.
type mat3 [9]float64

var identity3 = mat3{
	1, 0, 0,
	0, 1, 0,
	0, 0, 1,
}

// apply multiplies the matrix with the column vector (x, y, z).
func (m mat3) apply(x, y, z float64) (float64, float64, float64) {
	return m[0]*x + m[1]*y + m[2]*z,
		m[3]*x + m[4]*y + m[5]*z,
		m[6]*x + m[7]*y + m[8]*z
}

// applyT multiplies the transpose of the matrix with the column vector
// (x, y, z).  A matrix held in column-major order is the transpose of the
// mat3 with the same entries, so this applies it in its stored orientation.
func (m mat3) applyT(x, y, z float64) (float64, float64, float64) {
	return m[0]*x + m[3]*y + m[6]*z,
		m[1]*x + m[4]*y + m[7]*z,
		m[2]*x + m[5]*y + m[8]*z
}

// apply3 is [mat3.apply] for a vector held in a slice of length three.
func (m mat3) apply3(v []float64) [3]float64 {
	x, y, z := m.apply(v[0], v[1], v[2])
	return [3]float64{x, y, z}
}

// mul returns the matrix product m*n.
func (m mat3) mul(n mat3) mat3 {
	var out mat3
	for i := range 3 {
		for j := range 3 {
			var sum float64
			for k := range 3 {
				sum += m[i*3+k] * n[k*3+j]
			}
			out[i*3+j] = sum
		}
	}
	return out
}

// inverse returns the matrix inverse of m.  The matrices this package inverts
// are all well conditioned, so a singular matrix would be a programming error.
func (m mat3) inverse() mat3 {
	det := m[0]*(m[4]*m[8]-m[5]*m[7]) -
		m[1]*(m[3]*m[8]-m[5]*m[6]) +
		m[2]*(m[3]*m[7]-m[4]*m[6])
	adj := mat3{
		m[4]*m[8] - m[5]*m[7], m[2]*m[7] - m[1]*m[8], m[1]*m[5] - m[2]*m[4],
		m[5]*m[6] - m[3]*m[8], m[0]*m[8] - m[2]*m[6], m[2]*m[3] - m[0]*m[5],
		m[3]*m[7] - m[4]*m[6], m[1]*m[6] - m[0]*m[7], m[0]*m[4] - m[1]*m[3],
	}
	for i := range adj {
		adj[i] /= det
	}
	return adj
}

// primaryMatrix returns the matrix converting linear colour values in a space
// with the given primary chromaticities to CIE XYZ.  Its rows sum to white,
// so that (1, 1, 1) maps onto the white point exactly.
func primaryMatrix(primaries [3][2]float64, white []float64) mat3 {
	// unscaled primaries, one per column
	var p mat3
	for i, c := range primaries {
		x, y := c[0], c[1]
		p[i] = x / y
		p[3+i] = 1
		p[6+i] = (1 - x - y) / y
	}

	// scale each column so that the columns add up to the white point
	s := p.inverse().apply3(white)
	var out mat3
	for i := range 3 {
		for j := range 3 {
			out[i*3+j] = p[i*3+j] * s[j]
		}
	}
	return out
}

// toUint32 converts a float64 in [0,1] to uint32 in [0,0xffff].
func toUint32(v float64) uint32 {
	return uint32(clip01(v)*0xffff + 0.5)
}
