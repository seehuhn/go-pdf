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
	"reflect"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
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

func TestExtractGlyphWidthsSimple(t *testing.T) {
	tests := []struct {
		name     string
		fontDict pdf.Dict
		fontDesc *font.Descriptor
		expected []float64
	}{
		{
			name: "Normal case",
			fontDict: pdf.Dict{
				"FirstChar": pdf.Integer(32),
				"Widths":    pdf.Array{pdf.Real(250), pdf.Real(300), pdf.Real(350)},
			},
			fontDesc: &font.Descriptor{MissingWidth: 100},
			expected: func() []float64 {
				res := make([]float64, 256)
				for i := range res {
					res[i] = 100
				}
				res[32] = 250
				res[33] = 300
				res[34] = 350
				return res
			}(),
		},
		{
			name: "No FontDescriptor",
			fontDict: pdf.Dict{
				"FirstChar": pdf.Integer(32),
				"Widths":    pdf.Array{pdf.Real(250), pdf.Real(300), pdf.Real(350)},
			},
			fontDesc: nil,
			expected: func() []float64 {
				res := make([]float64, 256)
				res[32] = 250
				res[33] = 300
				res[34] = 350
				return res
			}(),
		},
		{
			name: "Negative FirstChar",
			fontDict: pdf.Dict{
				"FirstChar": pdf.Integer(-2),
				"Widths":    pdf.Array{pdf.Real(100), pdf.Real(200), pdf.Real(300), pdf.Real(400)},
			},
			fontDesc: &font.Descriptor{MissingWidth: 50},
			expected: func() []float64 {
				res := make([]float64, 256)
				for i := range res {
					res[i] = 50
				}
				res[0] = 300
				res[1] = 400
				return res
			}(),
		},
		{
			name: "Malformed Widths",
			fontDict: pdf.Dict{
				"FirstChar": pdf.Integer(32),
				"Widths":    pdf.Array{pdf.Real(250), pdf.Name("Invalid"), pdf.Real(350)},
			},
			fontDesc: &font.Descriptor{MissingWidth: 100},
			expected: func() []float64 {
				res := make([]float64, 256)
				for i := range res {
					res[i] = 100
				}
				res[32] = 250
				res[34] = 350
				return res
			}(),
		},
		{
			name: "Missing FirstChar",
			fontDict: pdf.Dict{
				"Widths": pdf.Array{pdf.Real(250), pdf.Real(300), pdf.Real(350)},
			},
			fontDesc: &font.Descriptor{MissingWidth: 100},
			expected: func() []float64 {
				res := make([]float64, 256)
				for i := range res {
					res[i] = 100
				}
				res[0] = 250
				res[1] = 300
				res[2] = 350
				return res
			}(),
		},
		{
			name: "FirstChar out of bounds",
			fontDict: pdf.Dict{
				"FirstChar": pdf.Integer(300),
				"Widths":    pdf.Array{pdf.Real(250), pdf.Real(300), pdf.Real(350)},
			},
			fontDesc: &font.Descriptor{MissingWidth: 100},
			expected: func() []float64 {
				res := make([]float64, 256)
				for i := range res {
					res[i] = 100
				}
				return res
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dicts := &font.Dicts{
				FontDict:       tt.fontDict,
				FontDescriptor: tt.fontDesc,
			}
			got, err := ExtractSimple(&MockGetter{}, dicts)
			if err != nil {
				t.Errorf("ExtractGlyphWidthsSimple() error = %v", err)
				return
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ExtractGlyphWidthsSimple() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// MockGetter is a mock implementation of pdf.Getter for testing
type MockGetter struct{}

func (m *MockGetter) Get(ref pdf.Reference, canObjStm bool) (pdf.Native, error) {
	return nil, nil
}

func (m *MockGetter) GetMeta() *pdf.MetaInfo {
	return nil
}
