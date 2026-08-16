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

// fromXYZSpace is implemented by the colour spaces which can convert in both
// directions.
type fromXYZSpace interface {
	Space
	FromXYZ(X, Y, Z float64, dst []float64, ws *icc.Workspace)
}

// TestDeviceFromXYZRoundTrip checks that a colour in a device space survives
// the trip to XYZ and back.
func TestDeviceFromXYZRoundTrip(t *testing.T) {
	cases := []struct {
		name   string
		space  fromXYZSpace
		values []float64
	}{
		{"gray black", SpaceDeviceGray, []float64{0}},
		{"gray mid", SpaceDeviceGray, []float64{0.5}},
		{"gray white", SpaceDeviceGray, []float64{1}},
		{"rgb red", SpaceDeviceRGB, []float64{1, 0, 0}},
		{"rgb mixed", SpaceDeviceRGB, []float64{0.2, 0.4, 0.6}},
		{"srgb mixed", SpaceSRGB, []float64{0.2, 0.4, 0.6}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			X, Y, Z := tc.space.ToXYZ(tc.values, &icc.Workspace{})
			got := make([]float64, tc.space.Channels())
			tc.space.FromXYZ(X, Y, Z, got, &icc.Workspace{})
			for i, want := range tc.values {
				if math.Abs(got[i]-want) > 1e-5 {
					t.Errorf("component %d: got %g, want %g", i, got[i], want)
				}
			}
		})
	}
}

// TestDeviceGrayFromXYZ checks that a chromatic colour is reduced to gray by
// the weights PDF prescribes, 0.3 red + 0.59 green + 0.11 blue, applied to the
// component values rather than to their luminance.  A colorimetric reduction
// would put red at 0.51 instead.
func TestDeviceGrayFromXYZ(t *testing.T) {
	for _, tc := range []struct {
		rgb  DeviceRGB
		want float64
	}{
		{DeviceRGB{1, 0, 0}, 0.3},
		{DeviceRGB{0, 1, 0}, 0.59},
		{DeviceRGB{0, 0, 1}, 0.11},
		{DeviceRGB{1, 1, 1}, 1},
		{DeviceRGB{0.8, 0.4, 0.2}, 0.3*0.8 + 0.59*0.4 + 0.11*0.2},
	} {
		X, Y, Z := tc.rgb.ToXYZ()
		got := make([]float64, 1)
		SpaceDeviceGray.FromXYZ(X, Y, Z, got, &icc.Workspace{})
		if math.Abs(got[0]-tc.want) > 1e-5 {
			t.Errorf("gray value of %v = %g, want %g", tc.rgb, got[0], tc.want)
		}
	}
}

// TestDeviceCMYKFromXYZ checks that the CMYK profile is used, and that the
// resulting ink mixture reproduces the requested colour.
func TestDeviceCMYKFromXYZ(t *testing.T) {
	if cmykXform() == nil {
		t.Skip("built-in CMYK profile unavailable")
	}

	orig := DeviceCMYK{0.2, 0.4, 0.6, 0.1}
	X, Y, Z := orig.ToXYZ()

	got := make([]float64, 4)
	SpaceDeviceCMYK.FromXYZ(X, Y, Z, got, &icc.Workspace{})

	X2, Y2, Z2 := SpaceDeviceCMYK.ToXYZ(got, &icc.Workspace{})
	const tol = 0.01
	if math.Abs(X2-X) > tol || math.Abs(Y2-Y) > tol || math.Abs(Z2-Z) > tol {
		t.Errorf("%v -> XYZ(%g,%g,%g) -> %v -> XYZ(%g,%g,%g)",
			orig, X, Y, Z, got, X2, Y2, Z2)
	}
}

// TestICCFallbackFromXYZ covers the conversion used when a profile cannot
// supply the reverse transform.
func TestICCFallbackFromXYZ(t *testing.T) {
	X, Y, Z := DeviceRGB{0.2, 0.4, 0.6}.ToXYZ()
	r, g, b := xyzToSRGB(X, Y, Z)

	for _, n := range []int{1, 3, 4, 2} {
		s := &SpaceICCBased{N: n}
		got := make([]float64, n)
		s.fallbackFromXYZ(X, Y, Z, got)

		var want []float64
		switch n {
		case 1:
			want = []float64{rgbToGray(r, g, b)}
		case 3:
			want = []float64{r, g, b}
		case 4:
			want = make([]float64, 4)
			rgbToCMYK(r, g, b, want)
		default:
			want = make([]float64, n)
		}
		for i := range want {
			if math.Abs(got[i]-want[i]) > 1e-9 {
				t.Errorf("N=%d component %d: got %g, want %g", n, i, got[i], want[i])
			}
		}
	}
}
