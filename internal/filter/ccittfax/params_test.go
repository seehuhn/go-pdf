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

package ccittfax

import "testing"

func TestFindB1B2(t *testing.T) {
	type testCase struct {
		Name       string
		Columns    int
		BlackIs1   bool
		LineData   []byte
		A0         int
		CurrentBit byte // 0 or 1
		ExpectedB1 int
		ExpectedB2 int
	}

	testCases := []testCase{
		// Basic case: alternating bits, BlackIs1=false (0=black, 1=white)
		{
			Name:       "AlternatingBits_BlackIs0",
			Columns:    8,
			BlackIs1:   false,
			LineData:   []byte{0b10101010}, // alternating pattern
			A0:         -1,
			CurrentBit: 1, // white (starting from left margin)
			ExpectedB1: 1, // first black bit
			ExpectedB2: 2, // next white bit
		},

		// Basic case: alternating bits, BlackIs1=true (1=black, 0=white)
		{
			Name:       "AlternatingBits_BlackIs1",
			Columns:    8,
			BlackIs1:   true,
			LineData:   []byte{0b10101010}, // alternating pattern
			A0:         -1,
			CurrentBit: 0, // white (starting from left margin)
			ExpectedB1: 0, // first black bit
			ExpectedB2: 1, // next white bit
		},

		// Test with A0 in middle of line
		{
			Name:       "A0InMiddleOfLine",
			Columns:    8,
			BlackIs1:   false,
			LineData:   []byte{0b01100011}, // 111000_11
			A0:         2,
			CurrentBit: 1, // white
			ExpectedB1: 3, // first black bit after A0
			ExpectedB2: 6, // next white bit
		},

		// boundary case: A0 at end of line
		{
			Name:       "A0AtEndOfLine",
			Columns:    8,
			BlackIs1:   true,
			LineData:   []byte{0b11111111}, // all black
			A0:         7,
			CurrentBit: 1, // black
			ExpectedB1: 8, // no white bit found
			ExpectedB2: 8, // no subsequent black bit
		},

		// all white line
		{
			Name:       "AllWhiteLine",
			Columns:    8,
			BlackIs1:   false,
			LineData:   []byte{0b11111111}, // all white
			A0:         -1,
			CurrentBit: 1, // white
			ExpectedB1: 8, // no black bit found
			ExpectedB2: 8, // no subsequent white bit
		},

		// Test all black line
		{
			Name:       "AllBlackLine",
			Columns:    8,
			BlackIs1:   false,
			LineData:   []byte{0b00000000}, // all black (BlackIs1=false)
			A0:         -1,
			CurrentBit: 1, // white
			ExpectedB1: 0, // first black bit at position 0
			ExpectedB2: 8, // no white bit found
		},

		// CCITT-style example: typical fax line with runs
		{
			Name:       "CCITTTypicalFaxLine",
			Columns:    16,
			BlackIs1:   false,
			LineData:   []byte{0b11110000, 0b01100011}, // white run, black run, white run, black run
			A0:         -1,
			CurrentBit: 1, // white (starting from margin)
			ExpectedB1: 4, // first black bit
			ExpectedB2: 9, // next white bit
		},

		// current bit doesn't match pixel at A0
		{
			Name:       "CurrentBitMismatchAtA0",
			Columns:    8,
			BlackIs1:   false,
			LineData:   []byte{0b01010101}, // starts with black
			A0:         1,                  // position 1 is white
			CurrentBit: 0,                  // looking for black->white transition
			ExpectedB1: 3,                  // first white bit after A0
			ExpectedB2: 4,                  // next black bit
		},

		// Large column count with sparse data
		{
			Name:       "LargeColumnSparseData",
			Columns:    24,
			BlackIs1:   false,
			LineData:   []byte{0b11111111, 0b11110001, 0b11111111}, // mostly white with black bits
			A0:         6,
			CurrentBit: 1,  // white
			ExpectedB1: 12, // first black bit
			ExpectedB2: 15, // next white bit
		},

		// Edge case: single column
		{
			Name:       "SingleColumn",
			Columns:    1,
			BlackIs1:   false,
			LineData:   []byte{0b10000000}, // single white pixel
			A0:         -1,
			CurrentBit: 1, // white
			ExpectedB1: 1, // no black bit found
			ExpectedB2: 1, // no subsequent transition
		},

		// BlackIs1=true and complex pattern
		{
			Name:       "BlackIs1TrueComplexPattern",
			Columns:    12,
			BlackIs1:   true,
			LineData:   []byte{0b11100111, 0b10000000}, // with BlackIs1=true: 000_11_000_0____
			A0:         1,
			CurrentBit: 0, // white
			ExpectedB1: 5, // first black bit after A0
			ExpectedB2: 9, // next white bit
		},

		// Test where B1 and B2 are at line end
		{
			Name:       "B1B2AtLineEnd",
			Columns:    6,
			BlackIs1:   false,
			LineData:   []byte{0b11111100}, // white pixels except last two
			A0:         3,
			CurrentBit: 1, // white
			ExpectedB1: 6, // no black bit found within columns
			ExpectedB2: 6, // no subsequent white bit
		},

		// Figure 3/T.4 from the spec
		{
			// Figure 3/T.4 from the spec
			Name:       "Figure3T4",
			Columns:    18,
			BlackIs1:   true,
			LineData:   []byte{0b11111000, 0b00111000, 0b00_000000},
			A0:         3,
			CurrentBit: 0,
			ExpectedB1: 10,
			ExpectedB2: 13,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			p := Params{
				Columns:  tc.Columns,
				BlackIs1: tc.BlackIs1,
			}

			b1, b2 := p.findB1B2(tc.LineData, tc.A0, tc.CurrentBit)

			if b1 != tc.ExpectedB1 {
				t.Errorf("expected B1=%d, got B1=%d", tc.ExpectedB1, b1)
			}
			if b2 != tc.ExpectedB2 {
				t.Errorf("expected B2=%d, got B2=%d", tc.ExpectedB2, b2)
			}
		})
	}
}
