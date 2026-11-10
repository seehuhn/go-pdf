// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package color

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/icc"
	"seehuhn.de/go/pdf"
)

// TestIndexedLookup checks that Lookup returns the base colour values of a
// palette entry, clamping the index to the palette.
func TestIndexedLookup(t *testing.T) {
	// entry 0 differs from the default colour of DeviceCMYK, so that
	// clamping an index below 0 can be told apart from the default
	first := []float64{0.8, 0.6, 0.4, 0.2}
	last := []float64{0.2, 0.4, 0.6, 1}
	cs, err := Indexed([]Color{
		DeviceCMYK{first[0], first[1], first[2], first[3]},
		DeviceCMYK{last[0], last[1], last[2], last[3]},
	})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		idx  int
		want []float64
	}{
		{0, first}, {1, last}, {5, last}, {-1, first},
	}
	for _, c := range cases {
		got := cs.Lookup(c.idx)
		if len(got) != len(c.want) {
			t.Errorf("Lookup(%d) = %v, want %v", c.idx, got, c.want)
			continue
		}
		for i := range c.want {
			if math.Abs(got[i]-c.want[i]) > 1.0/255 {
				t.Errorf("Lookup(%d) = %v, want %v", c.idx, got, c.want)
				break
			}
		}
	}
}

// TestInitialTints checks that the initial colour of a Separation or DeviceN
// colour space has all tints equal to 1.
func TestInitialTints(t *testing.T) {
	separation, _ := Separation("spot", SpaceDeviceRGB, testTintTransform())
	deviceN, _ := DeviceN([]pdf.Name{"a", "b"}, SpaceDeviceRGB, testDeviceNTransform(), nil)

	cases := []struct {
		space Space
		want  []float64
	}{
		{separation, []float64{1}},
		{deviceN, []float64{1, 1}},
	}
	for _, c := range cases {
		got, _ := Values(c.space.Default())
		if diff := cmp.Diff(c.want, got); diff != "" {
			t.Errorf("%s default tints (-want +got):\n%s", c.space.Family(), diff)
		}
	}
}

// TestIndexedOutOfRange checks that out-of-range, infinite and NaN palette
// indices are clamped to the palette in ToXYZ and FromValues.
func TestIndexedOutOfRange(t *testing.T) {
	cols := []Color{
		DeviceRGB{1, 0, 0},
		DeviceRGB{0, 1, 0},
		DeviceRGB{0, 0, 1},
	}
	cs, err := Indexed(cols)
	if err != nil {
		t.Fatal(err)
	}
	last := float64(cs.NumCol - 1)

	cases := []struct {
		in   float64
		want int
	}{
		{math.NaN(), 0},
		{math.Inf(1), 2},
		{math.Inf(-1), 0},
		{1e300, 2},
		{-1e300, 0},
		{-0.4, 0},
		{0.5, 1},
		{1.49, 1},
		{last + 0.6, 2},
	}
	for _, c := range cases {
		ws := &icc.Workspace{}
		wantX, wantY, wantZ := cs.ToXYZ([]float64{float64(c.want)}, icc.RelativeColorimetric, ws)
		x, y, z := cs.ToXYZ([]float64{c.in}, icc.RelativeColorimetric, ws)
		if x != wantX || y != wantY || z != wantZ {
			t.Errorf("ToXYZ(%g): got (%g,%g,%g), want entry %d", c.in, x, y, z, c.want)
		}

		got := FromValues(cs, []float64{c.in}, nil)
		if got != cs.New(c.want) { // colours are comparable
			t.Errorf("FromValues(%g): got %v, want entry %d", c.in, got, c.want)
		}
	}
}
