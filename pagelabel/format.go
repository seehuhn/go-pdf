// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package pagelabel

import (
	"strconv"
	"strings"

	"seehuhn.de/go/pdf/internal/limits"
)

// formatDecimal converts an integer to a decimal string.
func formatDecimal(n int) string {
	return strconv.Itoa(n)
}

// formatRoman converts a positive integer to a lowercase Roman numeral string.
// Values outside the range 1–3999 fall back to decimal.
func formatRoman(n int) string {
	if n < 1 || n > 3999 {
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
// for 53–78, and so on (each letter repeated).
func formatAlpha(n int) string {
	if n < 1 {
		return formatDecimal(n)
	}
	repeat := (n-1)/26 + 1
	if repeat > limits.MaxPageLabelLength {
		// amplification guard: fall back to decimal
		return formatDecimal(n)
	}
	letter := 'a' + rune((n-1)%26)
	return strings.Repeat(string(letter), repeat)
}
