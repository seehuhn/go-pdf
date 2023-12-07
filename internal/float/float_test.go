// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package float

import (
	"fmt"
	"math"
	"strconv"
	"testing"
)

func TestFormat(t *testing.T) {
	type testCase struct {
		in     float64
		digits int
		out    string
	}
	testCases := []testCase{
		{0, 0, "0"},
		{1, 0, "1"},
		{-1, 0, "-1"},
		{0, 1, "0"},
		{1, 1, "1"},
		{-1, 1, "-1"},
		{0.1, 0, "0"},
		{0.1, 1, ".1"},
		{0.1, 2, ".1"},
		{0.9, 0, "1"},
		{0.9, 1, ".9"},
		{0.9, 2, ".9"},
		{0.19, 1, ".2"},
		{-0.19, 1, "-0.2"},
		{math.Pi, 0, "3"},
		{math.Pi, 1, "3.1"},
		{math.Pi, 2, "3.14"},
		{math.Pi, 4, "3.1416"},
		{math.Pi, 5, "3.14159"},
	}
	for _, tc := range testCases {
		got := Format(tc.in, tc.digits)
		if got != tc.out {
			t.Errorf("Format(%g, %d) = %q, want %q", tc.in, tc.digits, got, tc.out)
		}
	}
}

func FuzzFormat(f *testing.F) {
	f.Fuzz(func(t *testing.T, x float64, digits int) {
		// The Format function is used in the PDF writer, which only uses it
		// with `digits` in the range {0, ..., 5}.
		if digits < 0 || digits > 5 {
			return
		}
		xString := Format(x, digits)
		y, err := strconv.ParseFloat(xString, 64)
		if err != nil {
			t.Errorf("strconv.ParseFloat(%q, 64) failed: %v", xString, err)
		} else if math.Log10(x)+float64(digits) < 15 && math.Abs(x-y) > 0.500001*math.Pow10(-digits) {
			fmt.Println(math.Abs(x-y), 0.500001*math.Pow10(-digits))
			t.Errorf("Format(%g, %d) = %q", x, digits, xString)
		}
	})
}

func FuzzRound(f *testing.F) {
	f.Add(0.0, 5)
	f.Add(1.0, 5)
	f.Add(9999.9999, 5)
	f.Fuzz(func(t *testing.T, a float64, b int) {
		if b < 1 || b > 5 {
			return
		}
		s1 := Format(a, b)
		s2 := Format(Round(a, b), b)
		if s1 != s2 {
			t.Errorf("Format(%g, %d) = %q, but Round(%g, %d) = %q", a, b, s1, a, b, s2)
		}
	})
}
