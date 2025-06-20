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

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
)

func TestSimpleWidthsRoundTrip(t *testing.T) {
	// Create test width data
	defaultWidth := 400.0
	originalWidths := make([]float64, 256)
	for i := range originalWidths {
		if i >= 32 && i <= 126 { // Standard ASCII printable range
			switch i % 3 {
			case 0:
				originalWidths[i] = 500
			case 1:
				originalWidths[i] = 600
			default:
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

func TestVMetricsRoundtrip(t *testing.T) {
	testCases := []struct {
		name    string
		metrics map[cid.CID]VMetrics
	}{
		{
			name:    "missing map",
			metrics: nil,
		},
		{
			name:    "empty map",
			metrics: map[cid.CID]VMetrics{},
		},
		{
			name: "single entry",
			metrics: map[cid.CID]VMetrics{
				42: {DeltaY: -1000, OffsX: 250, OffsY: 750},
			},
		},
		{
			name: "two consecutive CIDs with different metrics",
			metrics: map[cid.CID]VMetrics{
				20: {DeltaY: -900, OffsX: 400, OffsY: 800},
				21: {DeltaY: -900, OffsX: 400, OffsY: 0},
			},
		},
		{
			name: "two consecutive CIDs with same metrics",
			metrics: map[cid.CID]VMetrics{
				20: {DeltaY: -900, OffsX: 400, OffsY: 800},
				21: {DeltaY: -900, OffsX: 400, OffsY: 800},
			},
		},
		{
			name: "consecutive CIDs with different metrics",
			metrics: map[cid.CID]VMetrics{
				10: {DeltaY: -1000, OffsX: 250, OffsY: 750},
				11: {DeltaY: -1000, OffsX: 300, OffsY: 760},
				12: {DeltaY: -1000, OffsX: 350, OffsY: 770},
			},
		},
		{
			name: "consecutive CIDs with same metrics",
			metrics: map[cid.CID]VMetrics{
				20: {DeltaY: -900, OffsX: 400, OffsY: 800},
				21: {DeltaY: -900, OffsX: 400, OffsY: 800},
				22: {DeltaY: -900, OffsX: 400, OffsY: 800},
			},
		},
		{
			name: "non-consecutive CIDs",
			metrics: map[cid.CID]VMetrics{
				30: {DeltaY: -900, OffsX: 400, OffsY: 800},
				35: {DeltaY: -950, OffsX: 450, OffsY: 850},
				40: {DeltaY: -1000, OffsX: 500, OffsY: 900},
			},
		},
		{
			name: "mixed patterns",
			metrics: map[cid.CID]VMetrics{
				50: {DeltaY: -800, OffsX: 300, OffsY: 700},
				51: {DeltaY: -800, OffsX: 300, OffsY: 700},
				52: {DeltaY: -800, OffsX: 300, OffsY: 700},
				60: {DeltaY: -900, OffsX: 400, OffsY: 800},
				61: {DeltaY: -950, OffsX: 450, OffsY: 850},
				62: {DeltaY: -1000, OffsX: 500, OffsY: 900},
				70: {DeltaY: -1100, OffsX: 550, OffsY: 950},
				80: {DeltaY: -1100, OffsX: 550, OffsY: 950},
				81: {DeltaY: -1100, OffsX: 550, OffsY: 950},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			ref := buf.Alloc()

			// Encode
			encoded := encodeVMetrics(tc.metrics)
			buf.Put(ref, encoded)

			// Decode
			decoded, err := decodeVMetrics(buf, ref)
			if err != nil {
				t.Fatal(err)
			}

			// Compare original and decoded
			if d := cmp.Diff(tc.metrics, decoded); d != "" {
				t.Error(d)
			}
		})
	}
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
