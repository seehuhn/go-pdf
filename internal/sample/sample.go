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

// Package sample maps between n-bit integer codes and real values, as
// PDF Decode arrays do.
//
// With N = 2^Bits-1, a [Map] guarantees the following, in floating-point
// arithmetic and not only in exact arithmetic:
//
//   - Decode(0) is exactly Min and Decode(N) is exactly Max.
//   - Encode(Decode(j)) == j.  Where floats cannot separate neighbouring
//     codes, Encode returns one of the codes which decode to the value.
//   - Decode is monotone in the code, and Encode is monotone in its
//     argument (both decreasing when Min > Max).
//   - Encode gives every code an equal share of the range: code j covers
//     the fraction [j/(N+1), (j+1)/(N+1)) of the way from Min to Max, up to
//     float resolution.
//   - Results are finite and inside the range for all finite Min and Max,
//     even where Max-Min overflows.  Encode clamps values outside the range
//     and maps NaN to 0.
//   - Results are the same on every platform: no computation can be fused
//     into a multiply-add.
package sample

import "math"

// Map is the linear map between the integer codes 0, ..., 2^Bits-1 and the
// real interval from Min to Max.  Min may be larger than Max.
type Map struct {
	Bits     int // 1 to 32
	Min, Max float64
}

// maxCode returns the largest code, 2^Bits-1.
func (m Map) maxCode() uint32 {
	return uint32(uint64(1)<<m.Bits - 1)
}

// Decode returns the value represented by code.
func (m Map) Decode(code uint32) float64 {
	t := float64(code) / float64(m.maxCode())
	return Lerp(t, m.Min, m.Max)
}

// Encode returns the code whose share of the range contains x.
func (m Map) Encode(x float64) uint32 {
	return m.correct(m.guess(x), x)
}

// guess returns the code whose bin contains x, ignoring rounding errors.
func (m Map) guess(x float64) uint32 {
	if m.Min == m.Max {
		return 0
	}
	t := (x - m.Min) / (m.Max - m.Min)
	if math.IsInf(m.Max-m.Min, 0) {
		// halving keeps the width of the range finite
		t = (x/2 - m.Min/2) / (m.Max/2 - m.Min/2)
	}

	// clamp in float64, since converting an out-of-range float to an
	// integer is implementation-defined
	n := m.maxCode()
	z := math.Floor(t * (float64(n) + 1))
	if !(z > 0) { // also catches NaN
		return 0
	}
	return uint32(min(z, float64(n)))
}

// correct moves j by one code where rounding errors have put x on the wrong
// side of a neighbouring code's value.  Without this, Encode(Decode(j))
// could differ from j.
func (m Map) correct(j uint32, x float64) uint32 {
	if m.Min == m.Max {
		return j
	}

	// x belongs to a neighbour once it reaches that neighbour's value
	up := m.Max > m.Min
	if j < m.maxCode() {
		v := m.Decode(j + 1)
		if up && x >= v || !up && x <= v {
			return j + 1
		}
	}
	if j > 0 {
		v := m.Decode(j - 1)
		if up && x <= v || !up && x >= v {
			return j - 1
		}
	}
	return j
}

// Lerp returns the point at fraction t of the way from a to b.
//
// For finite a and b and for t in [0, 1], the result is finite and lies
// between a and b.  It equals a for t = 0 and b for t = 1, and it is
// monotone in t.  The algorithm is the one of C++20's std::lerp.
func Lerp(t, a, b float64) float64 {
	// The float64 conversions prevent fused multiply-add, which some
	// platforms use and others do not.
	if a <= 0 && b >= 0 || a >= 0 && b <= 0 {
		// a and b have opposite signs, so b-a might overflow
		return float64(t*b) + float64((1-t)*a)
	}
	if t == 1 {
		return b
	}
	x := a + float64(t*(b-a))
	if b > a {
		return min(x, b)
	}
	return max(x, b)
}
