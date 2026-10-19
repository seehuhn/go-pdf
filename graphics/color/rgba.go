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
	"sync/atomic"

	"seehuhn.de/go/pdf/internal/cie"
)

// This file contains helper functions for converting between colour spaces,
// used by the RGBA() and Convert() methods throughout the color package.

// ColorToXYZ extracts PCS-adapted CIE XYZ from any Go color.Color.
// For PDF colors, this uses the ToXYZ method directly, which converts with
// [icc.RelativeColorimetric].
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
	return cie.SRGBToXYZ(r, g, b)
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
// and [cie.PCSWhitePoint], in both directions.
type adaptation struct {
	toPCS   cie.Mat3
	fromPCS cie.Mat3
}

// newAdaptation derives the matrices adapting between wp and
// [cie.PCSWhitePoint].  A white point matching the PCS gives the identity in
// both directions, so that such a colour space converts without any rounding
// at all.
func newAdaptation(wp [3]float64) adaptation {
	if whitePointsEqual(wp[:], cie.PCSWhitePoint) {
		return adaptation{toPCS: cie.Identity, fromPCS: cie.Identity}
	}
	return adaptation{
		toPCS:   cie.BradfordMatrix(wp[:], cie.PCSWhitePoint),
		fromPCS: cie.BradfordMatrix(cie.PCSWhitePoint, wp[:]),
	}
}

// matrixInverse holds the inverse of a CalRGB matrix, in the column-major
// order the colour space stores it in.  ok is false when the matrix cannot be
// inverted, in which case there is no colour to return.
type matrixInverse struct {
	inverse cie.Mat3
	ok      bool
}

// newMatrixInverse inverts a 3x3 matrix held in column-major order.
//
// cie.Mat3 reads the entries in row-major order, which is the transpose of
// what the colour space stores.  Since the inverse of a transpose is the
// transpose of the inverse, inverting in that orientation yields the inverse
// in column-major order, with no transposing at either end.
func newMatrixInverse(m [9]float64) matrixInverse {
	inv := cie.Mat3(m).Inverse()
	return matrixInverse{inverse: inv, ok: allFinite(inv[:])}
}

// toUint32 converts a float64 in [0,1] to uint32 in [0,0xffff].
func toUint32(v float64) uint32 {
	return uint32(clip01(v)*0xffff + 0.5)
}
