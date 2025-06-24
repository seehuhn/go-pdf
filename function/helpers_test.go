// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package function

import (
	"math"
	"testing"
)

func TestIsRange(t *testing.T) {
	type testCase struct {
		x, y  float64
		valid bool
	}

	testCases := []testCase{
		{0, 1, true},
		{1, 0, false},
		{-1, 1, true},
		{1, -1, false},
		{0, 0, true},
		{-1, 1, true},

		{math.NaN(), 1, false},
		{1, math.NaN(), false},
		{math.Inf(-1), 0, false},
		{math.Inf(-1), math.Inf(1), false},
		{0, math.Inf(1), false},
	}
	for i, tc := range testCases {
		if isRange(tc.x, tc.y) != tc.valid {
			t.Errorf("Test case %d failed: isRange(%f, %f) = %v, want %v",
				i, tc.x, tc.y, !tc.valid, tc.valid)
		}
	}
}
