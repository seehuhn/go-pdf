// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

import (
	"fmt"
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestIdentityMatrix(t *testing.T) {
	for i, A := range testMatrices {
		t.Run(fmt.Sprintf("mat%d", i), func(t *testing.T) {
			B := A.Mul(IdentityMatrix)
			if d := cmp.Diff(A, B); d != "" {
				t.Error(d)
			}
			C := IdentityMatrix.Mul(A)
			if d := cmp.Diff(A, C); d != "" {
				t.Error(d)
			}
		})
	}
}

// TestMatrixInverse1 checks that a matrix multiplied by its inverse is the
// identity matrix.
func TestMatrixInverse1(t *testing.T) {
	for i, A := range testMatrices {
		t.Run(fmt.Sprintf("mat%d", i), func(t *testing.T) {
			Ainv := A.Inv()

			B := Ainv.Mul(A)
			if d := cmp.Diff(IdentityMatrix, B, cmpopts.EquateApprox(1e-6, 1e-6)); d != "" {
				t.Error(d)
			}

			B = A.Mul(Ainv)
			if d := cmp.Diff(IdentityMatrix, B, cmpopts.EquateApprox(1e-6, 1e-6)); d != "" {
				t.Error(d)
			}
		})
	}
}

// TestMatrixInverse2 checks that the inverse of the inverse of a matrix is the
// original matrix.
func TestMatrixInverse2(t *testing.T) {
	for i, A := range testMatrices {
		t.Run(fmt.Sprintf("mat%d", i), func(t *testing.T) {
			Ainv := A.Inv()
			B := Ainv.Inv()
			if d := cmp.Diff(A, B, cmpopts.EquateApprox(1e-6, 1e-6)); d != "" {
				t.Error(d)
			}
		})
	}
}

var testMatrices = []Matrix{
	IdentityMatrix,
	{2, 3, 4, 5, 6, 7},
	Translate(-0.5, 0.5),
	Translate(0, 1),
	Translate(1, 0),
	Translate(1, 2),
	Scale(0.5, 0.5),
	Scale(2, 1),
	Scale(1, 2),
	Scale(3, 4),
	Scale(-1, -1),
	Rotate(0.1),
	Rotate(math.Pi / 2),
	Rotate(math.Pi),
}
