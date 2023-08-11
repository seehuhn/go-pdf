// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package font

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/pdf"
)

func TestEncodeWidths(t *testing.T) {
	type testCase struct {
		in  [][]funit.Int16
		out pdf.Object
	}
	testCases := []testCase{
		{
			in:  [][]funit.Int16{{1, 2, 3}},
			out: pdf.Array{pdf.Integer(1), pdf.Array{pdf.Integer(1), pdf.Integer(2), pdf.Integer(3)}},
		},
		{
			in:  [][]funit.Int16{{1, 1, 1, 1}},
			out: pdf.Array{pdf.Integer(1), pdf.Integer(4), pdf.Integer(1)},
		},
		{
			in: [][]funit.Int16{{1, 2, 3, 0}, {4}},
			out: pdf.Array{pdf.Integer(1), pdf.Array{pdf.Integer(1), pdf.Integer(2), pdf.Integer(3)},
				pdf.Integer(6), pdf.Array{pdf.Integer(4)}},
		},
		{
			in: [][]funit.Int16{{1, 2, 3}, {3, 3}},
			out: pdf.Array{pdf.Integer(1), pdf.Array{pdf.Integer(1), pdf.Integer(2)},
				pdf.Integer(3), pdf.Integer(6), pdf.Integer(3)},
		},
		{
			in: [][]funit.Int16{{1, 2, 2, 2, 2, 2, 3}},
			out: pdf.Array{pdf.Integer(1), pdf.Array{pdf.Integer(1)},
				pdf.Integer(2), pdf.Integer(6), pdf.Integer(2),
				pdf.Integer(7), pdf.Array{pdf.Integer(3)}},
		},
	}
	for i, test := range testCases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			var ww []CIDWidth
			pos := type1.CID(1)
			for _, run := range test.in {
				for _, w := range run {
					ww = append(ww, CIDWidth{pos, w})
					pos++
				}
				pos++
			}

			// make sure 0 is the most frequent width
			n := len(ww) + 1
			for i := 0; i < n; i++ {
				ww = append(ww, CIDWidth{pos, 0})
				pos++
			}

			dw, w := EncodeCIDWidths(ww, 1000)
			if dw != 0 {
				t.Errorf("dw=%v, want 0", dw)
			}

			if d := cmp.Diff(w, test.out); d != "" {
				t.Errorf("w mismatch (-want +got):\n%s", d)
			}
		})
	}
}
