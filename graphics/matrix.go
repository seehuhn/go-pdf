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

package graphics

import "math"

// Matrix contains a PDF transformation matrix.
// The elements are stored in the same order as for the "cm" operator.
//
// If M = [a b c d e f] is a [Matrix], then M corresponds to the following
// 3x3 matrix:
//
//	/ a b 0 \
//	| c d 0 |
//	\ e f 1 /
//
// A vector (x, y, 1) is transformed by M into
//
//	(x y 1) * M = (a*x+c*y+e, b*x+d*y+f, 1)
type Matrix [6]float64

// Apply applies the transformation matrix to the given vector.
func (M Matrix) Apply(x, y float64) (float64, float64) {
	return x*M[0] + y*M[2] + M[4], x*M[1] + y*M[3] + M[5]
}

// Mul multiplies two transformation matrices and returns the result.
// The result is equivalent to first applying A and then B.
func (M Matrix) Mul(B Matrix) Matrix {
	// / A0 A1 0 \  / B0 B1 0 \   / A0*B0+A1*B2    A0*B1+A1*B3    0 \
	// | A2 A3 0 |  | B2 B3 0 | = | A2*B0+A3*B2    A2*B1+A3*B3    0 |
	// \ A4 A5 1 /  \ B4 B5 1 /   \ A4*B0+A5*B2+B4 A4*B1+A5*B3+B5 1 /
	return Matrix{
		M[0]*B[0] + M[1]*B[2],
		M[0]*B[1] + M[1]*B[3],
		M[2]*B[0] + M[3]*B[2],
		M[2]*B[1] + M[3]*B[3],
		M[4]*B[0] + M[5]*B[2] + B[4],
		M[4]*B[1] + M[5]*B[3] + B[5],
	}
}

// Inv computes the inverse of the transformation matrix M.
func (M Matrix) Inv() Matrix {
	det := M[0]*M[3] - M[1]*M[2]
	if det == 0 {
		panic("singular matrix")
	}
	invDet := 1 / det
	return Matrix{
		M[3] * invDet, -M[1] * invDet,
		-M[2] * invDet, M[0] * invDet,
		(M[2]*M[5] - M[3]*M[4]) * invDet,
		(M[1]*M[4] - M[0]*M[5]) * invDet,
	}
}

// IdentityMatrix is the identity transformation.
var IdentityMatrix = Matrix{1, 0, 0, 1, 0, 0}

// Translate moves the origin of the coordinate system.
//
// Drawing the unit square [0, 1] x [0, 1] after applying this transformation
// is equivalent to drawing the rectangle [dx, dx+1] x [dy, dy+1] in the
// original coordinate system.
func Translate(dx, dy float64) Matrix {
	return Matrix{1, 0, 0, 1, dx, dy}
}

// Scale scales the coordinate system.
//
// Drawing the unit square [0, 1] x [0, 1] after applying this transformation
// is equivalent to drawing the rectangle [0, xScale] x [0, yScale] in the
// original coordinate system.
func Scale(xScale, yScale float64) Matrix {
	return Matrix{xScale, 0, 0, yScale, 0, 0}
}

// Rotate rotates the coordinate system by the given angle (in radians).
func Rotate(phi float64) Matrix {
	c := math.Cos(phi)
	s := math.Sin(phi)
	return Matrix{c, s, -s, c, 0, 0}
}
