// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package widths

import (
	"math"
	"testing"

	"seehuhn.de/go/pdf"
)

func TestCompressWidths(t *testing.T) {
	type testCase struct {
		nLeft, nRight int
		wLeft, wRight float64
		expFirstChar  int
		expLastChar   int
		expMissing    pdf.Integer
	}
	cases := []testCase{
		{ // case 0: no compression
			nLeft:        0,
			nRight:       0,
			wLeft:        0,
			wRight:       0,
			expFirstChar: 0,
			expLastChar:  255,
			expMissing:   0,
		},
		{ // case 1: remove both sides
			nLeft:        10,
			nRight:       10,
			wLeft:        0,
			wRight:       0,
			expFirstChar: 10,
			expLastChar:  245,
			expMissing:   0,
		},
		{ // case 2: remove right
			nLeft:        10,
			nRight:       11,
			wLeft:        2,
			wRight:       4,
			expFirstChar: 0,
			expLastChar:  244,
			expMissing:   4,
		},
		{ // case 3: remove left
			nLeft:        11,
			nRight:       10,
			wLeft:        2,
			wRight:       4,
			expFirstChar: 11,
			expLastChar:  255,
			expMissing:   2,
		},
		{ // case 4: more on left, but cheaper on right
			nLeft:        11,
			nRight:       10,
			wLeft:        2,
			wRight:       0,
			expFirstChar: 0,
			expLastChar:  245,
			expMissing:   0,
		},
	}
	ww := make([]float64, 256)
	for k, c := range cases {
		for i := 0; i < 256; i++ {
			switch {
			case i < c.nLeft:
				ww[i] = c.wLeft
			case i >= 256-c.nRight:
				ww[i] = c.wRight
			default:
				ww[i] = 600 + 2*float64(i)
			}
		}
		info := EncodeSimple(ww)

		for i := 0; i < 256; i++ {
			var w pdf.Object = pdf.Number(info.MissingWidth)
			if i >= int(info.FirstChar) && i <= int(info.LastChar) {
				w = info.Widths[i-int(info.FirstChar)]
			}
			if math.Abs(float64(w.(pdf.Number)-pdf.Number(ww[i]))) > 1e-6 {
				t.Errorf("case %d: got w[%d] = %d, want %d (L=%d, R=%d, D=%f)",
					k, i, w, int(ww[i]),
					info.FirstChar, info.LastChar, info.MissingWidth)
			}
		}
	}
}
