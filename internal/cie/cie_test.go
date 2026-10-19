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

// TestSRGBWhiteIsPCSWhite checks that sRGB white lands on the white point of
// the Profile Connection Space.  Compositing depends on the exact value:
// blend modes such as colour burn test their operands against 0 and 1.
func TestSRGBWhiteIsPCSWhite(t *testing.T) {
	X, Y, Z := SRGBToXYZ(1, 1, 1)
	if !near(X, PCSWhitePoint[0], 1e-14) ||
		!near(Y, PCSWhitePoint[1], 1e-14) ||
		!near(Z, PCSWhitePoint[2], 1e-14) {
		t.Errorf("sRGB white -> (%g, %g, %g), want %v", X, Y, Z, PCSWhitePoint)
	}
}

// TestSRGBMatrixRowSums checks the property TestSRGBWhiteIsPCSWhite relies on:
// the rows of the conversion matrix add up to the white point.
func TestSRGBMatrixRowSums(t *testing.T) {
	for i := range 3 {
		sum := srgbToPCS[i*3] + srgbToPCS[i*3+1] + srgbToPCS[i*3+2]
		if !near(sum, PCSWhitePoint[i], 1e-14) {
			t.Errorf("row %d sums to %g, want %g", i, sum, PCSWhitePoint[i])
		}
	}
}

// TestSRGBPrimariesMatchPublishedMatrix compares the derived sRGB primaries,
// before chromatic adaptation, against the sRGB to XYZ (D65) matrix as it is
// published.  This is what justifies taking [srgbWhitePoint] from the
// tabulated D65 tristimulus values rather than from the chromaticity pair the
// sRGB standard quotes: substituting the latter here moves the matrix
// 2.3e-4 away from the published values, well outside their rounding.
func TestSRGBPrimariesMatchPublishedMatrix(t *testing.T) {
	published := Mat3{
		0.4124564, 0.3575761, 0.1804375,
		0.2126729, 0.7151522, 0.0721750,
		0.0193339, 0.1191920, 0.9503041,
	}
	got := primaryMatrix(srgbPrimaries, srgbWhitePoint)
	for i := range got {
		if !near(got[i], published[i], 1e-7) {
			t.Errorf("element %d = %.8f, want %.8f", i, got[i], published[i])
		}
	}
}

// TestSRGBMatrixInverse checks that the two directions invert each other, so
// that a colour survives a conversion to XYZ and back.
func TestSRGBMatrixInverse(t *testing.T) {
	for _, rgb := range [][3]float64{
		{0, 0, 0}, {1, 1, 1}, {1, 0, 0}, {0.2, 0.4, 0.6}, {0.5, 0.5, 0.5},
	} {
		X, Y, Z := SRGBToXYZ(rgb[0], rgb[1], rgb[2])
		r, g, b := XYZToSRGB(X, Y, Z)
		if !near(r, rgb[0], 1e-12) || !near(g, rgb[1], 1e-12) || !near(b, rgb[2], 1e-12) {
			t.Errorf("%v -> (%g, %g, %g)", rgb, r, g, b)
		}
	}
}

func TestXYZToSRGBClips(t *testing.T) {
	r, g, b := XYZToSRGB(2, 2, 2)
	if r != 1 || g != 1 || b != 1 {
		t.Errorf("got (%g, %g, %g), want (1, 1, 1)", r, g, b)
	}
	r, g, b = XYZToSRGB(math.NaN(), -1, 0)
	if r != 0 || g != 0 || b != 0 {
		t.Errorf("got (%g, %g, %g), want (0, 0, 0)", r, g, b)
	}
}

func TestExtendedSRGBToXYZ(t *testing.T) {
	X, Y, Z := ExtendedSRGBToXYZ(1, 1, 1)
	if !near(X, PCSWhitePoint[0], 1e-14) || !near(Y, 1, 1e-14) || !near(Z, PCSWhitePoint[2], 1e-14) {
		t.Errorf("white: got (%g,%g,%g)", X, Y, Z)
	}

	for _, v := range [][3]float64{{0.2, 0.5, 0.9}, {0, 1, 0.04}} {
		X1, Y1, Z1 := ExtendedSRGBToXYZ(v[0], v[1], v[2])
		X2, Y2, Z2 := SRGBToXYZ(v[0], v[1], v[2])
		if !near(X1, X2, 1e-12) || !near(Y1, Y2, 1e-12) || !near(Z1, Z2, 1e-12) {
			t.Errorf("%v: differs from SRGBToXYZ", v)
		}
	}

	// odd symmetry of the linearisation
	X1, Y1, Z1 := ExtendedSRGBToXYZ(-0.5, -0.5, -0.5)
	X2, Y2, Z2 := ExtendedSRGBToXYZ(0.5, 0.5, 0.5)
	if !near(X1, -X2, 1e-12) || !near(Y1, -Y2, 1e-12) || !near(Z1, -Z2, 1e-12) {
		t.Errorf("not odd: (%g,%g,%g) vs (%g,%g,%g)", X1, Y1, Z1, X2, Y2, Z2)
	}
	// linear value of 0.5 is about 0.2140
	if _, Y, _ := ExtendedSRGBToXYZ(0.5, 0.5, 0.5); !near(Y, 0.2140, 1e-3) {
		t.Errorf("got %g", Y)
	}
}
