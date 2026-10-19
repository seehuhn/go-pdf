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

package limits

import "testing"

func TestImageDecodedExceedsLimit(t *testing.T) {
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
			got := ImageDecodedExceedsLimit(tc.width, tc.height, tc.channels)
			if got != tc.want {
				t.Errorf("ImageDecodedExceedsLimit(%d, %d, %d) = %v, want %v",
					tc.width, tc.height, tc.channels, got, tc.want)
			}
		})
	}
}

// TestImageDecodedExceedsLimitBoundary checks the admissible pixel count
// against MaxImageDecodedBytes/4 directly: w*h*channels*4 <= 2 GiB must
// admit, and exceeding it must reject.
func TestImageDecodedExceedsLimitBoundary(t *testing.T) {
	const channels = 4
	maxElems := int64(MaxImageDecodedBytes) / 4 / channels // per-channel elements admitted

	w := int(maxElems)
	h := 1
	if ImageDecodedExceedsLimit(w, h, channels) {
		t.Errorf("ImageDecodedExceedsLimit(%d, %d, %d) = true, want false (at boundary)", w, h, channels)
	}

	w++
	if !ImageDecodedExceedsLimit(w, h, channels) {
		t.Errorf("ImageDecodedExceedsLimit(%d, %d, %d) = false, want true (just over boundary)", w, h, channels)
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

func TestShadingBudget(t *testing.T) {
	cases := []struct {
		name   string
		rawLen int64
		want   int64
	}{
		{"negative", -1, StreamBudgetBase},
		{"zero", 0, StreamBudgetBase},
		{"small", 1000, StreamBudgetBase + MaxShadingExpansion*1000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShadingBudget(tc.rawLen); got != tc.want {
				t.Errorf("ShadingBudget(%d) = %d, want %d", tc.rawLen, got, tc.want)
			}
		})
	}

	// the floor admits small inputs and the bound grows with the input size
	if ShadingBudget(0) != StreamBudgetBase {
		t.Error("budget floor is not StreamBudgetBase")
	}
	if ShadingBudget(1<<20) <= ShadingBudget(0) {
		t.Error("bound does not grow with input size")
	}
}

func TestJPXBudget(t *testing.T) {
	rgb := []int{8, 8, 8}
	rgbPerPixel := int64(3*(JPXBudgetPerSample+8*JPXBudgetPerBit) + 3*JPXBudgetPerChannel)
	cases := []struct {
		name          string
		rawLen        int64
		width, height int
		depths        []int
		channels      int
		tiles         int
		want          int64
	}{
		{"negative", -1, -5, 10, []int{-3}, -1, -1, StreamBudgetBase},
		{"zero", 0, 0, 0, nil, 0, 0, StreamBudgetBase},
		{"small", 1000, 20, 10, rgb, 3, 2, StreamBudgetBase + JPXBudgetPerByte*1000 + JPXBudgetPerTile*6 + rgbPerPixel*200},
		{"below cap", 0, 1 << 20, 1, rgb, 3, 1, StreamBudgetBase + JPXBudgetPerTile*3 + rgbPerPixel<<20},
		{"large image", 1000, MaxImageWidth, MaxImageHeight, rgb, 3, 1, JPXBudgetHardCap},
		{"many components", 0, 1 << 10, 1 << 10, make([]int, 1<<14), 1 << 14, 1, JPXBudgetHardCap},
		{"many tiles", 0, 1, 1, make([]int, 1<<14), 1, 65535, JPXBudgetHardCap},
		{"large stream", 1 << 40, 1, 1, rgb, 3, 1, JPXBudgetHardCap},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := JPXBudget(tc.rawLen, tc.width, tc.height, tc.depths, tc.channels, tc.tiles)
			if got != tc.want {
				t.Errorf("JPXBudget = %d, want %d", got, tc.want)
			}
		})
	}
}
