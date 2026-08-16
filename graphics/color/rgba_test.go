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
	"math"
	"testing"

	"seehuhn.de/go/icc"
)

func TestBradfordAdaptIdentity(t *testing.T) {
	// adapting with identical whitepoints should be the identity
	X, Y, Z := 0.4, 0.5, 0.3
	Xo, Yo, Zo := bradfordMatrix(WhitePointD65, WhitePointD65).apply(X, Y, Z)
	if math.Abs(Xo-X) > 1e-10 || math.Abs(Yo-Y) > 1e-10 || math.Abs(Zo-Z) > 1e-10 {
		t.Errorf("identity adaptation failed: got (%g,%g,%g), want (%g,%g,%g)",
			Xo, Yo, Zo, X, Y, Z)
	}
}

func TestBradfordAdaptD65White(t *testing.T) {
	// D65 white adapted to D50 should give D50 white.  The adaptation scales
	// the source white's cone response into the destination's, so this is
	// exact up to the accuracy of the inverse cone-response matrix.
	Xo, Yo, Zo := bradfordMatrix(WhitePointD65, WhitePointD50).apply(
		WhitePointD65[0], WhitePointD65[1], WhitePointD65[2])
	if math.Abs(Xo-WhitePointD50[0]) > 1e-14 ||
		math.Abs(Yo-WhitePointD50[1]) > 1e-14 ||
		math.Abs(Zo-WhitePointD50[2]) > 1e-14 {
		t.Errorf("D65 white -> D50: got (%g,%g,%g), want (%g,%g,%g)",
			Xo, Yo, Zo, WhitePointD50[0], WhitePointD50[1], WhitePointD50[2])
	}
}

// TestBradfordInverse checks that the cone-response matrix and its inverse are
// inverse to each other.  Rounding the inverse to the seven decimals usually
// quoted for it moves the largest element by 4.8e-8, enough to keep an
// adaptation from reproducing the destination white point.
func TestBradfordInverse(t *testing.T) {
	got := bradford.mul(bradfordInv)
	for i := range 3 {
		for j := range 3 {
			want := 0.0
			if i == j {
				want = 1
			}
			if math.Abs(got[i*3+j]-want) > 1e-14 {
				t.Errorf("element (%d,%d) = %g, want %g", i, j, got[i*3+j], want)
			}
		}
	}
}

// TestSRGBWhiteIsPCSWhite checks that sRGB white lands on the white point of
// the Profile Connection Space.  Compositing depends on the exact value:
// blend modes such as colour burn test their operands against 0 and 1.
func TestSRGBWhiteIsPCSWhite(t *testing.T) {
	X, Y, Z := srgbToXYZ(1, 1, 1)
	if math.Abs(X-pcsWhitePoint[0]) > 1e-14 ||
		math.Abs(Y-pcsWhitePoint[1]) > 1e-14 ||
		math.Abs(Z-pcsWhitePoint[2]) > 1e-14 {
		t.Errorf("sRGB white -> (%g, %g, %g), want %v", X, Y, Z, pcsWhitePoint)
	}

	// the same value reached through the colour space
	X, Y, Z = SpaceDeviceRGB.ToXYZ([]float64{1, 1, 1}, &icc.Workspace{})
	if math.Abs(X-pcsWhitePoint[0]) > 1e-14 ||
		math.Abs(Y-pcsWhitePoint[1]) > 1e-14 ||
		math.Abs(Z-pcsWhitePoint[2]) > 1e-14 {
		t.Errorf("DeviceRGB white -> (%g, %g, %g), want %v", X, Y, Z, pcsWhitePoint)
	}
}

// TestSRGBMatrixRowSums checks the property TestSRGBWhiteIsPCSWhite relies on:
// the rows of the conversion matrix add up to the white point.
func TestSRGBMatrixRowSums(t *testing.T) {
	for i := range 3 {
		sum := srgbToPCS[i*3] + srgbToPCS[i*3+1] + srgbToPCS[i*3+2]
		if math.Abs(sum-pcsWhitePoint[i]) > 1e-14 {
			t.Errorf("row %d sums to %g, want %g", i, sum, pcsWhitePoint[i])
		}
	}
}

// TestSRGBPrimariesMatchPublishedMatrix compares the derived sRGB primaries,
// before chromatic adaptation, against the sRGB to XYZ (D65) matrix as it is
// published.  This is what justifies taking [srgbWhitePoint] from the
// tabulated D65 tristimulus values rather than from the chromaticity pair the
// sRGB standard quotes: substituting [WhitePointD65] here moves the matrix
// 2.3e-4 away from the published values, well outside their rounding.
func TestSRGBPrimariesMatchPublishedMatrix(t *testing.T) {
	published := mat3{
		0.4124564, 0.3575761, 0.1804375,
		0.2126729, 0.7151522, 0.0721750,
		0.0193339, 0.1191920, 0.9503041,
	}
	got := primaryMatrix(srgbPrimaries, srgbWhitePoint)
	for i := range got {
		if math.Abs(got[i]-published[i]) > 1e-7 {
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
		X, Y, Z := srgbToXYZ(rgb[0], rgb[1], rgb[2])
		r, g, b := xyzToSRGB(X, Y, Z)
		if math.Abs(r-rgb[0]) > 1e-12 ||
			math.Abs(g-rgb[1]) > 1e-12 ||
			math.Abs(b-rgb[2]) > 1e-12 {
			t.Errorf("%v -> (%g, %g, %g)", rgb, r, g, b)
		}
	}
}

func TestBradfordAdaptRoundTrip(t *testing.T) {
	// adapting D50->D65->D50 should be the identity
	X, Y, Z := 0.3, 0.4, 0.2
	X2, Y2, Z2 := bradfordMatrix(WhitePointD50, WhitePointD65).apply(X, Y, Z)
	X3, Y3, Z3 := bradfordMatrix(WhitePointD65, WhitePointD50).apply(X2, Y2, Z2)
	if math.Abs(X3-X) > 1e-7 || math.Abs(Y3-Y) > 1e-7 || math.Abs(Z3-Z) > 1e-7 {
		t.Errorf("round-trip failed: got (%g,%g,%g), want (%g,%g,%g)",
			X3, Y3, Z3, X, Y, Z)
	}
}

// TestAdaptationCacheFollowsWhitePoint checks that the cached adaptation is
// tied to the white point it was derived from, so that a colour space built as
// a struct literal, or one whose white point is changed after the fact, still
// converts against the right white.
func TestAdaptationCacheFollowsWhitePoint(t *testing.T) {
	var c paramCache[[3]float64, adaptation]

	d65 := [3]float64{WhitePointD65[0], WhitePointD65[1], WhitePointD65[2]}
	first := c.get(d65, newAdaptation)
	if first != c.get(d65, newAdaptation) {
		t.Error("repeated lookup rebuilt the adaptation")
	}

	d50 := [3]float64{WhitePointD50[0], WhitePointD50[1], WhitePointD50[2]}
	if second := c.get(d50, newAdaptation); second == first {
		t.Fatal("changed white point reused the cached adaptation")
	}

	want := bradfordMatrix(WhitePointD50, pcsWhitePoint)
	if got := c.get(d50, newAdaptation).value.toPCS; got != want {
		t.Errorf("toPCS = %v, want %v", got, want)
	}
}

// TestPCSWhitePointAdaptsExactly checks that a colour space whose white point
// is the PCS white point converts without any rounding.  Real files commonly
// declare Lab and CalRGB spaces with exactly this white point.
func TestPCSWhitePointAdaptsExactly(t *testing.T) {
	s, err := Lab(pcsWhitePoint, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	a := s.adapt.get(s.WhitePoint, newAdaptation).value
	if a.toPCS != identity3 || a.fromPCS != identity3 {
		t.Errorf("adaptation is not the identity: %v, %v", a.toPCS, a.fromPCS)
	}

	X, Y, Z := 0.3, 0.4, 0.2
	if Xo, Yo, Zo := a.toPCS.apply(X, Y, Z); Xo != X || Yo != Y || Zo != Z {
		t.Errorf("got (%g,%g,%g), want (%g,%g,%g)", Xo, Yo, Zo, X, Y, Z)
	}
}

func TestCalGrayD65White(t *testing.T) {
	// CalGray with D65 whitepoint, value=1 should produce pure white in sRGB
	s, err := CalGray(WhitePointD65, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	c := s.New(1)

	// the adaptation maps the source white onto the PCS white point exactly,
	// so no tolerance beyond floating-point rounding is needed here
	X, Y, Z := c.ToXYZ()
	if math.Abs(X-pcsWhitePoint[0]) > 1e-14 ||
		math.Abs(Y-pcsWhitePoint[1]) > 1e-14 ||
		math.Abs(Z-pcsWhitePoint[2]) > 1e-14 {
		t.Errorf("CalGray(D65, 1).ToXYZ() = (%g,%g,%g), want %v",
			X, Y, Z, pcsWhitePoint)
	}

	// RGBA should return (near-)pure white, allowing tolerance of 1 for
	// accumulated floating-point error through XYZ conversion
	r, g, b, a := c.RGBA()
	if absDiffU32(r, 0xffff) > 1 || absDiffU32(g, 0xffff) > 1 ||
		absDiffU32(b, 0xffff) > 1 || a != 0xffff {
		t.Errorf("CalGray(D65, 1).RGBA() = (%d,%d,%d,%d), want ~(65535,65535,65535,65535)",
			r, g, b, a)
	}
}

func TestCalGrayD50White(t *testing.T) {
	// CalGray with D50 whitepoint, value=1 should also produce pure white
	s, err := CalGray(WhitePointD50, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	c := s.New(1)

	r, g, b, a := c.RGBA()
	if absDiffU32(r, 0xffff) > 1 || absDiffU32(g, 0xffff) > 1 ||
		absDiffU32(b, 0xffff) > 1 || a != 0xffff {
		t.Errorf("CalGray(D50, 1).RGBA() = (%d,%d,%d,%d), want ~(65535,65535,65535,65535)",
			r, g, b, a)
	}
}

func TestCIERoundTrip(t *testing.T) {
	// CalGray round-trip: create -> ToXYZ -> FromXYZ -> same value
	s, err := CalGray(WhitePointD65, nil, 2.2)
	if err != nil {
		t.Fatal(err)
	}

	var v [1]float64
	for _, val := range []float64{0, 0.25, 0.5, 0.75, 1} {
		c := s.New(val)
		X, Y, Z := c.ToXYZ()
		s.FromXYZ(X, Y, Z, v[:], &icc.Workspace{})
		if math.Abs(v[0]-val) > 1e-6 {
			t.Errorf("CalGray round-trip for %g: got %g", val, v[0])
		}
	}
}

func TestCalRGBRoundTrip(t *testing.T) {
	s, err := CalRGB(WhitePointD65, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	c := s.New(0.3, 0.5, 0.7)
	X, Y, Z := c.ToXYZ()
	var v [3]float64
	s.FromXYZ(X, Y, Z, v[:], &icc.Workspace{})
	if math.Abs(v[0]-0.3) > 1e-6 ||
		math.Abs(v[1]-0.5) > 1e-6 ||
		math.Abs(v[2]-0.7) > 1e-6 {
		t.Errorf("CalRGB round-trip: got %v, want [0.3, 0.5, 0.7]", v)
	}
}

func TestLabRoundTrip(t *testing.T) {
	s, err := Lab(WhitePointD65, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	c, err := s.New(50, 20, -30)
	if err != nil {
		t.Fatal(err)
	}
	X, Y, Z := c.ToXYZ()
	var v [3]float64
	s.FromXYZ(X, Y, Z, v[:], &icc.Workspace{})
	if math.Abs(v[0]-50) > 0.01 ||
		math.Abs(v[1]-20) > 0.01 ||
		math.Abs(v[2]+30) > 0.01 {
		t.Errorf("Lab round-trip: got %v, want [50, 20, -30]", v)
	}
}

func TestToXYZRGBAConsistency(t *testing.T) {
	// for every color type: ToXYZ -> xyzToSRGB -> toUint32 should match RGBA
	calGray, _ := CalGray(WhitePointD65, nil, 1)
	calRGB, _ := CalRGB(WhitePointD65, nil, nil, nil)
	lab, _ := Lab(WhitePointD65, nil, nil)
	iccSpace, _ := ICCBased(icc.SRGBv2Profile, nil)

	testColors := []Color{
		DeviceGray(0.5),
		DeviceRGB{0.2, 0.4, 0.6},
		DeviceCMYK{0.1, 0.2, 0.3, 0.4},
		DeviceCMYK{0, 0, 0, 0},
		DeviceCMYK{1, 0, 0, 0},
		calGray.New(0.7),
		calRGB.New(0.3, 0.5, 0.8),
		mustColor(lab.New(50, 10, -20)),
		SRGB(0.4, 0.5, 0.6),
		mustColor(iccSpace.New([]float64{0.3, 0.6, 0.9})),
		mustColor(iccSpace.New([]float64{1, 1, 1})),
		colorColoredPattern{Pat: nil},
	}

	for _, c := range testColors {
		X, Y, Z := c.ToXYZ()
		rf, gf, bf := xyzToSRGB(X, Y, Z)
		r1, g1, b1 := toUint32(rf), toUint32(gf), toUint32(bf)
		r2, g2, b2, _ := c.RGBA()

		// allow tolerance of 1 for rounding
		if absDiffU32(r1, r2) > 1 || absDiffU32(g1, g2) > 1 || absDiffU32(b1, b2) > 1 {
			t.Errorf("%T: ToXYZ->sRGB = (%d,%d,%d), RGBA = (%d,%d,%d)",
				c, r1, g1, b1, r2, g2, b2)
		}
	}
}

func absDiffU32(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

// TestCalRGBRejectsNonPositiveGamma checks that the constructor enforces the
// contract its documentation states, as [CalGray] already does.  A gamma of
// zero collapses every colour onto the white point, and a negative gamma sends
// values outside the range the space can represent.
func TestCalRGBRejectsNonPositiveGamma(t *testing.T) {
	for _, gamma := range [][]float64{
		{0, 1, 1},
		{1, 0, 1},
		{1, 1, 0},
		{-2.2, -2.2, -2.2},
	} {
		if _, err := CalRGB(WhitePointD65, nil, gamma, nil); err == nil {
			t.Errorf("CalRGB accepted gamma %v", gamma)
		}
	}
	if _, err := CalRGB(WhitePointD65, nil, []float64{2.2, 2.2, 2.2}, nil); err != nil {
		t.Errorf("CalRGB rejected a valid gamma: %v", err)
	}
}

// TestGammaFastPath checks that the shortcut taken when gamma is 1 agrees with
// the general path, so that the common case stays exact.
func TestGammaFastPath(t *testing.T) {
	for _, v := range []float64{0, 0.02, 0.25, 0.5, 0.75, 1} {
		if got, want := applyGamma(v, 1), math.Pow(v, 1); got != want {
			t.Errorf("applyGamma(%g, 1) = %g, want %g", v, got, want)
		}
		if v <= 0 || v >= 1 {
			continue
		}
		if got, want := invGamma(v, 1), math.Pow(v, 1.0/1); got != want {
			t.Errorf("invGamma(%g, 1) = %g, want %g", v, got, want)
		}
	}

	// the fast path must not change what a CalRGB space converts to
	s, err := CalRGB(WhitePointD65, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ws := &icc.Workspace{}
	values := []float64{0.3, 0.5, 0.7}
	X, Y, Z := s.ToXYZ(values, ws)
	var dst [3]float64
	s.FromXYZ(X, Y, Z, dst[:], ws)
	for i, want := range values {
		if math.Abs(dst[i]-want) > 1e-12 {
			t.Errorf("round trip component %d: got %g, want %g", i, dst[i], want)
		}
	}
}

// TestCIEToXYZClampsComponents checks that the CIE-based colour spaces adjust
// out-of-range components to the nearest valid value, as §8.6.5.2, §8.6.5.3
// and §8.6.5.4 require.  Without the adjustment a negative component raised to
// a non-integer gamma yields NaN, which a content stream or an image /Decode
// array can trigger, and which then spreads through every later blend.
func TestCIEToXYZClampsComponents(t *testing.T) {
	ws := &icc.Workspace{}

	calGray, err := CalGray(WhitePointD65, nil, 2.2)
	if err != nil {
		t.Fatal(err)
	}
	calRGB, err := CalRGB(WhitePointD65, nil, []float64{2.2, 2.2, 2.2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	lab, err := Lab(WhitePointD65, nil, []float64{-100, 100, -100, 100})
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name      string
		space     Space
		out, edge []float64
	}{
		{"CalGray below", calGray, []float64{-0.5}, []float64{0}},
		{"CalGray above", calGray, []float64{1.5}, []float64{1}},
		{"CalRGB below", calRGB, []float64{-0.5, -1, -0.25}, []float64{0, 0, 0}},
		{"CalRGB above", calRGB, []float64{1.5, 2, 1.25}, []float64{1, 1, 1}},
		{"CalRGB mixed", calRGB, []float64{-0.5, 0.5, 1.5}, []float64{0, 0.5, 1}},
		{"Lab L below", lab, []float64{-20, 0, 0}, []float64{0, 0, 0}},
		{"Lab L above", lab, []float64{120, 0, 0}, []float64{100, 0, 0}},
		{"Lab ab outside", lab, []float64{50, -150, 150}, []float64{50, -100, 100}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			X, Y, Z := tc.space.ToXYZ(tc.out, ws)
			if math.IsNaN(X) || math.IsNaN(Y) || math.IsNaN(Z) {
				t.Fatalf("%v gave NaN: (%g, %g, %g)", tc.out, X, Y, Z)
			}
			// the clamped input must give exactly the same result
			Xe, Ye, Ze := tc.space.ToXYZ(tc.edge, ws)
			if X != Xe || Y != Ye || Z != Ze {
				t.Errorf("%v -> (%g, %g, %g), but %v -> (%g, %g, %g)",
					tc.out, X, Y, Z, tc.edge, Xe, Ye, Ze)
			}
		})
	}
}

// TestMatrixCacheFollowsMatrix checks that the cached inverse is tied to the
// matrix it was derived from, so that a colour space built as a struct literal,
// or one whose matrix is changed afterwards, still converts correctly.
func TestMatrixCacheFollowsMatrix(t *testing.T) {
	var c paramCache[[9]float64, matrixInverse]

	a := [9]float64{0.4, 0.2, 0.02, 0.35, 0.7, 0.11, 0.18, 0.07, 0.95}
	first := c.get(a, newMatrixInverse)
	if !first.value.ok {
		t.Fatal("matrix reported as singular")
	}
	if first != c.get(a, newMatrixInverse) {
		t.Error("repeated lookup rebuilt the inverse")
	}

	b := [9]float64{1, 0, 0, 0, 1, 0, 0, 0, 1}
	if second := c.get(b, newMatrixInverse); second == first {
		t.Fatal("changed matrix reused the cached inverse")
	}

	// a matrix which cannot be inverted is reported, not cached as garbage
	singular := [9]float64{1, 0, 0, 1, 0, 0, 1, 0, 0}
	if c.get(singular, newMatrixInverse).value.ok {
		t.Error("singular matrix reported as invertible")
	}
}

// TestCalRGBMatrixInverse checks the inversion against the definition, for a
// matrix that is not the identity, so that the column-major handling is not
// simply symmetric by accident.
func TestCalRGBMatrixInverse(t *testing.T) {
	matrix := []float64{0.4, 0.2, 0.02, 0.35, 0.7, 0.11, 0.18, 0.07, 0.95}
	s, err := CalRGB(WhitePointD65, nil, nil, matrix)
	if err != nil {
		t.Fatal(err)
	}

	ws := &icc.Workspace{}
	var dst [3]float64
	for _, abc := range [][3]float64{
		{0.3, 0.5, 0.7}, {1, 0, 0}, {0, 1, 0}, {0, 0, 1}, {0.9, 0.9, 0.9},
	} {
		X, Y, Z := s.ToXYZ(abc[:], ws)
		s.FromXYZ(X, Y, Z, dst[:], ws)
		for i := range 3 {
			if math.Abs(dst[i]-abc[i]) > 1e-12 {
				t.Errorf("%v -> XYZ -> %v", abc, dst)
				break
			}
		}
	}

	// A singular matrix yields black rather than NaN or an infinity.  Neither
	// [CalRGB] nor the read path produces such a space, so the guard covers
	// the remaining route: a Matrix field assigned after the space was built.
	sing := &SpaceCalRGB{
		WhitePoint: [3]float64{WhitePointD65[0], WhitePointD65[1], WhitePointD65[2]},
		Gamma:      [3]float64{1, 1, 1},
		Matrix:     [9]float64{1, 0, 0, 1, 0, 0, 1, 0, 0},
	}
	sing.FromXYZ(0.3, 0.4, 0.2, dst[:], ws)
	if dst != [3]float64{0, 0, 0} {
		t.Errorf("singular matrix gave %v, want black", dst)
	}
}
