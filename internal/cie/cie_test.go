// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cie

import (
	"math"
	"testing"
)

// CIE illuminants D50 and D65, as PDF files name them.
var (
	whitePointD50 = []float64{0.9642, 1.0, 0.8249}
	whitePointD65 = []float64{0.9505, 1.0, 1.0890}
)

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestBradfordAdaptIdentity(t *testing.T) {
	// adapting with identical whitepoints should be the identity
	X, Y, Z := 0.4, 0.5, 0.3
	Xo, Yo, Zo := BradfordMatrix(whitePointD65, whitePointD65).Apply(X, Y, Z)
	if !near(Xo, X, 1e-10) || !near(Yo, Y, 1e-10) || !near(Zo, Z, 1e-10) {
		t.Errorf("identity adaptation failed: got (%g,%g,%g), want (%g,%g,%g)",
			Xo, Yo, Zo, X, Y, Z)
	}
}

func TestBradfordAdaptD65White(t *testing.T) {
	// D65 white adapted to D50 should give D50 white.  The adaptation scales
	// the source white's cone response into the destination's, so this is
	// exact up to the accuracy of the inverse cone-response matrix.
	Xo, Yo, Zo := BradfordMatrix(whitePointD65, whitePointD50).Apply(
		whitePointD65[0], whitePointD65[1], whitePointD65[2])
	if !near(Xo, whitePointD50[0], 1e-14) ||
		!near(Yo, whitePointD50[1], 1e-14) ||
		!near(Zo, whitePointD50[2], 1e-14) {
		t.Errorf("D65 white -> D50: got (%g,%g,%g), want %v", Xo, Yo, Zo, whitePointD50)
	}
}

// TestBradfordInverse checks that the cone-response matrix and its inverse are
// inverse to each other.  Rounding the inverse to the seven decimals usually
// quoted for it moves the largest element by 4.8e-8, enough to keep an
// adaptation from reproducing the destination white point.
func TestBradfordInverse(t *testing.T) {
	got := bradford.Mul(bradfordInv)
	for i := range 3 {
		for j := range 3 {
			want := 0.0
			if i == j {
				want = 1
			}
			if !near(got[i*3+j], want, 1e-14) {
				t.Errorf("element (%d,%d) = %g, want %g", i, j, got[i*3+j], want)
			}
		}
	}
}

func TestBradfordAdaptRoundTrip(t *testing.T) {
	// adapting D50->D65->D50 should be the identity
	X, Y, Z := 0.3, 0.4, 0.2
	X2, Y2, Z2 := BradfordMatrix(whitePointD50, whitePointD65).Apply(X, Y, Z)
	X3, Y3, Z3 := BradfordMatrix(whitePointD65, whitePointD50).Apply(X2, Y2, Z2)
	if !near(X3, X, 1e-7) || !near(Y3, Y, 1e-7) || !near(Z3, Z, 1e-7) {
		t.Errorf("round-trip failed: got (%g,%g,%g), want (%g,%g,%g)",
			X3, Y3, Z3, X, Y, Z)
	}
}
