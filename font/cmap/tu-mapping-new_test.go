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

package cmap

import "testing"

func TestMakeSimpleToUnicode(t *testing.T) {
	tests := []struct {
		name  string
		input map[byte]string
	}{
		{
			name:  "empty_mapping",
			input: map[byte]string{},
		},
		{
			name: "single_ascii",
			input: map[byte]string{
				65: "A",
			},
		},
		{
			name: "consecutive_ascii",
			input: map[byte]string{
				65: "A",
				66: "B",
				67: "C",
			},
		},
		{
			name: "with_gaps",
			input: map[byte]string{
				65: "A",
				// gap
				70: "F",
			},
		},
		{
			name: "non_incrementing_sequence",
			input: map[byte]string{
				65: "A",
				66: "Z",
				67: "B",
			},
		},
		{
			name: "unicode_characters",
			input: map[byte]string{
				65: "α",
				66: "β",
				67: "γ",
			},
		},
		{
			name: "mixed_patterns",
			input: map[byte]string{
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
			input: map[byte]string{
				0:   "Α",
				255: "Ω",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MakeSimpleToUnicode(tt.input)
			decoded := result.GetSimpleMapping()

			// Check that all mappings are preserved exactly
			for b := byte(0); b < 255; b++ {
				got := decoded[b]
				want := tt.input[b]
				if got != want {
					t.Errorf("byte %d: got %q, want %q", b, got, want)
				}
			}
		})
	}
}
