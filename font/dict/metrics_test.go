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

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
)

func TestCompositeWidthsEncode(t *testing.T) {
	for _, tc := range compositeWidthTests {
		t.Run(tc.name, func(t *testing.T) {
			res := encodeCompositeWidths(tc.widthMap)
			if !reflect.DeepEqual(res, tc.expected) {
				t.Errorf("unexpected result\nGot: %#v\nExpected: %#v", res, tc.expected)
			}
		})
	}
}

var compositeWidthTests = []struct {
	name     string
	widthMap map[cid.CID]float64
	expected pdf.Array
}{
	{
		name:     "single width",
		widthMap: map[cid.CID]float64{5: 105},
		expected: pdf.Array{
			pdf.Integer(5),
			pdf.Array{pdf.Number(105)},
		},
	},
	{
		name:     "contiguous equal widths",
		widthMap: map[cid.CID]float64{1: 101, 2: 101, 3: 101},
		expected: pdf.Array{
			pdf.Integer(1),
			pdf.Integer(3),
			pdf.Number(101),
		},
	},
	{
		name:     "non contiguous equal widths",
		widthMap: map[cid.CID]float64{1: 101, 3: 101},
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

// TestEncodeVMetricsSingleEntry tests the case of encoding a map which
// contains exactly one entry.
func TestEncodeVMetricsSingleEntry(t *testing.T) {
	metrics := map[cid.CID]VMetrics{
		cid.CID(100): {
			DeltaY: -1000,
			OffsX:  500,
			OffsY:  880,
		},
	}

	result := encodeVMetrics(metrics)

	if len(result) != 2 {
		t.Errorf("Expected 2 elements (CID + array), got %d", len(result))
	}
	if result[0] != pdf.Integer(100) {
		t.Errorf("Expected first element to be CID 100, got %v", result[0])
	}
}
