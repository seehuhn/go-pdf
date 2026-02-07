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
	Xo, Yo, Zo := bradfordAdapt(X, Y, Z, WhitePointD65, WhitePointD65)
	if math.Abs(Xo-X) > 1e-10 || math.Abs(Yo-Y) > 1e-10 || math.Abs(Zo-Z) > 1e-10 {
		t.Errorf("identity adaptation failed: got (%g,%g,%g), want (%g,%g,%g)",
			Xo, Yo, Zo, X, Y, Z)
	}
}

func TestBradfordAdaptD65White(t *testing.T) {
	// D65 white adapted to D50 should give D50 white
	Xo, Yo, Zo := bradfordAdapt(
		WhitePointD65[0], WhitePointD65[1], WhitePointD65[2],
		WhitePointD65, WhitePointD50)
	if math.Abs(Xo-WhitePointD50[0]) > 1e-4 ||
		math.Abs(Yo-WhitePointD50[1]) > 1e-4 ||
		math.Abs(Zo-WhitePointD50[2]) > 1e-4 {
		t.Errorf("D65 white -> D50: got (%g,%g,%g), want (%g,%g,%g)",
			Xo, Yo, Zo, WhitePointD50[0], WhitePointD50[1], WhitePointD50[2])
	}
}

func TestBradfordAdaptRoundTrip(t *testing.T) {
	// adapting D50->D65->D50 should be the identity
	X, Y, Z := 0.3, 0.4, 0.2
	X2, Y2, Z2 := bradfordAdapt(X, Y, Z, WhitePointD50, WhitePointD65)
	X3, Y3, Z3 := bradfordAdapt(X2, Y2, Z2, WhitePointD65, WhitePointD50)
	if math.Abs(X3-X) > 1e-7 || math.Abs(Y3-Y) > 1e-7 || math.Abs(Z3-Z) > 1e-7 {
		t.Errorf("round-trip failed: got (%g,%g,%g), want (%g,%g,%g)",
			X3, Y3, Z3, X, Y, Z)
	}
}

func TestCalGrayD65White(t *testing.T) {
	// CalGray with D65 whitepoint, value=1 should produce pure white in sRGB
	s, err := CalGray(WhitePointD65, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	c := s.New(1)

	// ToXYZ should return D50 white
	X, Y, Z := c.ToXYZ()
	if math.Abs(X-WhitePointD50[0]) > 0.01 ||
		math.Abs(Y-WhitePointD50[1]) > 0.01 ||
		math.Abs(Z-WhitePointD50[2]) > 0.01 {
		t.Errorf("CalGray(D65, 1).ToXYZ() = (%g,%g,%g), want D50 white (%g,%g,%g)",
			X, Y, Z, WhitePointD50[0], WhitePointD50[1], WhitePointD50[2])
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

	for _, val := range []float64{0, 0.25, 0.5, 0.75, 1} {
		c := s.New(val)
		X, Y, Z := c.ToXYZ()
		c2 := s.FromXYZ(X, Y, Z)
		cg := c2.(colorCalGray)
		if math.Abs(cg.Value-val) > 1e-6 {
			t.Errorf("CalGray round-trip for %g: got %g", val, cg.Value)
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
	c2 := s.FromXYZ(X, Y, Z)
	cr := c2.(colorCalRGB)
	if math.Abs(cr.Values[0]-0.3) > 1e-6 ||
		math.Abs(cr.Values[1]-0.5) > 1e-6 ||
		math.Abs(cr.Values[2]-0.7) > 1e-6 {
		t.Errorf("CalRGB round-trip: got %v, want [0.3, 0.5, 0.7]", cr.Values)
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
	c2 := s.FromXYZ(X, Y, Z)
	cl := c2.(colorLab)
	if math.Abs(cl.Values[0]-50) > 0.01 ||
		math.Abs(cl.Values[1]-20) > 0.01 ||
		math.Abs(cl.Values[2]+30) > 0.01 {
		t.Errorf("Lab round-trip: got %v, want [50, 20, -30]", cl.Values)
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
