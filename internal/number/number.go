// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package number implements the syntax of numeric tokens in PDF files.
package number

import (
	"bytes"
	"math"
	"strconv"
)

// Kind is the type of object a numeric token represents.
type Kind int

// These are the possible results of [Parse].
const (
	Invalid Kind = iota // the token has no numeric value
	Integer
	Real
)

// Parse interprets a complete token as a number.
//
// Besides the integer and real syntax of PDF, the function accepts the two
// PostScript forms which PDF writers must not use: exponential format
// ("6.02E23"), which always gives a real, and radix format ("16#FFFE"), which
// gives an integer.
//
// For an [Integer] the value is returned in i, for a [Real] in x.  A decimal
// integer outside the range of int64 is returned as a Real.  A radix number is
// unsigned and keeps its 32-bit pattern when converted to an integer, so
// "16#FFFFFFFF" is -1.  The result is [Invalid] for a token which is not a
// number, a real outside the range of float64, and a radix number which does
// not fit 32 bits.
func Parse(tok []byte) (kind Kind, i int64, x float64) {
	if base, digits, ok := bytes.Cut(tok, []byte{'#'}); ok {
		return parseRadix(base, digits)
	}

	isInt, ok := checkDecimal(tok)
	if !ok {
		return Invalid, 0, 0
	}

	s := string(tok)
	if isInt {
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return Integer, i, 0
		}
	}
	x, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsInf(x, 0) {
		return Invalid, 0, 0
	}
	return Real, 0, x
}

// checkDecimal reports whether tok is a decimal number, optionally followed
// by an exponent, and whether it is written as an integer.
func checkDecimal(tok []byte) (isInt, ok bool) {
	k := 0
	if k < len(tok) && (tok[k] == '+' || tok[k] == '-') {
		k++
	}

	// mantissa
	isInt = true
	digits := 0
	for ; k < len(tok); k++ {
		if c := tok[k]; c == '.' && isInt {
			isInt = false
		} else if isDigit(c) {
			digits++
		} else {
			break
		}
	}
	if digits == 0 {
		return false, false
	}
	if k == len(tok) {
		return isInt, true
	}

	// exponent
	if tok[k] != 'e' && tok[k] != 'E' {
		return false, false
	}
	k++
	if k < len(tok) && (tok[k] == '+' || tok[k] == '-') {
		k++
	}
	if k == len(tok) {
		return false, false
	}
	for ; k < len(tok); k++ {
		if !isDigit(tok[k]) {
			return false, false
		}
	}
	return false, true
}

// parseRadix interprets the two parts of a token of the form base#digits.
// With an explicit base, [strconv.ParseUint] rejects signs, underscores and
// digits outside the base.
func parseRadix(base, digits []byte) (Kind, int64, float64) {
	b, err := strconv.ParseUint(string(base), 10, 8)
	if err != nil || b < 2 || b > 36 {
		return Invalid, 0, 0
	}
	v, err := strconv.ParseUint(string(digits), int(b), 32)
	if err != nil {
		return Invalid, 0, 0
	}
	return Integer, int64(int32(uint32(v))), 0
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
