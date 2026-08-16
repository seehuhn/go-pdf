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

package fallback

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf/graphics/color"
)

// TestGetDarkLightCol pins the two shades a 3-D link border is drawn with.
// The expected values were recorded from the previous implementation, which
// used its own copy of the L*a*b* conversions; they must not change now that
// the conversions run through the color package.
func TestGetDarkLightCol(t *testing.T) {
	for _, tc := range []struct {
		name        string
		col         color.Color
		dark, light color.Color
	}{
		{"gray black", color.DeviceGray(0),
			color.DeviceGray(0), color.DeviceGray(0.19)},
		{"gray near-black", color.DeviceGray(0.02),
			color.DeviceGray(0), color.DeviceGray(0.19)},
		{"gray mid", color.DeviceGray(0.5),
			color.DeviceGray(0.31), color.DeviceGray(0.71)},
		{"gray light", color.DeviceGray(0.75),
			color.DeviceGray(0.54), color.DeviceGray(0.97)},
		{"gray white", color.DeviceGray(1),
			color.DeviceGray(0.78), color.DeviceGray(1)},

		{"rgb lapis", color.DeviceRGB{0.141, 0.353, 0.620},
			color.DeviceRGB{0, 0.18, 0.42}, color.DeviceRGB{0.38, 0.55, 0.84}},
		{"rgb red", color.DeviceRGB{1, 0, 0},
			color.DeviceRGB{0.74, 0, 0}, color.DeviceRGB{1, 0.38, 0.22}},
		{"rgb black", color.DeviceRGB{0, 0, 0},
			color.DeviceRGB{0, 0, 0}, color.DeviceRGB{0.19, 0.19, 0.19}},
		{"rgb white", color.DeviceRGB{1, 1, 1},
			color.DeviceRGB{0.78, 0.78, 0.78}, color.DeviceRGB{1, 1, 1}},
		{"rgb green", color.DeviceRGB{0.2, 0.9, 0.4},
			color.DeviceRGB{0, 0.68, 0.2}, color.DeviceRGB{0.48, 1, 0.6}},

		{"cmyk none", color.DeviceCMYK{0, 0, 0, 0},
			color.DeviceCMYK{0, 0, 0, 0.22}, color.DeviceCMYK{0, 0, 0, 0}},
		{"cmyk half black", color.DeviceCMYK{0, 0, 0, 0.5},
			color.DeviceCMYK{0, 0, 0, 0.69}, color.DeviceCMYK{0, 0, 0, 0.29}},
		{"cmyk mixed", color.DeviceCMYK{0.2, 0.4, 0.6, 0.1},
			color.DeviceCMYK{0, 0.32, 0.65, 0.5}, color.DeviceCMYK{0, 0.21, 0.41, 0.05}},
		{"cmyk cyan", color.DeviceCMYK{1, 0, 0, 0},
			color.DeviceCMYK{1, 0, 0, 0.12}, color.DeviceCMYK{0.7, 0, 0, 0}},
		{"cmyk black", color.DeviceCMYK{0, 0, 0, 1},
			color.DeviceCMYK{0, 0, 0, 1}, color.DeviceCMYK{0, 0, 0, 0.81}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dark, light := getDarkLightCol(tc.col)
			if diff := cmp.Diff(tc.dark, dark); diff != "" {
				t.Errorf("dark shade (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.light, light); diff != "" {
				t.Errorf("light shade (-want +got):\n%s", diff)
			}
		})
	}
}

// TestGetDarkLightColPassesThrough checks the cases the adjustment does not
// handle: a missing colour, and a colour space other than the three device
// families, are returned unchanged rather than being converted.
func TestGetDarkLightColPassesThrough(t *testing.T) {
	if dark, light := getDarkLightCol(nil); dark != nil || light != nil {
		t.Errorf("nil colour gave (%v, %v)", dark, light)
	}

	col := color.SpaceSRGB.Default()
	dark, light := getDarkLightCol(col)
	if dark != col || light != col {
		t.Errorf("sRGB colour gave (%v, %v), want it unchanged", dark, light)
	}
}

// TestLightnessRoundTrip checks that the conversions invert each other.  The
// border shades are computed by converting to L*a*b*, moving L*, and
// converting back, so drift here would show up as a colour cast.
func TestLightnessRoundTrip(t *testing.T) {
	const tol = 1e-12

	for _, rgb := range [][3]float64{
		{0, 0, 0}, {1, 1, 1}, {1, 0, 0}, {0.2, 0.4, 0.6}, {0.5, 0.5, 0.5},
		{0.02, 0.02, 0.02}, {0.141, 0.353, 0.620},
	} {
		L, A, B := rgbToLab(rgb[0], rgb[1], rgb[2])
		r, g, b := labToRGB(L, A, B)
		if math.Abs(r-rgb[0]) > tol || math.Abs(g-rgb[1]) > tol ||
			math.Abs(b-rgb[2]) > tol {
			t.Errorf("%v -> Lab(%g, %g, %g) -> (%g, %g, %g)",
				rgb, L, A, B, r, g, b)
		}
	}

	for _, gray := range []float64{0, 0.02, 0.25, 0.5, 0.75, 1} {
		if got := lToGray(grayToL(gray)); math.Abs(got-gray) > tol {
			t.Errorf("gray %g -> L %g -> %g", gray, grayToL(gray), got)
		}
	}

	// the ink mixture need not be recovered, but the colour must be: the
	// device formula maps different mixtures to the same colour
	for _, cmyk := range [][4]float64{
		{0, 0, 0, 0}, {0, 0, 0, 1}, {1, 0, 0, 0}, {0.2, 0.4, 0.6, 0.1},
	} {
		L, A, B := cmykToLab(cmyk[0], cmyk[1], cmyk[2], cmyk[3])
		c, m, y, k := labToCMYK(L, A, B)
		L2, A2, B2 := cmykToLab(c, m, y, k)
		if math.Abs(L2-L) > 1e-10 || math.Abs(A2-A) > 1e-10 ||
			math.Abs(B2-B) > 1e-10 {
			t.Errorf("%v -> Lab(%g, %g, %g) -> Lab(%g, %g, %g)",
				cmyk, L, A, B, L2, A2, B2)
		}
	}
}

// TestWhiteIsNeutral checks that white lands on the achromatic axis at full
// lightness, which holds only if the L*a*b* reference white is the one the
// device spaces convert to.
func TestWhiteIsNeutral(t *testing.T) {
	L, A, B := rgbToLab(1, 1, 1)
	if math.Abs(L-100) > 1e-10 || math.Abs(A) > 1e-10 || math.Abs(B) > 1e-10 {
		t.Errorf("white -> Lab(%g, %g, %g), want (100, 0, 0)", L, A, B)
	}
	if L := grayToL(1); math.Abs(L-100) > 1e-10 {
		t.Errorf("white gray -> L %g, want 100", L)
	}
}

// TestLightnessIsMonotonic checks the property the border shades rely on:
// raising L* makes the colour lighter, lowering it makes it darker.
func TestLightnessIsMonotonic(t *testing.T) {
	L, A, B := rgbToLab(0.4, 0.3, 0.6)
	rd, gd, bd := labToRGB(L-15, A, B)
	rl, gl, bl := labToRGB(L+15, A, B)
	if !(rd < 0.4 && gd < 0.3 && bd < 0.6) {
		t.Errorf("darker gave (%g, %g, %g)", rd, gd, bd)
	}
	if !(rl > 0.4 && gl > 0.3 && bl > 0.6) {
		t.Errorf("lighter gave (%g, %g, %g)", rl, gl, bl)
	}
}
