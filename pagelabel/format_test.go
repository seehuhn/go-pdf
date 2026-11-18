// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package pagelabel

import (
	"strings"
	"testing"

	"seehuhn.de/go/pdf/internal/limits"
)

func TestFormatDecimal(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{1, "1"},
		{10, "10"},
		{999, "999"},
	}
	for _, tc := range tests {
		got := formatDecimal(tc.n)
		if got != tc.want {
			t.Errorf("formatDecimal(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestFormatRoman(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{1, "i"},
		{2, "ii"},
		{3, "iii"},
		{4, "iv"},
		{5, "v"},
		{9, "ix"},
		{10, "x"},
		{14, "xiv"},
		{40, "xl"},
		{49, "xlix"},
		{90, "xc"},
		{99, "xcix"},
		{400, "cd"},
		{500, "d"},
		{900, "cm"},
		{1000, "m"},
		{1994, "mcmxciv"},
		{3999, "mmmcmxcix"},
		{0, "0"},         // below range: decimal fallback
		{4000, "4000"},   // above range: decimal fallback
		{12000, "12000"}, // above range: decimal fallback
	}
	for _, tc := range tests {
		got := formatRoman(tc.n)
		if got != tc.want {
			t.Errorf("formatRoman(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestFormatAlpha(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{1, "a"},
		{2, "b"},
		{26, "z"},
		{27, "aa"},
		{28, "bb"},
		{52, "zz"},
		{53, "aaa"},
		{78, "zzz"},
	}
	for _, tc := range tests {
		got := formatAlpha(tc.n)
		if got != tc.want {
			t.Errorf("formatAlpha(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// TestFormatAlphaCap guards against the page-label amplification attack: a
// large value must not produce a string longer than the cap.  At the boundary
// the formatter switches from repeated letters to a decimal fallback.
func TestFormatAlphaCap(t *testing.T) {
	limit := limits.MaxPageLabelLength

	// largest value still rendered as repeated letters (repeat == limit)
	if got := formatAlpha(maxAlphaValue); got != strings.Repeat("z", limit) {
		t.Errorf("formatAlpha(%d) = %d chars, want %d 'z'", maxAlphaValue, len(got), limit)
	}

	// one past the boundary: decimal fallback
	if got := formatAlpha(maxAlphaValue + 1); got != formatDecimal(maxAlphaValue+1) {
		t.Errorf("formatAlpha(%d) = %q, want decimal fallback", maxAlphaValue+1, got)
	}

	// a hostile /St value must not allocate a multi-megabyte string
	huge := 260_000_000
	if got := formatAlpha(huge); got != formatDecimal(huge) {
		t.Errorf("formatAlpha(%d) produced %d bytes; amplification not bounded", huge, len(got))
	}
}
