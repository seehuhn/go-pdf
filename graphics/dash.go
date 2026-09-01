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

package graphics

import (
	"errors"
	"fmt"
	"math"
)

// CheckDashArray returns an error unless the given dash array can be written
// to a PDF file.  The dash lengths must be non-negative and must not all be
// zero.  An empty dash array is allowed and denotes a solid line.
//
// Use this for the dash arrays which have no phase of their own, for example
// in annotation border styles, where the phase is fixed at zero.  Use
// [CheckDashPattern] where a phase is stored alongside the array.
func CheckDashArray(pattern []float64) error {
	// The bounds are checked by accepting the valid values rather than
	// rejecting the invalid ones, so that a NaN, which fails every ordered
	// comparison, is refused too.  The bounds themselves refuse the
	// infinities, which PDF cannot represent.
	//
	// "Not all zero" is tracked with a flag rather than by summing the
	// lengths: for non-negative values the two are equivalent, since the sum
	// is at least as large as the largest element, but the flag cannot
	// overflow.
	allZero := true
	for _, x := range pattern {
		if !(x >= 0 && x <= math.MaxFloat64) {
			return fmt.Errorf("invalid dash length %g", x)
		}
		if x != 0 {
			allZero = false
		}
	}
	if len(pattern) > 0 && allZero {
		return errors.New("all dash lengths are zero")
	}
	return nil
}

// CheckDashPattern returns an error unless the given line dash pattern can be
// written to a PDF file.  The dash array must satisfy [CheckDashArray], and
// an empty dash array, which denotes a solid line, requires a zero phase.
//
// A negative phase is allowed.  It denotes a distance into the pattern
// measured backwards, and is incremented by twice the sum of the dash lengths
// until it is positive.
func CheckDashPattern(pattern []float64, phase float64) error {
	if err := CheckDashArray(pattern); err != nil {
		return err
	}

	if !(phase >= -math.MaxFloat64 && phase <= math.MaxFloat64) {
		return fmt.Errorf("invalid dash phase %g", phase)
	}
	if len(pattern) == 0 && phase != 0 {
		return errors.New("non-zero dash phase for a solid line")
	}

	return nil
}

// RepairDashArray returns a dash array equivalent to the given one, but
// satisfying the rules enforced by [CheckDashArray].  Negative dash lengths
// are snapped to zero, and an array where every length is zero is replaced by
// an empty array, denoting a solid line.
//
// This is used when reading files, so that every dash array a PDF file can
// contain can be written back out again.  Since a PDF file cannot contain an
// infinity or a NaN, such values are not repaired here; they are refused by
// [CheckDashArray] instead.
//
// The pattern slice may be modified in place.
func RepairDashArray(pattern []float64) []float64 {
	allZero := true
	for i, x := range pattern {
		if x < 0 {
			pattern[i] = 0
			continue
		}
		if x != 0 {
			allZero = false
		}
	}
	if allZero {
		return pattern[:0]
	}
	return pattern
}

// RepairDashPattern returns a line dash pattern equivalent to the given one,
// but satisfying the rules enforced by [CheckDashPattern].  The dash array is
// repaired as described for [RepairDashArray], and a solid line is given a
// zero phase.
//
// The pattern slice may be modified in place.
func RepairDashPattern(pattern []float64, phase float64) ([]float64, float64) {
	pattern = RepairDashArray(pattern)
	if len(pattern) == 0 {
		// a solid line, which requires a zero phase
		return pattern, 0
	}
	return pattern, phase
}
