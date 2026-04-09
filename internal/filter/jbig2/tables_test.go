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

	"seehuhn.de/go/pdf/graphics/bitmap"
)

// customTableTestCases defines test cases for custom Huffman table
// round-trip testing. Each entry is used both by the unit test and
// as a fuzz seed.
var customTableTestCases = []struct {
	name  string
	table *huffTable
}{
	{
		// simple positive-only table, no OOB
		name: "positive",
		table: newHuffTable([]huffLine{
			{RangeLow: 0, PrefLen: 1, RangeLen: 4},
			{RangeLow: 16, PrefLen: 2, RangeLen: 8},
			{RangeLow: 272, PrefLen: 3, RangeLen: 16},
			{RangeLow: 65808, PrefLen: 3, RangeLen: 32},
		}),
	},
	{
		// signed table with lower range, no OOB
		name: "signed",
		table: newHuffTable([]huffLine{
			{RangeLow: -128, PrefLen: 4, RangeLen: 7},
			{RangeLow: 0, PrefLen: 1, RangeLen: 5},
			{RangeLow: 32, PrefLen: 2, RangeLen: 6},
			{RangeLow: 96, PrefLen: 3, RangeLen: 7},
			{IsLower: true, RangeLow: -129, PrefLen: 5, RangeLen: 32},
			{RangeLow: 224, PrefLen: 5, RangeLen: 32},
		}),
	},
	{
		// table with OOB
		name: "with_oob",
		table: newHuffTable([]huffLine{
			{RangeLow: 0, PrefLen: 1, RangeLen: 0},
			{RangeLow: 1, PrefLen: 2, RangeLen: 0},
			{RangeLow: 2, PrefLen: 3, RangeLen: 2},
			{RangeLow: 6, PrefLen: 4, RangeLen: 32},
			{IsOOB: true, PrefLen: 4},
		}),
	},
	{
		// small range, single normal line
		name: "minimal",
		table: newHuffTable([]huffLine{
			{RangeLow: 1, PrefLen: 1, RangeLen: 0},
			{RangeLow: 2, PrefLen: 1, RangeLen: 32},
		}),
	},
}

func customTableRoundTrip(t *testing.T, orig *huffTable) {
	t.Helper()

	data, err := encodeCustomHuffTable(orig)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	decoded, err := parseCustomHuffTable(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// verify by encoding and decoding a range of values
	testValues := []int64{-1000, -100, -1, 0, 1, 2, 5, 10, 50, 100, 1000, 100000}
	for _, v := range testValues {
		// try encoding with original
		w1 := &bitWriter{}
		err1 := orig.encode(w1, v)

		w2 := &bitWriter{}
		err2 := decoded.encode(w2, v)

		if (err1 == nil) != (err2 == nil) {
			t.Errorf("value %d: encode error mismatch: orig=%v, decoded=%v", v, err1, err2)
			continue
		}
		if err1 != nil {
			continue
		}

		// decode from original's encoding
		hr1 := newHuffReader(w1.bytes())
		val1 := orig.decode(hr1)

		hr2 := newHuffReader(w2.bytes())
		val2 := decoded.decode(hr2)

		if val1 != val2 {
			t.Errorf("value %d: decode mismatch: orig=%d, decoded=%d", v, val1, val2)
		}
	}

	// test OOB if supported
	w1 := &bitWriter{}
	err1 := orig.encodeOOB(w1)
	w2 := &bitWriter{}
	err2 := decoded.encodeOOB(w2)
	if (err1 == nil) != (err2 == nil) {
		t.Errorf("OOB encode mismatch: orig=%v, decoded=%v", err1, err2)
	}
}

func TestCustomHuffTableRoundTrip(t *testing.T) {
	for _, tc := range customTableTestCases {
		t.Run(tc.name, func(t *testing.T) {
			customTableRoundTrip(t, tc.table)
		})
	}
}

// TestCustomHuffTableStandardRoundTrip verifies that standard tables
// survive encode/decode through the type-53 format.
func TestCustomHuffTableStandardRoundTrip(t *testing.T) {
	standards := []struct {
		name  string
		table *huffTable
	}{
		{"B1", huffTableB1},
		{"B2", huffTableB2},
		{"B4", huffTableB4},
		{"B6", huffTableB6},
		{"B8", huffTableB8},
		{"B11", huffTableB11},
		{"B14", huffTableB14},
		{"B15", huffTableB15},
	}
	for _, tc := range standards {
		t.Run(tc.name, func(t *testing.T) {
			customTableRoundTrip(t, tc.table)
		})
	}
}

func FuzzCustomHuffTable(f *testing.F) {
	// seed from test cases
	for _, tc := range customTableTestCases {
		data, err := encodeCustomHuffTable(tc.table)
		if err != nil {
			f.Fatalf("seed encode failed: %v", err)
		}
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		t1, err := parseCustomHuffTable(data)
		if err != nil {
			return
		}

		// encode → parse must be stable: a second encode→parse
		// must produce a table functionally equivalent to the first
		enc1, err := encodeCustomHuffTable(t1)
		if err != nil {
			return // table can't be represented in type-53 format
		}
		t2, err := parseCustomHuffTable(enc1)
		if err != nil {
			t.Fatalf("re-parse failed: %v", err)
		}

		enc2, err := encodeCustomHuffTable(t2)
		if err != nil {
			t.Fatalf("second encode failed: %v", err)
		}
		t3, err := parseCustomHuffTable(enc2)
		if err != nil {
			t.Fatalf("second re-parse failed: %v", err)
		}

		// t2 and t3 must be functionally equivalent
		testValues := []int64{-100, -1, 0, 1, 10, 100, 1000}
		for _, v := range testValues {
			w2 := &bitWriter{}
			err2 := t2.encode(w2, v)
			w3 := &bitWriter{}
			err3 := t3.encode(w3, v)
			if (err2 == nil) != (err3 == nil) {
				t.Errorf("value %d: encode mismatch after stable round-trip (t2=%v, t3=%v)", v, err2, err3)
				t.Errorf("t2 lines: %+v", t2.Lines)
				t.Errorf("t3 lines: %+v", t3.Lines)
				t.Errorf("enc1: %X", enc1)
				t.Errorf("enc2: %X", enc2)
				continue
			}
			if err2 != nil {
				continue
			}
			hr2 := newHuffReader(w2.bytes())
			hr3 := newHuffReader(w3.bytes())
			if t2.decode(hr2) != t3.decode(hr3) {
				t.Errorf("value %d: decode mismatch after stable round-trip", v)
			}
		}
	})
}

func TestCustomTableTextRegion(t *testing.T) {
	// test all corner/transposed combinations, same as
	// TestTextRegionHuffmanRoundTrip but with a custom FS table
	for _, tc := range textRegionTestCases {
		t.Run(tc.name, func(t *testing.T) {
			customTableTextRegionRoundTrip(t, tc.refCorner, tc.transposed)
		})
	}
}

func customTableTextRegionRoundTrip(t *testing.T, refCorner int, transposed bool) {
	t.Helper()

	symbols := makeTextTestSymbols()
	width, height, instances, expected := buildTextInstances(
		symbols, refCorner, transposed)

	// use B.6 as the custom FS table (functionally equivalent to the
	// standard selection, but transmitted as a type-53 segment)
	fsTable := huffTableB6

	tableData, err := encodeCustomHuffTable(fsTable)
	if err != nil {
		t.Fatalf("encode custom table: %v", err)
	}

	sdData := EncodeSymbolDictSegment(symbols, 1)

	// SBHUFFFS=3 (user-supplied) in htags bits 0-1
	trData, err := encodeTextRegionHuffman(
		width, height, 0, 0, instances, symbols,
		refCorner, transposed, bitmap.CombOpOR,
		fsTable, uint16(3))
	if err != nil {
		t.Fatalf("encode text region: %v", err)
	}

	// segment 0: symbol dictionary
	// segment 1: custom Huffman table
	// segment 2: page info
	// segment 3: text region (refs segments 0 and 1)
	var stream []byte

	stream = WriteSegmentHeader(stream, 0, segSymbolDict, 0, nil, uint32(len(sdData)))
	stream = append(stream, sdData...)

	stream = WriteSegmentHeader(stream, 1, segTables, 0, nil, uint32(len(tableData)))
	stream = append(stream, tableData...)

	pageData := WritePageInfo(nil, width, height)
	stream = WriteSegmentHeader(stream, 2, segPageInfo, 1, nil, uint32(len(pageData)))
	stream = append(stream, pageData...)

	stream = WriteSegmentHeader(stream, 3, segImmediateTextRegion, 1,
		[]uint32{0, 1}, uint32(len(trData)))
	stream = append(stream, trData...)

	got, err := Decode(nil, stream)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if !bitmapsEqual(got, expected) {
		t.Errorf("round-trip mismatch")
	}
}

func TestCustomTableSymbolDict(t *testing.T) {
	symbols := []*bitmap.Bitmap{
		makeAllZeros(6, 10),
		makeCheckerboard(8, 10),
		makeDiagonal(10, 10),
	}

	// create a custom DH table (equivalent to B.4)
	dhTable := huffTableB4

	// encode custom table segment
	tableData, err := encodeCustomHuffTable(dhTable)
	if err != nil {
		t.Fatalf("encode custom table: %v", err)
	}

	// encode Huffman symbol dictionary with custom DH table
	// Use the existing EncodeSymbolDictSegmentHuffRef as reference,
	// but we need a simpler direct-coded SD with custom DH.
	// For now, just verify the decoder handles custom tables in SDs
	// by constructing the stream manually.

	// encode reference SD (arithmetic, for the actual symbols)
	sdData := EncodeSymbolDictSegment(symbols, 1)

	// build stream and decode to verify symbols survive
	var stream []byte
	stream = WriteSegmentHeader(stream, 0, segTables, 0, nil, uint32(len(tableData)))
	stream = append(stream, tableData...)
	stream = WriteSegmentHeader(stream, 1, segSymbolDict, 0, nil, uint32(len(sdData)))
	stream = append(stream, sdData...)
	pageData := WritePageInfo(nil, 1, 1)
	stream = WriteSegmentHeader(stream, 2, segPageInfo, 1, nil, uint32(len(pageData)))
	stream = append(stream, pageData...)

	// decode — this verifies type-53 segments are parsed without error
	d := &decoder{
		segments:  make(map[uint32]segmentResult),
		inputSize: len(stream),
	}
	if err := d.processStream(stream); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// verify the table was stored
	seg, ok := d.segments[0]
	if !ok || seg.table == nil {
		t.Fatalf("custom table not stored in segments")
	}

	// verify symbols decoded correctly
	seg1, ok := d.segments[1]
	if !ok || seg1.symbols == nil {
		t.Fatalf("symbol dictionary not decoded")
	}
	if len(seg1.symbols) != len(symbols) {
		t.Fatalf("got %d symbols, want %d", len(seg1.symbols), len(symbols))
	}
	for i, want := range symbols {
		if !bitmapsEqual(seg1.symbols[i], want) {
			t.Errorf("symbol %d mismatch", i)
		}
	}
}

func FuzzCustomTableTextRegion(f *testing.F) {
	symbols := makeTextTestSymbols()
	fsTable := huffTableB6

	// seed from all corner/transposed combinations
	for _, tc := range textRegionTestCases {
		width, height, instances, _ := buildTextInstances(
			symbols, tc.refCorner, tc.transposed)

		tableData, err := encodeCustomHuffTable(fsTable)
		if err != nil {
			f.Fatalf("encode custom table: %v", err)
		}
		sdData := EncodeSymbolDictSegment(symbols, 1)
		trData, err := encodeTextRegionHuffman(
			width, height, 0, 0, instances, symbols,
			tc.refCorner, tc.transposed, bitmap.CombOpOR,
			fsTable, uint16(3))
		if err != nil {
			f.Fatalf("encode text region: %v", err)
		}

		var stream []byte
		stream = WriteSegmentHeader(stream, 0, segSymbolDict, 0, nil, uint32(len(sdData)))
		stream = append(stream, sdData...)
		stream = WriteSegmentHeader(stream, 1, segTables, 0, nil, uint32(len(tableData)))
		stream = append(stream, tableData...)
		pageData := WritePageInfo(nil, width, height)
		stream = WriteSegmentHeader(stream, 2, segPageInfo, 1, nil, uint32(len(pageData)))
		stream = append(stream, pageData...)
		stream = WriteSegmentHeader(stream, 3, segImmediateTextRegion, 1,
			[]uint32{0, 1}, uint32(len(trData)))
		stream = append(stream, trData...)
		f.Add(stream)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzBitmapRoundTrip(t, data)
	})
}
