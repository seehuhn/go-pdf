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

package pdf

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestRoundSimple1(t *testing.T) {
	x := Round(math.Pi, 2)
	y := 3.14
	if math.Abs(x-y) > 1e-6 {
		t.Errorf("Round(math.Pi, 2) = %g, want %g", x, y)
	}
}

func TestRoundSimple2(t *testing.T) {
	x := Round(1234, -1)
	y := 1230.0
	if math.Abs(x-y) > 1e-6 {
		t.Errorf("Round(math.Pi, 2) = %g, want %g", x, y)
	}
}

// testRound tests whether y corresponds to x rounded to the specified number
// of digits.
func testRound(t *testing.T, x, y float64, digits int) {
	t.Helper()

	eps := math.Pow10(-digits)
	if d := math.Abs(x - y); d > eps {
		t.Errorf("|x-y| = %g too large (%d digits)", d, digits)
	}

	yStr := strconv.FormatFloat(y, 'f', -1, 64)
	if digits <= 0 {
		if strings.Contains(yStr, ".") {
			t.Errorf("%q has a decimal point but should not (%d digits)", yStr, digits)
		} else if digits < 0 {
			tail := strings.Repeat("0", -digits)
			if !strings.HasSuffix(yStr, tail) {
				t.Errorf("%q does not end in %q (%d digits)",
					yStr, tail, digits)
			}
		}
	} else {
		pattern := fmt.Sprintf(`\.\d{%d}`, digits+1)
		re := regexp.MustCompile(pattern)
		if re.MatchString(yStr) {
			t.Errorf("%q has too many decimal places (%d digits)", yStr, digits)
		}
	}
}

func FuzzRound(f *testing.F) {
	f.Add(0.0)
	f.Add(1.0)
	f.Add(math.Pi)
	f.Fuzz(func(t *testing.T, x float64) {
		if !(x >= -10_000 && x <= 10_000) {
			t.Skipf("x = %g is out of range", x)
		}
		for digits := 0; digits <= 6; digits++ {
			y := Round(x, digits)
			testRound(t, x, y, digits)
		}
	})
}
