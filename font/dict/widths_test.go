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
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
	"seehuhn.de/go/postscript/cid"
)

func TestSimpleWidthsRoundTrip(t *testing.T) {
	// Create test width data
	defaultWidth := 400.0
	originalWidths := make([]float64, 256)
	for i := range originalWidths {
		if i >= 32 && i <= 126 { // Standard ASCII printable range
			if i%3 == 0 {
				originalWidths[i] = 500
			} else if i%3 == 1 {
				originalWidths[i] = 600
			} else {
				originalWidths[i] = 700
			}
		} else {
			originalWidths[i] = defaultWidth
		}
	}

	// Create PDF writer
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)

	// Create a font dictionary and set widths
	fontDict := pdf.Dict{}
	objects, refs := setSimpleWidths(w, fontDict, originalWidths, encoding.MacRoman, defaultWidth)
	for i, ref := range refs {
		w.Put(ref, objects[i])
	}

	// Read back the widths
	resultWidths := make([]float64, 256)
	success := getSimpleWidths(resultWidths, w, fontDict, defaultWidth)

	// Check that reading was successful
	if !success {
		t.Fatalf("getSimpleWidths failed")
	}

	// Compare original and read-back widths
	for i := 0; i < 256; i++ {
		if originalWidths[i] != resultWidths[i] {
			t.Errorf("Width mismatch at index %d: expected %f, got %f",
				i, originalWidths[i], resultWidths[i])
		}
	}

	// Verify that FirstChar and LastChar were correctly set
	firstChar, ok := fontDict["FirstChar"].(pdf.Integer)
	if !ok {
		t.Errorf("FirstChar not found in font dictionary or wrong type")
	}

	lastChar, ok := fontDict["LastChar"].(pdf.Integer)
	if !ok {
		t.Errorf("LastChar not found in font dictionary or wrong type")
	}

	ww, err := pdf.GetArray(w, fontDict["Widths"])
	if err != nil {
		t.Fatal(err)
	}
	if len(ww) != int(lastChar-firstChar+1) {
		t.Errorf("Unexpected number of widths: expected %d, got %d", lastChar-firstChar+1, len(ww))
	}

	// Check that the extracted range is appropriate
	if firstChar != 32 || lastChar != 126 {
		t.Errorf("Unexpected character range: firstChar=%d, lastChar=%d", firstChar, lastChar)
	}
}

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

func TestCompositeWidthsRoundTrip(t *testing.T) {
	for _, tc := range compositeWidthTests {
		t.Run(tc.name, func(t *testing.T) {
			in := tc.widthMap
			encoded := encodeCompositeWidths(in)
			out, err := decodeCompositeWidths(mock.Getter, encoded)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if d := cmp.Diff(in, out); d != "" {
				t.Errorf("unexpected result (-want +got):\n%s", d)
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
