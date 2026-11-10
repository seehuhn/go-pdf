// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cie

import (
	"math"
	"testing"
)

// srgbGammaPow and srgbGammaInvPow are the textbook forms of the sRGB
// transfer functions, used as the reference.
func srgbGammaPow(v float64) float64 {
	if v <= 0.0031308 {
		return 12.92 * v
	}
	return 1.055*math.Pow(v, 1/2.4) - 0.055
}

func srgbGammaInvPow(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

// maxULP is the largest permitted distance from the reference, in units in
// the last place of the reference value.
const maxULP = 16

// ulpDist returns the distance between got and want in units in the last
// place of want.
func ulpDist(got, want float64) float64 {
	if got == want {
		return 0
	}
	a := math.Abs(want)
	return math.Abs(got-want) / (math.Nextafter(a, math.Inf(1)) - a)
}

// gammaInputs returns a dense grid over [0, 1] together with the
// breakpoints, their neighbours and the end points.
func gammaInputs() []float64 {
	const n = 100000
	xs := make([]float64, 0, n+20)
	for i := range n + 1 {
		xs = append(xs, float64(i)/n)
	}
	for _, b := range []float64{0, 0.0031308, 0.04045, 1} {
		xs = append(xs, b,
			math.Nextafter(b, math.Inf(-1)), math.Nextafter(b, math.Inf(1)),
			b-1e-9, b+1e-9)
	}
	return xs
}

func TestSRGBGammaAccuracy(t *testing.T) {
	for _, v := range gammaInputs() {
		if d := ulpDist(srgbGamma(v), srgbGammaPow(v)); d > maxULP {
			t.Errorf("srgbGamma(%v) = %v, want %v (%.0f ulp)", v, srgbGamma(v), srgbGammaPow(v), d)
		}
		if d := ulpDist(srgbGammaInv(v), srgbGammaInvPow(v)); d > maxULP {
			t.Errorf("srgbGammaInv(%v) = %v, want %v (%.0f ulp)", v, srgbGammaInv(v), srgbGammaInvPow(v), d)
		}
	}
}

func TestSRGBGammaEndPoints(t *testing.T) {
	for _, f := range []struct {
		name string
		fn   func(float64) float64
	}{
		{"srgbGamma", srgbGamma},
		{"srgbGammaInv", srgbGammaInv},
	} {
		if got := f.fn(0); got != 0 {
			t.Errorf("%s(0) = %v, want 0", f.name, got)
		}
		if got := f.fn(1); got != 1 {
			t.Errorf("%s(1) = %v, want 1", f.name, got)
		}
	}
}

// TestSRGBGammaMonotone checks that neither transfer function decreases from
// one floating-point number to the next, at sample points spread over [0, 1]
// and across the breakpoints.
func TestSRGBGammaMonotone(t *testing.T) {
	const n = 100000
	const steps = 32
	starts := make([]float64, 0, n+3)
	for i := range n {
		starts = append(starts, float64(i)/n)
	}
	for _, b := range []float64{0.0031308, 0.04045, 1} {
		v := b
		for range steps / 2 {
			v = math.Nextafter(v, math.Inf(-1))
		}
		starts = append(starts, v)
	}

	for _, f := range []struct {
		name    string
		fn      func(float64) float64
		hasJump bool // may drop by up to 3e-8 just above 0.0031308
	}{
		// The segments of the standard do not meet: just above 0.0031308,
		// the power segment lies 3e-8 below the linear one.
		{"srgbGamma", srgbGamma, true},
		{"srgbGammaInv", srgbGammaInv, false},
	} {
		for _, v := range starts {
			prev := f.fn(v)
			for range steps {
				w := math.Nextafter(v, math.Inf(1))
				y := f.fn(w)
				atJump := f.hasJump && v == 0.0031308
				if y < prev && !(atJump && prev-y <= 3e-8) {
					t.Fatalf("%s(%v) = %v < %s(%v) = %v", f.name, w, y, f.name, v, prev)
				}
				v, prev = w, y
			}
		}
	}
}

func TestSRGBGammaRoundTrip(t *testing.T) {
	for _, v := range gammaInputs() {
		got := srgbGamma(srgbGammaInv(v))
		tol := 1e-14
		if v > 12.92*0.0031308 && v <= 0.04045 {
			// The breakpoints of the standard do not match exactly: these
			// values decode on the linear segment, just above 0.0031308, and
			// encode on the power segment, which lies 3e-8 below.
			tol = 3e-8
		}
		if !near(got, v, tol) {
			t.Errorf("srgbGamma(srgbGammaInv(%v)) = %v", v, got)
		}
	}
}

func TestExtendedSRGBGammaInv(t *testing.T) {
	for i := range 10001 {
		v := 100 * float64(i) / 10000
		want := srgbGammaInvPow(v)
		got := extendedSRGBGammaInv(v)
		if d := ulpDist(got, want); d > maxULP {
			t.Errorf("extendedSRGBGammaInv(%v) = %v, want %v (%.0f ulp)", v, got, want, d)
		}
		if neg := extendedSRGBGammaInv(-v); neg != -got {
			t.Errorf("extendedSRGBGammaInv(%v) = %v, want %v", -v, neg, -got)
		}
	}
	for _, v := range []float64{math.Inf(1), math.Inf(-1)} {
		if got := extendedSRGBGammaInv(v); got != v {
			t.Errorf("extendedSRGBGammaInv(%v) = %v", v, got)
		}
	}
	if got := extendedSRGBGammaInv(math.NaN()); !math.IsNaN(got) {
		t.Errorf("extendedSRGBGammaInv(NaN) = %v, want NaN", got)
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

// benchmarkGamma evaluates f on inputs spread over the power segment
// [lo, 1], where the two implementations differ.
func benchmarkGamma(b *testing.B, f func(float64) float64, lo float64) {
	xs := make([]float64, 1024)
	for i := range xs {
		xs[i] = lo + (1-lo)*float64(i)/float64(len(xs)-1)
	}
	i := 0
	for b.Loop() {
		f(xs[i&1023])
		i++
	}
}

func BenchmarkSRGBGamma(b *testing.B) {
	b.Run("Pow", func(b *testing.B) { benchmarkGamma(b, srgbGammaPow, 0.0032) })
	b.Run("Roots", func(b *testing.B) { benchmarkGamma(b, srgbGamma, 0.0032) })
}

func BenchmarkSRGBGammaInv(b *testing.B) {
	b.Run("Pow", func(b *testing.B) { benchmarkGamma(b, srgbGammaInvPow, 0.041) })
	b.Run("ExpLog", func(b *testing.B) { benchmarkGamma(b, srgbGammaInv, 0.041) })
}
