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
	"slices"
	"testing"
)

// TestColorReportsItsSpace checks the [Color] contract that a colour reports
// the colour space it was created from.
func TestColorReportsItsSpace(t *testing.T) {
	calGray, err := CalGray(WhitePointD65, nil, 2.2)
	if err != nil {
		t.Fatal(err)
	}
	calRGB, err := CalRGB(WhitePointD65, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	lab, err := Lab(WhitePointD65, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	labColor, err := lab.New(50, 20, -30)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name  string
		color Color
		want  Space
	}{
		{"CalGray", calGray.New(0.5), calGray},
		{"CalGray default", calGray.Default(), calGray},
		{"CalRGB", calRGB.New(0.3, 0.5, 0.7), calRGB},
		{"CalRGB default", calRGB.Default(), calRGB},
		{"Lab", labColor, lab},
		{"Lab default", lab.Default(), lab},
	} {
		if got := tc.color.ColorSpace(); got != tc.want {
			t.Errorf("%s: ColorSpace() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestLabNewRangeContract checks that [SpaceLab.New] accepts exactly the
// values the colour space can represent: L* in [0, 100] and a*, b* within
// Ranges, boundaries included.
func TestLabNewRangeContract(t *testing.T) {
	s, err := Lab(WhitePointD65, nil, []float64{-50, 60, -70, 80})
	if err != nil {
		t.Fatal(err)
	}

	for _, v := range [][3]float64{
		{0, -50, -70}, {100, 60, 80}, {50, 0, 0},
	} {
		if _, err := s.New(v[0], v[1], v[2]); err != nil {
			t.Errorf("New%v rejected a representable colour: %v", v, err)
		}
	}
	for _, v := range [][3]float64{
		{-1, 0, 0}, {101, 0, 0},
		{50, -51, 0}, {50, 61, 0},
		{50, 0, -71}, {50, 0, 81},
	} {
		if _, err := s.New(v[0], v[1], v[2]); err == nil {
			t.Errorf("New%v accepted a colour outside the space", v)
		}
	}
}

// TestLabDefaultIsRepresentable checks that the default colour of a Lab space
// lies inside that space, using [SpaceLab.New] as the oracle.  Ranges need not
// contain the achromatic point, so the default cannot simply be zero.
func TestLabDefaultIsRepresentable(t *testing.T) {
	for _, ranges := range [][]float64{
		nil,
		{-100, 100, -100, 100},
		{10, 60, -70, -20}, // a* above zero, b* below
		{-60, -10, 20, 70}, // a* below zero, b* above
	} {
		s, err := Lab(WhitePointD65, nil, ranges)
		if err != nil {
			t.Fatal(err)
		}
		values, _ := Values(s.Default())
		if _, err := s.New(values[0], values[1], values[2]); err != nil {
			t.Errorf("ranges %v: Default() = %v is not representable: %v",
				ranges, values, err)
		}
	}
}

// TestCIEConstructorsRejectInvalidParameters checks that the factory functions
// refuse parameters which would produce an invalid PDF file.  Reading repairs
// such values instead, see TestExtractSpaceRepairsCIEParameters.
func TestCIEConstructorsRejectInvalidParameters(t *testing.T) {
	for _, wp := range [][]float64{
		{0, 1, 0.8249},        // X not positive
		{0.9642, 0.9, 0.8249}, // Y not 1
		{0.9642, 1, 0},        // Z not positive
		{0.9642, 1},           // too short
		nil,                   // missing
	} {
		if _, err := CalGray(wp, nil, 1); err == nil {
			t.Errorf("CalGray accepted white point %v", wp)
		}
		if _, err := CalRGB(wp, nil, nil, nil); err == nil {
			t.Errorf("CalRGB accepted white point %v", wp)
		}
		if _, err := Lab(wp, nil, nil); err == nil {
			t.Errorf("Lab accepted white point %v", wp)
		}
	}

	for _, bp := range [][]float64{{-1, 0, 0}, {0, 0}} {
		if _, err := CalGray(WhitePointD65, bp, 1); err == nil {
			t.Errorf("CalGray accepted black point %v", bp)
		}
		if _, err := CalRGB(WhitePointD65, bp, nil, nil); err == nil {
			t.Errorf("CalRGB accepted black point %v", bp)
		}
		if _, err := Lab(WhitePointD65, bp, nil); err == nil {
			t.Errorf("Lab accepted black point %v", bp)
		}
	}

	for _, ranges := range [][]float64{
		{100, -100, -100, 100}, // a* inverted
		{-100, 100, 100, -100}, // b* inverted
		{0, 0, -100, 100},      // a* empty
		{-100, 100},            // too short
	} {
		if _, err := Lab(WhitePointD65, nil, ranges); err == nil {
			t.Errorf("Lab accepted ranges %v", ranges)
		}
	}

	if _, err := CalRGB(WhitePointD65, nil, []float64{2.2}, nil); err == nil {
		t.Error("CalRGB accepted a gamma of the wrong length")
	}
	if _, err := CalRGB(WhitePointD65, nil, nil, []float64{1, 0}); err == nil {
		t.Error("CalRGB accepted a matrix of the wrong length")
	}
	if _, err := CalGray(WhitePointD65, nil, 0); err == nil {
		t.Error("CalGray accepted a gamma of zero")
	}
}

// cieParameterCorpus holds parameter values a file might contain, valid and
// not, for the tests which check that reading and writing agree about them.
var cieParameterCorpus = [][]float64{
	nil,
	{},
	{1},
	{0.9642, 1},
	{0.9642, 1, 0.8249},
	{0.9642, 1, 0.8249, 5},
	{0.95047, 1, 1.08883},
	{0, 1, 1}, {1, 1, 0}, {-1, 1, 1}, {1, 1, -1},
	{0.9642, 0.999, 0.8249},
	{0.9642, 0, 0.8249},
	{0.9642, -1, 0.8249},
	{0, 5e-324, 0},
	{1e300, 1, 1e300},
	{1.7e308, 1, 1.7e308},
	{math.Inf(1), 1, 1},
	{1, math.Inf(1), 1},
	{math.NaN(), 1, 1},
	{1, 1, math.NaN()},
	{-100, 100, -100, 100},
	{100, -100, 100, -100},
	{0, 0, -100, 100},
	{-1e300, 1e300, -30, 30},
	{1e300, 2e300, -30, 30},
	{math.Inf(-1), math.Inf(1), -30, 30},
	{math.NaN(), 100, -100, 100},
	{1, 1, 1},
	{2.2, 2.2, 2.2},
	{0, 1, 1}, {1, -2, 1},
	{math.Inf(1), 1, 1},
	{1, 0, 0, 0, 1, 0, 0, 0, 1},
	{1, 0, 0, 1, 0, 0, 1, 0, 0},
	{0.4, 0.2, 0.02, 0.35, 0.7, 0.11, 0.18, 0.07, 0.95},
	{1e308, 0, 0, 1e308, 1e308, 0, 1e308, 0, 1e308},
	{1, -1, -2.2e307, -4.4e307, -2.2e307, -4.4e307, -4.4e307, 1, -4.4e307},
	{math.Inf(1), 0, 0, 0, 1, 0, 0, 0, 1},
	{math.NaN(), 0, 0, 0, 1, 0, 0, 0, 1},
}

// TestRepairAgreesWithValidation checks the invariant which keeps reading and
// writing consistent: whatever a file contains, the repaired parameter passes
// the check the corresponding factory function applies.  Without this the
// library could write a colour space which its own reader would then repair,
// so that the file no longer describes the colours held in memory.
func TestRepairAgreesWithValidation(t *testing.T) {
	for _, x := range cieParameterCorpus {
		if got := repairWhitePoint(x); !isValidWhitePoint(got) {
			t.Errorf("repairWhitePoint(%v) = %v, which is not valid", x, got)
		}
		if got := repairBlackPoint(x); got != nil && !isValidBlackPoint(got) {
			t.Errorf("repairBlackPoint(%v) = %v, which is not valid", x, got)
		}
		if got := repairLabRanges(x); got != nil && !isValidLabRanges(got) {
			t.Errorf("repairLabRanges(%v) = %v, which is not valid", x, got)
		}
		if got := repairMatrix(x); got != nil && !isValidMatrix(got) {
			t.Errorf("repairMatrix(%v) = %v, which is not valid", x, got)
		}
		if got := repairGammaArray(slices.Clone(x)); got != nil && !isValidGammaArray(got) {
			t.Errorf("repairGammaArray(%v) = %v, which is not valid", x, got)
		}
		for _, gamma := range x {
			if got := repairGamma(gamma); !isValidGamma(got) {
				t.Errorf("repairGamma(%g) = %g, which is not valid", gamma, got)
			}
		}
	}
}

// TestRepairKeepsValidParameters checks the other half of the invariant: a
// parameter the factory functions accept is passed through unchanged, so that
// reading a file the library itself wrote never alters it.
func TestRepairKeepsValidParameters(t *testing.T) {
	same := func(a, b []float64) bool { return slices.Equal(a, b) }

	for _, x := range cieParameterCorpus {
		if isValidWhitePoint(x) {
			if got := repairWhitePoint(x); !same(got, x) {
				t.Errorf("repairWhitePoint changed a valid %v to %v", x, got)
			}
		}
		if isValidBlackPoint(x) {
			if got := repairBlackPoint(x); !same(got, x) {
				t.Errorf("repairBlackPoint changed a valid %v to %v", x, got)
			}
		}
		if isValidLabRanges(x) {
			if got := repairLabRanges(x); !same(got, x) {
				t.Errorf("repairLabRanges changed a valid %v to %v", x, got)
			}
		}
		if isValidMatrix(x) {
			if got := repairMatrix(x); !same(got, x) {
				t.Errorf("repairMatrix changed a valid %v to %v", x, got)
			}
		}
		if isValidGammaArray(x) {
			if got := repairGammaArray(slices.Clone(x)); !same(got, x) {
				t.Errorf("repairGammaArray changed a valid %v to %v", x, got)
			}
		}
	}
}

// TestFactoriesAcceptRepairedParameters closes the loop through the exported
// API: every combination of repaired parameters builds a colour space, so the
// error branches after the repair calls in [ExtractSpace] cannot fire.
func TestFactoriesAcceptRepairedParameters(t *testing.T) {
	for _, wp := range cieParameterCorpus {
		w := repairWhitePoint(wp)
		for _, other := range cieParameterCorpus {
			bp := repairBlackPoint(other)

			if _, err := CalGray(w, bp, repairGamma(1.8)); err != nil {
				t.Errorf("CalGray(%v, %v) rejected repaired parameters: %v",
					w, bp, err)
			}
			g := repairGammaArray(slices.Clone(other))
			m := repairMatrix(other)
			if _, err := CalRGB(w, bp, g, m); err != nil {
				t.Errorf("CalRGB(%v, %v, %v, %v) rejected repaired parameters: %v",
					w, bp, g, m, err)
			}
			r := repairLabRanges(other)
			if _, err := Lab(w, bp, r); err != nil {
				t.Errorf("Lab(%v, %v, %v) rejected repaired parameters: %v",
					w, bp, r, err)
			}
		}
	}
}

// TestCIEConstructorsRejectUnusableParameters checks the specific bounds which
// keep the write side in step with the read side.  Each value here is one the
// read path repairs, so accepting it would let the library write a file
// describing a colour space other than the one held in memory.
func TestCIEConstructorsRejectUnusableParameters(t *testing.T) {
	inf, nan := math.Inf(1), math.NaN()

	for _, wp := range [][]float64{
		{inf, 1, 1},           // infinite
		{1, 1, nan},           // not a number
		{1e5, 1, 1e5},         // beyond maxWhitePoint
		{1.7e308, 1, 1.7e308}, // overflows the Lab cube
	} {
		if _, err := CalGray(wp, nil, 1); err == nil {
			t.Errorf("CalGray accepted white point %v", wp)
		}
		if _, err := Lab(wp, nil, nil); err == nil {
			t.Errorf("Lab accepted white point %v", wp)
		}
	}

	if _, err := CalGray(WhitePointD65, []float64{inf, 0, 0}, 1); err == nil {
		t.Error("CalGray accepted an infinite black point")
	}
	for _, gamma := range []float64{inf, nan} {
		if _, err := CalGray(WhitePointD65, nil, gamma); err == nil {
			t.Errorf("CalGray accepted gamma %g", gamma)
		}
	}
	for _, gamma := range [][]float64{{inf, 1, 1}, {nan, nan, nan}} {
		if _, err := CalRGB(WhitePointD65, nil, gamma, nil); err == nil {
			t.Errorf("CalRGB accepted gamma %v", gamma)
		}
	}

	half := math.MaxFloat64 / 2
	for _, m := range [][]float64{
		{nan, 0, 0, 0, nan, 0, 0, 0, nan},
		{inf, 0, 0, 0, 1, 0, 0, 0, 1},
		{half, 0, 0, half, half, 0, half, 0, half}, // rows overflow
	} {
		if _, err := CalRGB(WhitePointD65, nil, nil, m); err == nil {
			t.Errorf("CalRGB accepted matrix %v", m)
		}
	}
	// a matrix which cannot be inverted describes a colour space collapsed
	// onto a plane or a line, which the read path replaces by the identity
	for _, m := range [][]float64{
		{1, 0, 0, 1, 0, 0, 1, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 0, 0},
	} {
		if _, err := CalRGB(WhitePointD65, nil, nil, m); err == nil {
			t.Errorf("CalRGB accepted singular matrix %v", m)
		}
	}

	for _, r := range [][]float64{
		{nan, nan, nan, nan},
		{math.Inf(-1), inf, math.Inf(-1), inf},
		{-1e6, 1e6, -1e6, 1e6}, // beyond maxLabRange
	} {
		if _, err := Lab(WhitePointD65, nil, r); err == nil {
			t.Errorf("Lab accepted ranges %v", r)
		}
	}
}
