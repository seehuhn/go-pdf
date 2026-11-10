// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cmap

import (
	"testing"

	"seehuhn.de/go/pdf/font/charcode"
)

func TestMakeSimpleToUnicode(t *testing.T) {
	tests := []struct {
		name  string
		input map[charcode.Code]string
	}{
		{
			name:  "empty_mapping",
			input: map[charcode.Code]string{},
		},
		{
			name: "single_ascii",
			input: map[charcode.Code]string{
				65: "A",
			},
		},
		{
			name: "consecutive_ascii",
			input: map[charcode.Code]string{
				65: "A",
				66: "B",
				67: "C",
			},
		},
		{
			name: "with_gaps",
			input: map[charcode.Code]string{
				65: "A",
				// gap
				70: "F",
			},
		},
		{
			name: "non_incrementing_sequence",
			input: map[charcode.Code]string{
				65: "A",
				66: "Z",
				67: "B",
			},
		},
		{
			name: "unicode_characters",
			input: map[charcode.Code]string{
				10: "A", // single
				20: "B", // start of incrementing sequence
				21: "C",
				22: "D",
				30: "X", // start of non-incrementing sequence
				31: "Z",
				32: "Y",
			},
		},
		{
			name: "boundaries",
			input: map[charcode.Code]string{
				0:   "Α",
				255: "Ω",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NewToUnicodeFile(charcode.Simple, tt.input)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := result.GetMapping()
			if err != nil {
				t.Fatal(err)
			}

			// Check that all mappings are preserved exactly
			for b := range charcode.Code(256) {
				got := decoded[b]
				want := tt.input[b]
				if got != want {
					t.Errorf("byte %d: got %q, want %q", b, got, want)
				}
			}
		})
	}
}
