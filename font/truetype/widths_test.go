// seehuhn.de/go/pdf - support for reading and writing PDF files
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

package truetype

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"seehuhn.de/go/pdf"
)

func TestWidths(t *testing.T) {
	type A = pdf.Array
	type I = pdf.Integer

	cases := []struct {
		in  []int
		out A
	}{
		// test sequence detection
		{
			in: []int{1, 2, 3, 9, 9, 9, 9, 9, 9, 4, 5, 6},
			out: A{
				I(0), A{I(1), I(2), I(3)},
				I(3), I(8), I(9),
				I(9), A{I(4), I(5), I(6)},
			},
		},
		{
			in:  []int{},
			out: nil,
		},
		{
			in: []int{1, 1, 1, 1, 1, 2, 3, 4},
			out: A{
				I(0), I(4), I(1),
				I(5), A{I(2), I(3), I(4)},
			},
		},
		{
			in: []int{2, 1, 4, 1, 1, 1, 1, 1},
			out: A{
				I(0), A{I(2), I(1), I(4)},
				I(3), I(7), I(1),
			},
		},
		{
			in: []int{1, 1, 1, 1, 1},
			out: A{
				I(0), I(4), I(1),
			},
		},

		// test default widths
		{
			in: []int{1, 0, 2},
			out: A{
				I(0), A{I(1), I(0), I(2)},
			},
		},
		{
			in: []int{0, 1, 0, 2, 0},
			out: A{
				I(1), A{I(1), I(0), I(2)},
			},
		},
		{
			in: []int{1, 0, 0, 2},
			out: A{
				I(0), A{I(1)},
				I(3), A{I(2)},
			},
		},
		{
			in: []int{0, 0, 0, 0, 1, 0, 0, 2, 0, 0, 0},
			out: A{
				I(4), A{I(1)},
				I(7), A{I(2)},
			},
		},
	}
	for i, test := range cases {
		W := encodeWidths(test.in, 0)
		buf := &bytes.Buffer{}
		W.PDF(buf)
		fmt.Println(i, buf.String())
		if !reflect.DeepEqual(W, test.out) {
			t.Error(i, "wrong result "+buf.String())
		}
	}
}
