// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package pagelabel

import (
	"math"
	"strconv"
	"strings"

	"seehuhn.de/go/pdf/internal/limits"
)

// formatDecimal converts an integer to a decimal string.
func formatDecimal(n int) string {
	return strconv.Itoa(n)
}

// maxRomanValue is the largest value formatRoman writes as a Roman numeral.
const maxRomanValue = 3999

// maxAlphaValue is the largest value formatAlpha writes as letters.
const maxAlphaValue = 26 * limits.MaxPageLabelLength

// formatRoman converts a positive integer to a lowercase Roman numeral string.
// Values outside the range 1 to [maxRomanValue] fall back to decimal.
func formatRoman(n int) string {
	if n < 1 || n > maxRomanValue {
		return formatDecimal(n)
	}

	var buf strings.Builder
	for _, entry := range romanTable {
		for n >= entry.value {
			buf.WriteString(entry.numeral)
			n -= entry.value
		}
	}
	return buf.String()
}

var romanTable = []struct {
	value   int
	numeral string
}{
	{1000, "m"},
	{900, "cm"},
	{500, "d"},
	{400, "cd"},
	{100, "c"},
	{90, "xc"},
	{50, "l"},
	{40, "xl"},
	{10, "x"},
	{9, "ix"},
	{5, "v"},
	{4, "iv"},
	{1, "i"},
}

// formatAlpha converts a positive integer to a lowercase alphabetic label.
// Per the PDF spec, the pattern is: a–z for 1–26, aa–zz for 27–52, aaa–zzz
// for 53–78, and so on (each letter repeated).  Values outside the range
// 1 to [maxAlphaValue] fall back to decimal, which bounds the output length.
func formatAlpha(n int) string {
	if n < 1 || n > maxAlphaValue {
		return formatDecimal(n)
	}
	repeat := (n-1)/26 + 1
	letter := 'a' + rune((n-1)%26)
	return strings.Repeat(string(letter), repeat)
}

// parseDecimal reads a decimal integer.
func parseDecimal(str string) (int, bool) {
	n, err := strconv.Atoi(str)
	if err != nil {
		return 0, false
	}
	return n, true
}

// parseDecimalOutside reads a decimal integer outside the range 1 to maxValue,
// where a style writes its own numerals instead.
func parseDecimalOutside(str string, maxValue int) (int, bool) {
	n, ok := parseDecimal(str)
	if !ok || n >= 1 && n <= maxValue {
		return 0, false
	}
	return n, true
}

// parseRoman reads a Roman numeral, ignoring letter case.
//
// The numeral is read from left to right as a sequence of terms.  A term is
// either a single numeral, or a subtractive pair: a numeral followed by a
// larger one, worth the difference.  Any smaller numeral may be subtracted,
// so that "ic" reads as 99.  Each term must not exceed the single numeral
// before it, and must be smaller than the subtracted numeral of the pair
// before it.  This rejects inputs such as "vix", "ivi" and "iic".
func parseRoman(str string) (int, bool) {
	if str == "" {
		return 0, false
	}
	total := 0
	limit := math.MaxInt // largest value allowed for the next term
	for i := 0; i < len(str); i++ {
		v := romanValue(str[i])
		if v == 0 {
			return 0, false
		}
		term, next := v, v
		if i+1 < len(str) {
			if w := romanValue(str[i+1]); w > v {
				term, next = w-v, v-1
				i++
			}
		}
		if term > limit {
			return 0, false
		}
		total += term
		limit = next
	}
	return total, true
}

// romanValue returns the value of a Roman numeral letter, ignoring letter
// case, or 0 if c is not a Roman numeral.
func romanValue(c byte) int {
	switch c | 0x20 {
	case 'i':
		return 1
	case 'v':
		return 5
	case 'x':
		return 10
	case 'l':
		return 50
	case 'c':
		return 100
	case 'd':
		return 500
	case 'm':
		return 1000
	}
	return 0
}

// parseAlpha reads a string of one repeated letter, ignoring letter case.
func parseAlpha(str string) (int, bool) {
	if str == "" {
		return 0, false
	}
	first := str[0] | 0x20
	if first < 'a' || first > 'z' {
		return 0, false
	}
	for i := 1; i < len(str); i++ {
		if str[i]|0x20 != first {
			return 0, false
		}
	}
	return (len(str)-1)*26 + int(first-'a') + 1, true
}
