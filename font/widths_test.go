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
	"bytes"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/funit"
)

func TestWidths(t *testing.T) {
	type A = pdf.Array
	type I = pdf.Integer

	cases := []struct {
		in  []funit.Int16
		dw  pdf.Integer
		out A
	}{
		// test sequence detection
		{
			in: []funit.Int16{1, 2, 3, 9, 9, 9, 9, 9, 9, 4, 5, 6},
			dw: 9,
			out: A{
				I(0), A{I(1), I(2), I(3)},
				I(9), A{I(4), I(5), I(6)},
			},
		},
		{
			in:  []funit.Int16{},
			out: nil,
		},
		{
			in: []funit.Int16{1, 1, 1, 1, 1, 2, 3, 4, 0, 0, 0, 0, 0, 0},
			out: A{
				I(0), I(4), I(1),
				I(5), A{I(2), I(3), I(4)},
			},
		},
		{
			in: []funit.Int16{2, 1, 4, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0},
			out: A{
				I(0), A{I(2), I(1), I(4)},
				I(3), I(7), I(1),
			},
		},
		{
			in: []funit.Int16{1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0},
			out: A{
				I(0), I(4), I(1),
			},
		},

		// test default widths
		{
			in: []funit.Int16{1, 0, 2, 0},
			out: A{
				I(0), A{I(1), I(0), I(2)},
			},
		},
		{
			in: []funit.Int16{0, 1, 0, 2, 0},
			out: A{
				I(1), A{I(1), I(0), I(2)},
			},
		},
		{
			in: []funit.Int16{1, 0, 0, 2},
			out: A{
				I(0), A{I(1)},
				I(3), A{I(2)},
			},
		},
		{
			in: []funit.Int16{0, 0, 0, 0, 1, 0, 0, 2, 0, 0, 0},
			out: A{
				I(4), A{I(1)},
				I(7), A{I(2)},
			},
		},
	}
	for i, test := range cases {
		DW, W := EncodeCIDWidths(test.in, 1)
		buf := &bytes.Buffer{}
		_ = W.PDF(buf)
		fmt.Println(i, buf.String())
		if DW != test.dw {
			t.Errorf("%d: wrong default width: expected %d but got %d",
				i, test.dw, DW)
		}
		if d := cmp.Diff(test.out, W); d != "" {
			t.Errorf("%d: wrong widths (-expected, +got): %s", i, d)
		}
	}
}
