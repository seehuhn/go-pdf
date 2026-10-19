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

// Package cie holds the colour arithmetic shared by the PDF colour spaces:
// 3×3 matrices, Bradford chromatic adaptation, and the conversion between
// sRGB and the ICC Profile Connection Space.
package cie

import (
	"slices"

	"seehuhn.de/go/icc"
)

// PCSWhitePoint is the white point every XYZ value exchanged between the
// PDF colour spaces is relative to.  It is the white point of the ICC
// Profile Connection Space, which is close to but not the same as the D50
// illuminant a PDF file can name.
var PCSWhitePoint = slices.Clone(icc.PCSWhitePoint[:])

// Mat3 is a 3x3 matrix in row-major order.
type Mat3 [9]float64

// Identity is the identity matrix.
var Identity = Mat3{
	1, 0, 0,
	0, 1, 0,
	0, 0, 1,
}

// Apply multiplies the matrix with the column vector (x, y, z).
func (m Mat3) Apply(x, y, z float64) (float64, float64, float64) {
	return m[0]*x + m[1]*y + m[2]*z,
		m[3]*x + m[4]*y + m[5]*z,
		m[6]*x + m[7]*y + m[8]*z
}

// ApplyT multiplies the transpose of the matrix with the column vector
// (x, y, z).  A matrix held in column-major order is the transpose of the
// Mat3 with the same entries, so this applies it in its stored orientation.
func (m Mat3) ApplyT(x, y, z float64) (float64, float64, float64) {
	return m[0]*x + m[3]*y + m[6]*z,
		m[1]*x + m[4]*y + m[7]*z,
		m[2]*x + m[5]*y + m[8]*z
}

// Apply3 is [Mat3.Apply] for a vector held in a slice of length three.
func (m Mat3) Apply3(v []float64) [3]float64 {
	x, y, z := m.Apply(v[0], v[1], v[2])
	return [3]float64{x, y, z}
}

// Mul returns the matrix product m*n.
func (m Mat3) Mul(n Mat3) Mat3 {
	var out Mat3
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

// Inverse returns the matrix inverse of m.  A singular matrix gives
// non-finite entries; callers inverting matrices from untrusted input
// must check for them.
func (m Mat3) Inverse() Mat3 {
	det := m[0]*(m[4]*m[8]-m[5]*m[7]) -
		m[1]*(m[3]*m[8]-m[5]*m[6]) +
		m[2]*(m[3]*m[7]-m[4]*m[6])
	adj := Mat3{
		m[4]*m[8] - m[5]*m[7], m[2]*m[7] - m[1]*m[8], m[1]*m[5] - m[2]*m[4],
		m[5]*m[6] - m[3]*m[8], m[0]*m[8] - m[2]*m[6], m[2]*m[3] - m[0]*m[5],
		m[3]*m[7] - m[4]*m[6], m[1]*m[6] - m[0]*m[7], m[0]*m[4] - m[1]*m[3],
	}
	for i := range adj {
		adj[i] /= det
	}
	return adj
}

// bradford is the Bradford cone-response matrix, and bradfordInv its inverse.
// The inverse is computed rather than tabulated: rounding it to the seven
// decimals usually quoted moves the largest element by 4.8e-8, which is enough
// to keep an adaptation from returning the destination white point exactly.
var (
	bradford = Mat3{
		0.8951, 0.2664, -0.1614,
		-0.7502, 1.7135, 0.0367,
		0.0389, -0.0685, 1.0296,
	}
	bradfordInv = bradford.Inverse()
)

// BradfordMatrix returns the Bradford chromatic adaptation from srcWP to dstWP
// as a single matrix.  Both white points are CIE 1931 XYZ coordinates.
func BradfordMatrix(srcWP, dstWP []float64) Mat3 {
	src := bradford.Apply3(srcWP)
	dst := bradford.Apply3(dstWP)

	// scale each cone response by the destination/source ratio
	var scaled Mat3
	for i := range 3 {
		ratio := dst[i] / src[i]
		for j := range 3 {
			scaled[i*3+j] = ratio * bradford[i*3+j]
		}
	}
	return bradfordInv.Mul(scaled)
}

// primaryMatrix returns the matrix converting linear colour values in a space
// with the given primary chromaticities to CIE XYZ.  Its rows sum to white,
// so that (1, 1, 1) maps onto the white point exactly.
func primaryMatrix(primaries [3][2]float64, white []float64) Mat3 {
	// unscaled primaries, one per column
	var p Mat3
	for i, c := range primaries {
		x, y := c[0], c[1]
		p[i] = x / y
		p[3+i] = 1
		p[6+i] = (1 - x - y) / y
	}

	// scale each column so that the columns add up to the white point
	s := p.Inverse().Apply3(white)
	var out Mat3
	for i := range 3 {
		for j := range 3 {
			out[i*3+j] = p[i*3+j] * s[j]
		}
	}
	return out
}
