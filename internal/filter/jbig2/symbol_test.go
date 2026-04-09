// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package jbig2

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf/graphics/bitmap"
)

type symbolDictTestCase struct {
	name     string
	template int
	symbols  []*bitmap.Bitmap
}

var symbolDictTestCases = []symbolDictTestCase{
	{
		name:     "single_symbol",
		template: 1,
		symbols:  []*bitmap.Bitmap{makeDiagonal(8, 8)},
	},
	{
		name:     "single_height_class",
		template: 1,
		symbols: []*bitmap.Bitmap{
			makeAllZeros(6, 10),
			makeCheckerboard(8, 10),
			makeDiagonal(10, 10),
		},
	},
	{
		name:     "two_height_classes",
		template: 1,
		symbols: []*bitmap.Bitmap{
			// height class 1: height=8
			makeDiagonal(6, 8),
			makeCheckerboard(8, 8),
			// height class 2: height=12
			makeCenterBlock(10, 12),
			makeHStripes(12, 12),
		},
	},
	{
		name:     "three_height_classes",
		template: 1,
		symbols: []*bitmap.Bitmap{
			// height class 1: height=6
			makeAllZeros(4, 6),
			// height class 2: height=8
			makeDiagonal(6, 8),
			makeVStripes(8, 8),
			// height class 3: height=16
			makeCheckerboard(16, 16),
		},
	},
	{
		name:     "template0",
		template: 0,
		symbols: []*bitmap.Bitmap{
			makeCheckerboard(8, 10),
			makeDiagonal(10, 10),
		},
	},
	{
		name:     "template2",
		template: 2,
		symbols: []*bitmap.Bitmap{
			makeHStripes(8, 8),
			makeVStripes(10, 8),
		},
	},
	{
		name:     "template3",
		template: 3,
		symbols: []*bitmap.Bitmap{
			makeCenterBlock(8, 8),
			makeDiagonal(10, 8),
		},
	},
}

func symbolDictRoundTrip(t *testing.T, tc symbolDictTestCase) {
	t.Helper()

	// encode
	segData := EncodeSymbolDictSegment(tc.symbols, tc.template)

	// wrap in segment stream: SD segment + page info
	var stream []byte
	stream = WriteSegmentHeader(stream, 0, segSymbolDict, 0, nil, uint32(len(segData)))
	stream = append(stream, segData...)

	pageData := WritePageInfo(nil, 1, 1)
	stream = WriteSegmentHeader(stream, 1, segPageInfo, 1, nil, uint32(len(pageData)))
	stream = append(stream, pageData...)

	// decode
	d := &decoder{
		segments:  make(map[uint32]segmentResult),
		inputSize: len(stream),
		memBudget: 1 << 30,
	}
	if err := d.processStream(stream); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	seg, ok := d.segments[0]
	if !ok || seg.symbols == nil {
		t.Fatalf("no symbol dictionary decoded")
	}

	if len(seg.symbols) != len(tc.symbols) {
		t.Fatalf("got %d symbols, want %d", len(seg.symbols), len(tc.symbols))
	}

	for i, want := range tc.symbols {
		got := seg.symbols[i]
		if diff := cmp.Diff(want.Pix, got.Pix); diff != "" {
			t.Errorf("symbol %d (%dx%d) mismatch (-want +got):\n%s",
				i, want.Width(), want.Height(), diff)
		}
		if got.Width() != want.Width() || got.Height() != want.Height() {
			t.Errorf("symbol %d: got %dx%d, want %dx%d",
				i, got.Width(), got.Height(), want.Width(), want.Height())
		}
	}
}

func TestSymbolDictRoundTrip(t *testing.T) {
	for _, tc := range symbolDictTestCases {
		t.Run(tc.name, func(t *testing.T) {
			symbolDictRoundTrip(t, tc)
		})
	}
}

type huffRefAggTestCase struct {
	name        string
	sdrTemplate int
	symbols     []*bitmap.Bitmap
	refSymbols  []*bitmap.Bitmap
}

var huffRefAggTestCases = []huffRefAggTestCase{
	{
		name:        "single_template1",
		sdrTemplate: 1,
		symbols:     []*bitmap.Bitmap{makeCheckerboard(8, 8)},
		refSymbols:  []*bitmap.Bitmap{makeDiagonal(8, 8)},
	},
	{
		name:        "two_symbols_template1",
		sdrTemplate: 1,
		symbols: []*bitmap.Bitmap{
			makeCheckerboard(8, 8),
			makeHStripes(10, 8),
		},
		refSymbols: []*bitmap.Bitmap{
			makeDiagonal(8, 8),
			makeVStripes(10, 8),
		},
	},
	{
		name:        "two_height_classes_template0",
		sdrTemplate: 0,
		symbols: []*bitmap.Bitmap{
			// height class 1: height=8
			makeCheckerboard(6, 8),
			makeDiagonal(8, 8),
			// height class 2: height=12
			makeHStripes(10, 12),
		},
		refSymbols: []*bitmap.Bitmap{
			makeVStripes(6, 8),
			makeCenterBlock(8, 8),
			makeAllZeros(10, 12),
		},
	},
}

// buildHuffRefAggStream constructs a JBIG2 segment stream containing a
// reference SD (segment 0), a Huffman+refagg SD (segment 1), and a page info
// segment (segment 2).
func buildHuffRefAggStream(tc huffRefAggTestCase) ([]byte, error) {
	refSegData := EncodeSymbolDictSegment(tc.refSymbols, 1)
	segData, err := EncodeSymbolDictSegmentHuffRef(
		tc.symbols, tc.refSymbols, tc.sdrTemplate,
	)
	if err != nil {
		return nil, err
	}
	pageData := WritePageInfo(nil, 1, 1)

	var stream []byte
	stream = WriteSegmentHeader(stream, 0, segSymbolDict, 0, nil, uint32(len(refSegData)))
	stream = append(stream, refSegData...)
	stream = WriteSegmentHeader(stream, 1, segSymbolDict, 0, []uint32{0}, uint32(len(segData)))
	stream = append(stream, segData...)
	stream = WriteSegmentHeader(stream, 2, segPageInfo, 1, nil, uint32(len(pageData)))
	stream = append(stream, pageData...)
	return stream, nil
}

func huffRefAggRoundTrip(t *testing.T, tc huffRefAggTestCase) {
	t.Helper()

	stream, err := buildHuffRefAggStream(tc)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	d := &decoder{
		segments:  make(map[uint32]segmentResult),
		inputSize: len(stream),
		memBudget: 1 << 30,
	}
	if err := d.processStream(stream); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	seg, ok := d.segments[1]
	if !ok || seg.symbols == nil {
		t.Fatalf("no symbol dictionary decoded for segment 1")
	}

	if len(seg.symbols) != len(tc.symbols) {
		t.Fatalf("got %d symbols, want %d", len(seg.symbols), len(tc.symbols))
	}

	for i, want := range tc.symbols {
		got := seg.symbols[i]
		if got.Width() != want.Width() || got.Height() != want.Height() {
			t.Errorf("symbol %d: got %dx%d, want %dx%d",
				i, got.Width(), got.Height(), want.Width(), want.Height())
		}
		if diff := cmp.Diff(want.Pix, got.Pix); diff != "" {
			t.Errorf("symbol %d mismatch (-want +got):\n%s", i, diff)
		}
	}
}

func TestHuffRefAggSymbolDict(t *testing.T) {
	for _, tc := range huffRefAggTestCases {
		t.Run(tc.name, func(t *testing.T) {
			huffRefAggRoundTrip(t, tc)
		})
	}
}

func FuzzSymbolDictRoundTrip(f *testing.F) {
	// seed from test cases
	for _, tc := range symbolDictTestCases {
		segData := EncodeSymbolDictSegment(tc.symbols, tc.template)
		var stream []byte
		stream = WriteSegmentHeader(stream, 0, segSymbolDict, 0, nil, uint32(len(segData)))
		stream = append(stream, segData...)
		pageData := WritePageInfo(nil, 1, 1)
		stream = WriteSegmentHeader(stream, 1, segPageInfo, 1, nil, uint32(len(pageData)))
		stream = append(stream, pageData...)
		f.Add(stream)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// first read
		d1 := &decoder{
			segments:  make(map[uint32]segmentResult),
			inputSize: len(data),
		}
		if err := d1.processStream(data); err != nil {
			return
		}
		seg1, ok := d1.segments[0]
		if !ok || seg1.symbols == nil || len(seg1.symbols) == 0 {
			return
		}

		// determine template from flags
		if len(data) < 13 { // segment header + at least 2 bytes of SD data
			return
		}

		// re-encode with template 1 (always safe, single AT byte)
		reEncoded := EncodeSymbolDictSegment(seg1.symbols, 1)

		var stream []byte
		stream = WriteSegmentHeader(stream, 0, segSymbolDict, 0, nil, uint32(len(reEncoded)))
		stream = append(stream, reEncoded...)
		pageData := WritePageInfo(nil, 1, 1)
		stream = WriteSegmentHeader(stream, 1, segPageInfo, 1, nil, uint32(len(pageData)))
		stream = append(stream, pageData...)

		// second read
		d2 := &decoder{
			segments:  make(map[uint32]segmentResult),
			inputSize: len(stream),
		}
		if err := d2.processStream(stream); err != nil {
			t.Fatalf("re-decode failed: %v", err)
		}
		seg2, ok := d2.segments[0]
		if !ok || seg2.symbols == nil {
			t.Fatalf("no symbols after re-decode")
		}

		if len(seg2.symbols) != len(seg1.symbols) {
			t.Fatalf("symbol count: got %d, want %d",
				len(seg2.symbols), len(seg1.symbols))
		}
		for i := range seg1.symbols {
			if diff := cmp.Diff(seg1.symbols[i].Pix, seg2.symbols[i].Pix); diff != "" {
				t.Errorf("symbol %d round-trip failed (-want +got):\n%s", i, diff)
			}
		}
	})
}

func FuzzHuffRefAggSymbolDict(f *testing.F) {
	// seed from test cases
	for _, tc := range huffRefAggTestCases {
		stream, err := buildHuffRefAggStream(tc)
		if err != nil {
			f.Fatalf("seed encode failed: %v", err)
		}
		f.Add(stream)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// first read: decode the fuzzed stream
		d1 := &decoder{
			segments:  make(map[uint32]segmentResult),
			inputSize: len(data),
		}
		if err := d1.processStream(data); err != nil {
			return
		}

		// look for any symbol dictionary segment
		var symbols []*bitmap.Bitmap
		for _, seg := range d1.segments {
			if len(seg.symbols) > 0 {
				symbols = seg.symbols
				break
			}
		}
		if len(symbols) == 0 {
			return
		}

		// re-encode as arithmetic SD, then re-decode to verify consistency
		reEncoded := EncodeSymbolDictSegment(symbols, 1)

		var stream []byte
		stream = WriteSegmentHeader(stream, 0, segSymbolDict, 0, nil, uint32(len(reEncoded)))
		stream = append(stream, reEncoded...)
		pageData := WritePageInfo(nil, 1, 1)
		stream = WriteSegmentHeader(stream, 1, segPageInfo, 1, nil, uint32(len(pageData)))
		stream = append(stream, pageData...)

		d2 := &decoder{
			segments:  make(map[uint32]segmentResult),
			inputSize: len(stream),
		}
		if err := d2.processStream(stream); err != nil {
			t.Fatalf("re-decode failed: %v", err)
		}
		seg2, ok := d2.segments[0]
		if !ok || seg2.symbols == nil {
			t.Fatalf("no symbols after re-decode")
		}

		if len(seg2.symbols) != len(symbols) {
			// fuzzed input may produce symbols that don't survive
			// a re-encode/re-decode cycle (e.g. due to tighter size limits
			// with different encoding parameters)
			return
		}
		for i := range symbols {
			if diff := cmp.Diff(symbols[i].Pix, seg2.symbols[i].Pix); diff != "" {
				t.Errorf("symbol %d round-trip failed (-want +got):\n%s", i, diff)
			}
		}
	})
}

// TestMultiInstanceAggregation tests the encoder and decoder for
// multi-instance aggregation (REFAGGNINST > 1) in symbol dictionaries.
func TestMultiInstanceAggregation(t *testing.T) {
	// create two small component symbols
	comp0 := makeDiagonal(6, 8)     // component 0
	comp1 := makeCheckerboard(8, 8) // component 1

	// first encode a basic symbol dictionary with the components
	compSyms := []*bitmap.Bitmap{comp0, comp1}
	sdData := EncodeSymbolDictSegment(compSyms, 1)

	// create a composite symbol by placing comp0 and comp1 side by side
	compositeW := comp0.Width() + comp1.Width()
	compositeH := 8
	expected := bitmap.New(compositeW, compositeH)
	expected.Combine(comp0, 0, 0, bitmap.CombOpOR)
	expected.Combine(comp1, comp0.Width(), 0, bitmap.CombOpOR)

	// encode an aggregate symbol dictionary using multi-instance
	// use T=0 (top-left) for bottomLeft corner placement
	aggSyms := []AggregateSymbol{
		{
			Width:  compositeW,
			Height: compositeH,
			Instances: []SymbolInstance{
				{SymID: 0, T: comp0.Height() - 1, S: 0,
					Wi: comp0.Width(), Hi: comp0.Height()},
				{SymID: 1, T: comp1.Height() - 1, S: comp0.Width(),
					Wi: comp1.Width(), Hi: comp1.Height()},
			},
		},
	}
	aggData := EncodeSymbolDictSegmentAgg(aggSyms, len(compSyms))

	// build stream: seg 0 = component SD (global), seg 1 = aggregate SD
	var stream []byte
	stream = WriteSegmentHeader(stream, 0, segSymbolDict, 0, nil, uint32(len(sdData)))
	stream = append(stream, sdData...)
	stream = WriteSegmentHeader(stream, 1, segSymbolDict, 0, []uint32{0}, uint32(len(aggData)))
	stream = append(stream, aggData...)

	// add page info so processStream works
	pageData := WritePageInfo(nil, 1, 1)
	stream = WriteSegmentHeader(stream, 2, segPageInfo, 1, nil, uint32(len(pageData)))
	stream = append(stream, pageData...)

	// decode to get the symbols
	d := &decoder{
		segments:  make(map[uint32]segmentResult),
		inputSize: len(stream),
		memBudget: 1 << 30,
	}
	err := d.processStream(stream)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// the aggregate SD (segment 1) should have exported symbols
	seg1, ok := d.segments[1]
	if !ok || seg1.symbols == nil {
		t.Fatalf("no symbols from aggregate SD")
	}

	// find the composite symbol (last in the exported set)
	if len(seg1.symbols) == 0 {
		t.Fatalf("aggregate SD exported 0 symbols")
	}
	got := seg1.symbols[len(seg1.symbols)-1]

	if got.Width() != expected.Width() || got.Height() != expected.Height() {
		t.Fatalf("dimensions: got %dx%d, want %dx%d",
			got.Width(), got.Height(), expected.Width(), expected.Height())
	}

	if diff := cmp.Diff(expected.Pix, got.Pix); diff != "" {
		t.Errorf("multi-instance aggregation round-trip failed (-want +got):\n%s", diff)
	}
}
