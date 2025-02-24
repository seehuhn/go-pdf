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

package dict

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/mock"
	"seehuhn.de/go/postscript/cid"
)

func TestEncodeComposite(t *testing.T) {
	tests := []struct {
		name     string
		widthMap map[cid.CID]float64
		dw       float64
		expected pdf.Array
	}{
		{
			name:     "all default widths",
			widthMap: map[cid.CID]float64{1: 100, 2: 100},
			dw:       100,
			expected: nil,
		},
		{
			name:     "single non-default",
			widthMap: map[cid.CID]float64{5: 105},
			dw:       100,
			expected: pdf.Array{
				pdf.Integer(5),
				pdf.Array{pdf.Number(105)},
			},
		},
		{
			name:     "contiguous equal widths",
			widthMap: map[cid.CID]float64{1: 101, 2: 101, 3: 101},
			dw:       100,
			expected: pdf.Array{
				pdf.Integer(1),
				pdf.Integer(3),
				pdf.Number(101),
			},
		},
		{
			name:     "non contiguous equal widths",
			widthMap: map[cid.CID]float64{1: 101, 3: 101},
			dw:       100,
			expected: pdf.Array{
				pdf.Integer(1),
				pdf.Array{pdf.Number(101)},
				pdf.Integer(3),
				pdf.Array{pdf.Number(101)},
			},
		},
		{
			name:     "contiguous unequal widths",
			widthMap: map[cid.CID]float64{1: 101, 2: 102, 3: 103},
			dw:       100,
			expected: pdf.Array{
				pdf.Integer(1),
				pdf.Array{
					pdf.Number(101),
					pdf.Number(102),
					pdf.Number(103),
				},
			},
		},
		{
			name: "mixed equal and unequal widths",
			widthMap: map[cid.CID]float64{
				1: 101,
				2: 101,
				3: 101,
				4: 102,
				5: 103,
			},
			dw: 100,
			expected: pdf.Array{
				pdf.Integer(1),
				pdf.Integer(3),
				pdf.Number(101),
				pdf.Integer(4),
				pdf.Array{
					pdf.Number(102),
					pdf.Number(103),
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := encodeComposite(tc.widthMap, tc.dw)
			if !reflect.DeepEqual(res, tc.expected) {
				t.Errorf("unexpected result\nGot: %#v\nExpected: %#v", res, tc.expected)
			}
		})
	}
}

func TestWidthCompositeRoundTrip(t *testing.T) {
	in := map[cid.CID]float64{
		1:  100,
		2:  101,
		3:  101,
		4:  101,
		10: 101,
		11: 102,
		12: 103,
		15: 104,
	}
	encoded := encodeComposite(in, 0)

	out, err := decodeComposite(mock.Getter, encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if d := cmp.Diff(in, out); d != "" {
		t.Errorf("unexpected result (-want +got):\n%s", d)
	}
}
