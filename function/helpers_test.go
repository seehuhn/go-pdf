// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
