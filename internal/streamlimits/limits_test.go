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

package streamlimits

import "testing"

func TestImageDecodedFloat64ExceedsLimit(t *testing.T) {
	cases := []struct {
		name                    string
		width, height, channels int
		want                    bool
	}{
		{"zero width", 0, 100, 4, false},
		{"zero height", 100, 0, 4, false},
		{"zero channels", 100, 100, 0, false},
		{"negative width", -1, 100, 4, false},
		{"small grayscale", 1024, 1024, 1, false},
		{"4K CMYK", 3840, 2160, 4, false},
		{"DoS shape: 1x4096x500000", 1, 4096, 500_000, true},
		{"absurd dimensions", 1 << 16, 1 << 16, 32, true},
		{"just under cap", 1024, 1024, 4, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ImageDecodedFloat64ExceedsLimit(tc.width, tc.height, tc.channels)
			if got != tc.want {
				t.Errorf("ImageDecodedFloat64ExceedsLimit(%d, %d, %d) = %v, want %v",
					tc.width, tc.height, tc.channels, got, tc.want)
			}
		})
	}
}

func TestMaxXRefEntries(t *testing.T) {
	cases := []struct {
		name   string
		rawLen int64
		want   int64
	}{
		{"negative", -1, XRefEntriesBase},
		{"zero", 0, XRefEntriesBase},
		{"small", 1000, XRefEntriesBase + 32000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MaxXRefEntries(tc.rawLen); got != tc.want {
				t.Errorf("MaxXRefEntries(%d) = %d, want %d", tc.rawLen, got, tc.want)
			}
		})
	}

	// the bound grows with the input size
	if MaxXRefEntries(1<<20) <= MaxXRefEntries(0) {
		t.Error("bound does not grow with input size")
	}
}
