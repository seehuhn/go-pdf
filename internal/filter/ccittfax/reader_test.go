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

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSimpleRead(t *testing.T) {
	// A single row of 8 white pixels, with an EOL marker.
	withEOL := []byte{0b10011_000, 0b00000000, 0b10000000}

	// A single row of 8 white pixels, without an EOL marker.
	withoutEOL := []byte{0b10011_000}

	for _, blackIs1 := range []bool{false, true} {
		var allWhite byte
		if blackIs1 {
			allWhite = 0x00
		} else {
			allWhite = 0xFF
		}
		for _, eolFlag := range []bool{false, true} {
			for _, data := range [][]byte{withEOL, withoutEOL} {
				t.Run(fmt.Sprintf("s-%t-%t-%t", blackIs1, len(data) > 1, eolFlag),
					func(t *testing.T) {
						p := &Params{
							Columns:   8,
							K:         0,
							EndOfLine: eolFlag,
							BlackIs1:  blackIs1,
						}

						r := NewReader(bytes.NewReader(data), p)
						data, err := io.ReadAll(r)
						if err != nil {
							t.Fatalf("unexpected error: %v", err)
						}

						expected := []byte{allWhite} // one row, eight columns, all white
						if d := cmp.Diff(expected, data); d != "" {
							t.Fatalf("unexpected output: %s", d)
						}
					})
			}
		}
	}
}

func TestFillRowBits(t *testing.T) {
	cases := []struct {
		name       string
		start      int
		end        int
		fill       bool
		columns    int
		wantOutput []byte
	}{
		{
			name:       "all white pixels",
			start:      0,
			end:        8,
			fill:       false, // white pixels
			columns:    8,
			wantOutput: []byte{0x00},
		},
		{
			name:       "all black pixels",
			start:      0,
			end:        8,
			fill:       true, // black pixels
			columns:    8,
			wantOutput: []byte{0xFF},
		},
		{
			name:       "partial black run",
			start:      2,
			end:        6,
			fill:       true, // black pixels
			columns:    8,
			wantOutput: []byte{0x3C}, // 00111100
		},
		{
			name:       "zero-length run",
			start:      0,
			end:        0,
			fill:       false,
			columns:    8,
			wantOutput: nil,
		},
		{
			name:       "multi-byte line",
			start:      4,
			end:        20,
			fill:       true, // black pixels
			columns:    32,
			wantOutput: []byte{0x0F, 0xFF, 0xF0},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &Reader{
				p: Params{
					Columns: tc.columns,
				},
			}

			r.fillRowBits(tc.start, tc.end, tc.fill)

			if d := cmp.Diff(tc.wantOutput, r.line); d != "" {
				t.Fatalf("unexpected output: %s", d)
			}
		})
	}
}
