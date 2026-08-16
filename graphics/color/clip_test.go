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

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/icc"
)

func TestClipComponent(t *testing.T) {
	for _, tc := range []struct {
		name      string
		v, lo, hi float64
		want      float64
	}{
		{"inside", 0.5, 0, 1, 0.5},
		{"below", -0.5, 0, 1, 0},
		{"above", 1.5, 0, 1, 1},
		{"lower edge", 0, 0, 1, 0},
		{"upper edge", 1, 0, 1, 1},
		{"negative range", -120, -100, 100, -100},
		{"NaN", math.NaN(), -100, 100, -100},
		{"-Inf", math.Inf(-1), 0, 1, 0},
		{"+Inf", math.Inf(+1), 0, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClipComponent(tc.v, tc.lo, tc.hi); got != tc.want {
				t.Errorf("ClipComponent(%g, %g, %g) = %g, want %g",
					tc.v, tc.lo, tc.hi, got, tc.want)
			}
		})
	}
}

// TestClipComponentIsIdempotent checks the property the write side relies on:
// a clipped value is a fixed point, so "clipping would change this" is a sound
// test for "this value would not survive a round trip".
func TestClipComponentIsIdempotent(t *testing.T) {
	for _, v := range []float64{
		-1e9, -1, -0.5, 0, 0.25, 1, 2, 1e9,
		math.NaN(), math.Inf(-1), math.Inf(+1),
	} {
		once := ClipComponent(v, 0, 1)
		if twice := ClipComponent(once, 0, 1); twice != once {
			t.Errorf("clipping %g gave %g, then %g", v, once, twice)
		}
	}
}

func TestClipComponents(t *testing.T) {
	lab, err := Lab(WhitePointD65, nil, []float64{-60, 60, -70, 70})
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name   string
		space  Space
		values []float64
		want   []float64
	}{
		{
			name:   "device rgb",
			space:  DeviceRGB{}.ColorSpace(),
			values: []float64{-1, 0.5, 2},
			want:   []float64{0, 0.5, 1},
		},
		{
			// L* is bounded by the space, a* and b* by its Range entry
			name:   "lab uses per-component ranges",
			space:  lab,
			values: []float64{120, -90, 90},
			want:   []float64{100, -60, 70},
		},
		{
			name:   "NaN goes to the lower bound",
			space:  lab,
			values: []float64{math.NaN(), math.NaN(), math.NaN()},
			want:   []float64{0, -60, -70},
		},
		{
			name:   "short input",
			space:  DeviceRGB{}.ColorSpace(),
			values: []float64{-1},
			want:   []float64{0},
		},
		{
			name:   "extra components are left alone",
			space:  DeviceGray(0).ColorSpace(),
			values: []float64{-1, -1},
			want:   []float64{0, -1},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ClipComponents(tc.space, tc.values)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("wrong result (-want +got):\n%s", diff)
			}
			if &got[0] != &tc.values[0] {
				t.Error("values were not clipped in place")
			}
		})
	}
}

// TestClipComponentsDoesNotAllocate covers the reason [Space.ComponentRange]
// reports one component at a time: clipping happens once per colour operator
// while a content stream is read, so it must not allocate.
func TestClipComponentsDoesNotAllocate(t *testing.T) {
	for _, space := range testColorSpaces {
		values := make([]float64, space.Channels())
		n := testing.AllocsPerRun(100, func() {
			ClipComponents(space, values)
		})
		if n != 0 {
			t.Errorf("%s: ClipComponents made %g allocations", space.Family(), n)
		}
	}
}

// TestToXYZAlwaysFinite checks the contract documented on [Space.ToXYZ]: a
// component outside the range reported by ComponentRange is adjusted to the
// nearest value within it, so the result is always finite.  Without a
// NaN-aware clip a NaN component slipped through every space untouched.
func TestToXYZAlwaysFinite(t *testing.T) {
	ws := &icc.Workspace{}
	for _, space := range testColorSpaces {
		if space.Channels() == 0 { // colored patterns have no components
			continue
		}
		for _, bad := range []float64{
			math.NaN(), math.Inf(-1), math.Inf(+1), -1e300, 1e300,
		} {
			values := make([]float64, space.Channels())
			for i := range values {
				values[i] = bad
			}
			X, Y, Z := space.ToXYZ(values, ws)
			if math.IsNaN(X) || math.IsNaN(Y) || math.IsNaN(Z) ||
				math.IsInf(X, 0) || math.IsInf(Y, 0) || math.IsInf(Z, 0) {
				t.Errorf("%s: ToXYZ(%g) = %g, %g, %g",
					space.Family(), bad, X, Y, Z)
			}
		}
	}
}
