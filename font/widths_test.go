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
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/pdf"
)

func TestCompressWidths(t *testing.T) {
	type testCase struct {
		nLeft, nRight int
		wLeft, wRight funit.Int16
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
	ww := make([]funit.Int16, 256)
	for k, c := range cases {
		for _, u := range []uint16{500, 1000, 2000} {
			for i := 0; i < 256; i++ {
				switch {
				case i < c.nLeft:
					ww[i] = c.wLeft
				case i >= 256-c.nRight:
					ww[i] = c.wRight
				default:
					ww[i] = 600 + 2*funit.Int16(i)
				}
			}
			info := EncodeWidthsSimple(ww, u)

			for i := 0; i < 256; i++ {
				var w pdf.Object = info.MissingWidth
				if i >= int(info.FirstChar) && i <= int(info.LastChar) {
					w = info.Widths[i-int(info.FirstChar)]
				}
				if math.Abs(float64(w.(pdf.Number)-pdf.Number(ww[i])*1000/pdf.Number(u))) > 1e-6 {
					t.Errorf("case %d, u=%d: got w[%d] = %d, want %d (L=%d, R=%d, D=%f)",
						k, u, i, w, int(ww[i])*1000/int(u),
						info.FirstChar, info.LastChar, info.MissingWidth)
				}
			}
		}
	}
}

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
			pos := type1.CID(1)
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

			widths := make(map[type1.CID]float64)
			for _, w := range ww {
				widths[w.CID] = w.GlyphWidth
			}
			dw, w := EncodeWidthsComposite(widths, v)
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
// The test checks that DecodeWidthsComposite and EncodeWidthsComposite
// are inverse functions of each other.
func TestWidthsRoundTrip(t *testing.T) {
	w1 := map[type1.CID]float64{
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
	w2 := map[type1.CID]float64{
		0: 1000,
	}
	w3 := map[type1.CID]float64{
		0: 1000,
		1: 1000,
		2: 1000,
	}
	w4 := map[type1.CID]float64{
		0: 0,
		1: 800,
		2: 900,
		3: 1000,
		4: 1100,
	}
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		data := pdf.NewData(v)
		for _, wIn := range []map[type1.CID]float64{w1, w2, w3, w4} {
			dw, ww := EncodeWidthsComposite(wIn, pdf.GetVersion(data))
			wOut, err := DecodeWidthsComposite(data, ww, dw)
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
