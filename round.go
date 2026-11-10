// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package pdf

import (
	"math"
)

// Round rounds the float64 value x to the specified number of digits. This
// should be used for all computed coordinates and dimensions before they are
// written to a PDF file.
func Round(x float64, digits int) float64 {
	// Thanks, Alexander Ertli:
	// https://groups.google.com/g/golang-nuts/c/UfynqG8K1n0/m/PqyZfANPAgAJ
	scale := math.Pow10(digits)
	// +0 turns -0 into 0, which is written without a sign
	return math.Round(x*scale)/scale + 0
}

// Numbers are written in the shortest form which reads back as the same
// float64 value, so the values written with at most the given number of
// digits after the decimal point are the float64 values nearest to
// k/10^digits for integers k.  Floor and Ceil find the nearest such value
// on the correct side of x.  This is either x itself, or the float64 value
// nearest to the decimal x·10^digits rounded to an integer and divided by
// 10^digits, which is less than 10^-digits from x.
//
// Below maxIndex, k and 10^digits are exact, so k/scale is the float64 value
// nearest to k/10^digits.  The rounded index t is within one of the exact
// one, and the float64 spacing near x is less than 2·10^-digits, so the
// correction loops stop after a few steps and k stays exact.
//
// At or above maxIndex, 2^(e+1)·10^digits >= 2^53 for the binary exponent e
// of x, since no product 2^a·5^digits with digits >= 1 lies in
// [2^53-4, 2^53).  The float64 spacing near x is therefore at least 1 for
// digits = 0, so that x is an integer, and exceeds 10^-digits for
// digits >= 1.  Then the interval of numbers which read back as x contains
// some k/10^digits: x is at least 2^53/10^12 > 1, so a power of two is an
// integer, and otherwise the interval has the width of the spacing.  Thus x
// itself is written with at most the given number of digits after the
// decimal point.

// maxIndex bounds the scaled values handled by computing grid indices.
const maxIndex = 1<<53 - 4

// Floor rounds the float64 value x down to the specified number of digits
// after the decimal point, which must be between 0 and 12.
//
// The result is the largest float64 value y <= x which, when written to a
// PDF file, has at most the given number of digits after the decimal point.
// It satisfies x - y < 10^-digits + u/2, where u is the float64 spacing near
// y.  The term u/2 allows for the rounding of y itself and cannot be avoided
// in general.
func Floor(x float64, digits int) float64 {
	scale := math.Pow10(digits)
	t := x * scale
	if !(math.Abs(t) < maxIndex) {
		return x
	}
	k := math.Floor(t)
	for k/scale > x {
		k--
	}
	for (k+1)/scale <= x {
		k++
	}
	// +0 turns -0 into 0, which is written without a sign
	return k/scale + 0
}

// Ceil rounds the float64 value x up to the specified number of digits after
// the decimal point, which must be between 0 and 12.
//
// The result is the smallest float64 value y >= x which, when written to a
// PDF file, has at most the given number of digits after the decimal point.
// It satisfies y - x < 10^-digits + u/2, where u is the float64 spacing near
// y.  The term u/2 allows for the rounding of y itself and cannot be avoided
// in general: for x = 5e-324 and one digit, the result is the float64 value
// nearest to 0.1, which exceeds 0.1 and is the smallest value allowed.
func Ceil(x float64, digits int) float64 {
	scale := math.Pow10(digits)
	t := x * scale
	if !(math.Abs(t) < maxIndex) {
		return x
	}
	k := math.Ceil(t)
	for k/scale < x {
		k++
	}
	for (k-1)/scale >= x {
		k--
	}
	// +0 turns -0 into 0, which is written without a sign
	return k/scale + 0
}
