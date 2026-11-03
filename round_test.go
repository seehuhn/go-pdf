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
	"math/big"
	"math/rand/v2"
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
		t.Errorf("Round(1234, -1) = %g, want %g", x, y)
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
	checkDigits(t, y, digits)
}

// checkDigits tests whether y, written to a PDF file, has at most the
// specified number of digits after the decimal point.  For negative digits,
// it tests whether y ends in -digits zeros.
func checkDigits(t *testing.T, y float64, digits int) {
	t.Helper()

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

// testFloorCeil tests the guarantees of Floor and Ceil for x.
func testFloorCeil(t *testing.T, x float64, digits int) {
	t.Helper()

	xr := new(big.Rat).SetFloat64(x)

	lo := Floor(x, digits)
	if !(lo <= x) {
		t.Errorf("Floor(%v, %d) = %v is above x", x, digits, lo)
	} else if d := new(big.Rat).Sub(xr, new(big.Rat).SetFloat64(lo)); d.Cmp(floorCeilLimit(lo, digits)) >= 0 {
		t.Errorf("Floor(%v, %d) = %v is too far from x", x, digits, lo)
	}
	checkDigits(t, lo, digits)

	hi := Ceil(x, digits)
	if !(hi >= x) {
		t.Errorf("Ceil(%v, %d) = %v is below x", x, digits, hi)
	} else if d := new(big.Rat).Sub(new(big.Rat).SetFloat64(hi), xr); d.Cmp(floorCeilLimit(hi, digits)) >= 0 {
		t.Errorf("Ceil(%v, %d) = %v is too far from x", x, digits, hi)
	}
	checkDigits(t, hi, digits)

	if lo == x != (hi == x) {
		t.Errorf("Floor(%v, %d) = %v and Ceil = %v disagree on whether x is kept", x, digits, lo, hi)
	}
}

// floorCeilLimit returns 10^-digits + u/2, where u is the larger of the
// finite float64 spacings next to y.
func floorCeilLimit(y float64, digits int) *big.Rat {
	a := math.Abs(y)
	u := a - math.Nextafter(a, 0)
	if up := math.Nextafter(a, math.Inf(1)) - a; up > u && !math.IsInf(up, 0) {
		u = up
	}
	limit := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(digits)), nil))
	return limit.Add(limit, new(big.Rat).SetFloat64(u/2))
}

func TestFloorCeilSimple(t *testing.T) {
	tests := []struct {
		x      float64
		digits int
		floor  float64
		ceil   float64
	}{
		{math.Pi, 2, 3.14, 3.15},
		{-math.Pi, 2, -3.15, -3.14},
		{0.29, 2, 0.29, 0.29},
		{2.5, 0, 2, 3},
		{-2.5, 0, -3, -2},
		{1e-7, 3, 0, 0.001},
		{0, 5, 0, 0},
		{5e-324, 1, 0, 0.1},
	}
	for _, tt := range tests {
		if got := Floor(tt.x, tt.digits); got != tt.floor {
			t.Errorf("Floor(%v, %d) = %v, want %v", tt.x, tt.digits, got, tt.floor)
		}
		if got := Ceil(tt.x, tt.digits); got != tt.ceil {
			t.Errorf("Ceil(%v, %d) = %v, want %v", tt.x, tt.digits, got, tt.ceil)
		}
	}
}

// TestNoNegativeZero checks that results which are zero carry no sign, so
// that they are not written as "-0.".
func TestNoNegativeZero(t *testing.T) {
	for _, x := range []float64{math.Copysign(0, -1), -0.001, -1e-300} {
		if y := Round(x, 2); y != 0 || math.Signbit(y) {
			t.Errorf("Round(%v, 2) = %v, want 0", x, y)
		}
		if y := Ceil(x, 2); y != 0 || math.Signbit(y) {
			t.Errorf("Ceil(%v, 2) = %v, want 0", x, y)
		}
	}
	if y := Floor(math.Copysign(0, -1), 2); y != 0 || math.Signbit(y) {
		t.Errorf("Floor(-0, 2) = %v, want 0", y)
	}
}

// TestFloorCeilGrid checks that values on the decimal grid are kept, and
// that their float64 neighbours satisfy the guarantees.
func TestFloorCeilGrid(t *testing.T) {
	for digits := 0; digits <= 4; digits++ {
		scale := math.Pow10(digits)
		for k := -20000; k <= 20000; k++ {
			x := float64(k) / scale
			if Floor(x, digits) != x || Ceil(x, digits) != x {
				t.Fatalf("%v not kept (%d digits): Floor = %v, Ceil = %v",
					x, digits, Floor(x, digits), Ceil(x, digits))
			}
			testFloorCeil(t, math.Nextafter(x, math.Inf(-1)), digits)
			testFloorCeil(t, math.Nextafter(x, math.Inf(1)), digits)
		}
	}
}

// TestFloorCeilRandom checks the guarantees for values of widely varying
// magnitude.
func TestFloorCeilRandom(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for range 200_000 {
		x := math.Ldexp(rng.Float64(), rng.IntN(140)-80)
		if rng.IntN(2) == 0 {
			x = -x
		}
		testFloorCeil(t, x, rng.IntN(13))
	}
}

// TestFloorCeilLarge checks the guarantees near the magnitude where Floor
// and Ceil return x unchanged, and for powers of two.
func TestFloorCeilLarge(t *testing.T) {
	for digits := 0; digits <= 12; digits++ {
		for _, base := range []float64{1 << 52, 1 << 53, 1 << 54} {
			x := base / math.Pow10(digits)
			lo, hi := x, x
			for range 200 {
				testFloorCeil(t, lo, digits)
				testFloorCeil(t, hi, digits)
				testFloorCeil(t, -hi, digits)
				lo = math.Nextafter(lo, 0)
				hi = math.Nextafter(hi, math.Inf(1))
			}
		}
		for e := -60; e <= 80; e++ {
			testFloorCeil(t, math.Ldexp(1, e), digits)
			testFloorCeil(t, math.Nextafter(math.Ldexp(1, e), 0), digits)
		}
		testFloorCeil(t, math.MaxFloat64, digits)
		testFloorCeil(t, 5e-324, digits)
	}
}

func FuzzFloorCeil(f *testing.F) {
	f.Add(0.0, uint8(0))
	f.Add(math.Pi, uint8(2))
	f.Add(0.29, uint8(2))
	f.Add(1e15, uint8(12))
	f.Fuzz(func(t *testing.T, x float64, digits uint8) {
		if math.IsNaN(x) || math.IsInf(x, 0) || digits > 12 {
			t.Skip()
		}
		testFloorCeil(t, x, int(digits))
	})
}
