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
