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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
)

func TestEncodeWidths(t *testing.T) {
	type testCase struct {
		in  [][]float64
		out pdf.Object
	}
	testCases := []testCase{
		{
			in:  [][]float64{{1, 2, 3}},
			out: pdf.Array{pdf.Integer(1), pdf.Array{pdf.Number(1), pdf.Number(2), pdf.Number(3)}},
		},
		{
			in:  [][]float64{{1, 1, 1, 1}},
			out: pdf.Array{pdf.Integer(1), pdf.Integer(4), pdf.Number(1)},
		},
		{
			in: [][]float64{{1, 2, 3, 0}, {4}},
			out: pdf.Array{pdf.Integer(1), pdf.Array{pdf.Number(1), pdf.Number(2), pdf.Number(3)},
				pdf.Integer(6), pdf.Array{pdf.Number(4)}},
		},
		{
			in: [][]float64{{1, 2, 3}, {3, 3}},
			out: pdf.Array{pdf.Integer(1), pdf.Array{pdf.Number(1), pdf.Number(2)},
				pdf.Integer(3), pdf.Integer(6), pdf.Number(3)},
		},
		{
			in: [][]float64{{1, 2, 2, 2, 2, 2, 3}},
			out: pdf.Array{pdf.Integer(1), pdf.Array{pdf.Number(1)},
				pdf.Integer(2), pdf.Integer(6), pdf.Number(2),
				pdf.Integer(7), pdf.Array{pdf.Number(3)}},
		},
	}
	v := pdf.V2_0
	for i, test := range testCases {
		t.Run(fmt.Sprintf("%s-%d", v, i), func(t *testing.T) {
			var ww []cidWidth
			pos := cid.CID(1)
			for _, run := range test.in {
				for _, w := range run {
					ww = append(ww, cidWidth{pos, w})
					pos++
				}
				pos++
			}

			// make sure 0 is the most frequent width
			n := len(ww) + 1
			for i := 0; i < n; i++ {
				ww = append(ww, cidWidth{pos, 0})
				pos++
			}

			widths := make(map[cid.CID]float64)
			for _, w := range ww {
				widths[w.CID] = w.GlyphWidth
			}
			dw, w := EncodeComposite(widths, v)
			if dw != 0 {
				t.Errorf("dw=%v, want 0", dw)
			}

			if d := cmp.Diff(w, test.out); d != "" {
				fmt.Printf("%v\n", pdf.Format(w))
				fmt.Println(pdf.Format(test.out))
				t.Errorf("w mismatch (-want +got):\n%s", d)
			}
		})
	}
}

// TestWidthsRoundTrip tests that the widths encoding is invertible.
// The test checks that DecodeComposite and EncodeComposite
// are inverse functions of each other.
func TestWidthsRoundTrip(t *testing.T) {
	w1 := map[cid.CID]float64{
		0:  1000,
		1:  1000,
		2:  500,
		3:  1000,
		4:  1000,
		5:  1000,
		6:  1000,
		7:  800,
		8:  600,
		9:  400,
		10: 1000,
		12: 1000,
	}
	w2 := map[cid.CID]float64{
		0: 1000,
	}
	w3 := map[cid.CID]float64{
		0: 1000,
		1: 1000,
		2: 1000,
	}
	w4 := map[cid.CID]float64{
		0: 0,
		1: 800,
		2: 900,
		3: 1000,
		4: 1100,
	}
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		data := pdf.NewData(v)
		for _, wIn := range []map[cid.CID]float64{w1, w2, w3, w4} {
			dw, ww := EncodeComposite(wIn, pdf.GetVersion(data))
			wOut, err := DecodeComposite(data, ww, dw)
			if err != nil {
				t.Error(err)
				continue
			}
			for cid, expect := range wIn {
				if got, ok := wOut[cid]; ok {
					if got != expect {
						t.Errorf("got w[%d] = %f, want %f", cid, got, expect)
					}
				} else if expect != dw {
					t.Errorf("got w[%d] = %f, want %f", cid, got, expect)
				}
			}
		}
	}
}
