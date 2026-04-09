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

type textRegionTestCase struct {
	name       string
	refCorner  int
	transposed bool
}

var textRegionTestCases = []textRegionTestCase{
	{"bottomleft", cornerBottomLeft, false},
	{"topleft", cornerTopLeft, false},
	{"bottomright", cornerBottomRight, false},
	{"topright", cornerTopRight, false},
	{"bottomleft_transposed", cornerBottomLeft, true},
	{"topleft_transposed", cornerTopLeft, true},
	{"bottomright_transposed", cornerBottomRight, true},
	{"topright_transposed", cornerTopRight, true},
}

// makeTextTestSymbols creates a small set of distinct symbols for testing.
func makeTextTestSymbols() []*bitmap.Bitmap {
	return []*bitmap.Bitmap{
		makeDiagonal(6, 8),
		makeCheckerboard(8, 8),
		makeVStripes(6, 8),
		makeHStripes(8, 8),
	}
}

// buildTextInstances creates a row of non-overlapping symbol placements.
// Returns instances in T/S coordinates and the expected page bitmap.
func buildTextInstances(
	symbols []*bitmap.Bitmap,
	refCorner int, transposed bool,
) (width, height int, instances []SymbolInstance, expected *bitmap.Bitmap) {
	// place symbols 0, 1, 2, 3 in a horizontal (or vertical if transposed) line
	type placement struct {
		symID  int
		px, py int // top-left pixel position
	}

	symIDs := []int{0, 1, 2, 3}
	placements := make([]placement, len(symIDs))

	// lay out symbols left-to-right (or top-to-bottom if transposed)
	offset := 2 // starting offset
	for i, id := range symIDs {
		sym := symbols[id]
		if !transposed {
			placements[i] = placement{id, offset, 2}
			offset += sym.Width() + 1
		} else {
			placements[i] = placement{id, 2, offset}
			offset += sym.Height() + 1
		}
	}

	// region dimensions
	if !transposed {
		width = offset + 2
		height = 12
	} else {
		width = 12
		height = offset + 2
	}

	// build expected bitmap
	expected = bitmap.New(width, height)
	for _, p := range placements {
		expected.Combine(symbols[p.symID], p.px, p.py, bitmap.CombOpOR)
	}

	// Convert pixel positions to T/S coordinates.
	//
	// The decoder's pre-update and corner adjustment cancel out for
	// the inline (S) axis, so S is always the leading edge:
	//   not transposed: S = px (left x)
	//   transposed:     S = py (top y)
	//
	// For the strip (T) axis, the corner determines which edge:
	//   TOP corners:    T = py (not transposed) or T = px (transposed)
	//   BOTTOM corners: T = py + hi - 1 (not transposed) or
	//                   same logic for transposed with wi/hi swapped
	instances = make([]SymbolInstance, len(placements))
	for i, p := range placements {
		sym := symbols[p.symID]
		wi := sym.Width()
		hi := sym.Height()

		var t, s int
		if !transposed {
			// T = strip Y, S = inline X
			s = p.px
			switch refCorner {
			case cornerTopLeft, cornerTopRight:
				t = p.py
			case cornerBottomLeft, cornerBottomRight:
				t = p.py + hi - 1
			}
		} else {
			// T = strip X, S = inline Y
			// corner adjustments: RIGHT → px -= wi-1, BOTTOM → py -= hi-1
			// so T accounts for RIGHT (px adjustment), S accounts for BOTTOM (py adjustment)
			s = p.py
			switch refCorner {
			case cornerTopLeft, cornerBottomLeft:
				t = p.px
			case cornerTopRight, cornerBottomRight:
				t = p.px + wi - 1
			}
		}

		instances[i] = SymbolInstance{
			SymID: p.symID,
			T:     t,
			S:     s,
			Wi:    wi,
			Hi:    hi,
		}
	}

	return width, height, instances, expected
}

func textRegionRoundTrip(t *testing.T, tc textRegionTestCase) {
	t.Helper()

	symbols := makeTextTestSymbols()
	width, height, instances, expected := buildTextInstances(
		symbols, tc.refCorner, tc.transposed)

	// encode symbol dictionary
	sdData := EncodeSymbolDictSegment(symbols, 1)

	// encode text region
	trData := EncodeTextRegionSegment(
		width, height, 0, 0, instances, symbols,
		tc.refCorner, tc.transposed, bitmap.CombOpOR)

	// build page stream
	var stream []byte

	// segment 0: symbol dictionary (global)
	stream = WriteSegmentHeader(stream, 0, segSymbolDict, 0, nil, uint32(len(sdData)))
	stream = append(stream, sdData...)

	// segment 1: page info
	pageData := WritePageInfo(nil, width, height)
	stream = WriteSegmentHeader(stream, 1, segPageInfo, 1, nil, uint32(len(pageData)))
	stream = append(stream, pageData...)

	// segment 2: immediate text region (refs segment 0)
	stream = WriteSegmentHeader(stream, 2, segImmediateTextRegion, 1, []uint32{0}, uint32(len(trData)))
	stream = append(stream, trData...)

	// decode
	got, err := Decode(nil, stream)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if diff := cmp.Diff(expected.Pix, got.Pix); diff != "" {
		t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
	}
}

func TestTextRegionRoundTrip(t *testing.T) {
	for _, tc := range textRegionTestCases {
		t.Run(tc.name, func(t *testing.T) {
			textRegionRoundTrip(t, tc)
		})
	}
}

func textRegionHuffmanRoundTrip(t *testing.T, tc textRegionTestCase) {
	t.Helper()

	symbols := makeTextTestSymbols()
	width, height, instances, expected := buildTextInstances(
		symbols, tc.refCorner, tc.transposed)

	// encode symbol dictionary
	sdData := EncodeSymbolDictSegment(symbols, 1)

	// encode text region with Huffman
	trData, err := EncodeTextRegionSegmentHuffman(
		width, height, 0, 0, instances, symbols,
		tc.refCorner, tc.transposed, bitmap.CombOpOR)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	// build page stream
	var stream []byte
	stream = WriteSegmentHeader(stream, 0, segSymbolDict, 0, nil, uint32(len(sdData)))
	stream = append(stream, sdData...)
	pageData := WritePageInfo(nil, width, height)
	stream = WriteSegmentHeader(stream, 1, segPageInfo, 1, nil, uint32(len(pageData)))
	stream = append(stream, pageData...)
	stream = WriteSegmentHeader(stream, 2, segImmediateTextRegion, 1, []uint32{0}, uint32(len(trData)))
	stream = append(stream, trData...)

	// decode
	got, err := Decode(nil, stream)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if diff := cmp.Diff(expected.Pix, got.Pix); diff != "" {
		t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
	}
}

func TestTextRegionHuffmanRoundTrip(t *testing.T) {
	for _, tc := range textRegionTestCases {
		t.Run(tc.name, func(t *testing.T) {
			textRegionHuffmanRoundTrip(t, tc)
		})
	}
}

func FuzzTextRegionHuffmanRoundTrip(f *testing.F) {
	symbols := makeTextTestSymbols()

	// seed from test cases
	for _, tc := range textRegionTestCases {
		width, height, instances, _ := buildTextInstances(
			symbols, tc.refCorner, tc.transposed)
		sdData := EncodeSymbolDictSegment(symbols, 1)
		trData, err := EncodeTextRegionSegmentHuffman(
			width, height, 0, 0, instances, symbols,
			tc.refCorner, tc.transposed, bitmap.CombOpOR)
		if err != nil {
			f.Fatalf("seed encode failed: %v", err)
		}

		var stream []byte
		stream = WriteSegmentHeader(stream, 0, segSymbolDict, 0, nil, uint32(len(sdData)))
		stream = append(stream, sdData...)
		pageData := WritePageInfo(nil, width, height)
		stream = WriteSegmentHeader(stream, 1, segPageInfo, 1, nil, uint32(len(pageData)))
		stream = append(stream, pageData...)
		stream = WriteSegmentHeader(stream, 2, segImmediateTextRegion, 1, []uint32{0}, uint32(len(trData)))
		stream = append(stream, trData...)
		f.Add(stream)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzBitmapRoundTrip(t, data)
	})
}

// buildTextRefineInstances creates symbol placements where symbols 1 and 3
// are refined (different bitmap, same size as dictionary symbol).
func buildTextRefineInstances(
	symbols []*bitmap.Bitmap,
	refCorner int, transposed bool,
) (width, height int, instances []SymbolInstance, expected *bitmap.Bitmap) {
	// refined bitmaps for symbols 1 and 3
	refinedSyms := map[int]*bitmap.Bitmap{
		1: makeHStripes(symbols[1].Width(), symbols[1].Height()),
		3: makeVStripes(symbols[3].Width(), symbols[3].Height()),
	}

	width, height, instances, expected = buildTextInstances(
		symbols, refCorner, transposed)

	// patch instances: use refined bitmaps for symbols 1 and 3
	for i := range instances {
		if bm, ok := refinedSyms[instances[i].SymID]; ok {
			instances[i].Bitmap = bm
		}
	}

	// rebuild expected bitmap with refined symbols
	expected = bitmap.New(width, height)
	for _, inst := range instances {
		var bm *bitmap.Bitmap
		if inst.Bitmap != nil {
			bm = inst.Bitmap
		} else {
			bm = symbols[inst.SymID]
		}

		// compute pixel position from T/S (reverse of buildTextInstances logic)
		var px, py int
		if !transposed {
			px = inst.S
			switch refCorner {
			case cornerTopLeft, cornerTopRight:
				py = inst.T
			case cornerBottomLeft, cornerBottomRight:
				py = inst.T - bm.Height() + 1
			}
		} else {
			py = inst.S
			switch refCorner {
			case cornerTopLeft, cornerBottomLeft:
				px = inst.T
			case cornerTopRight, cornerBottomRight:
				px = inst.T - bm.Width() + 1
			}
		}

		expected.Combine(bm, px, py, bitmap.CombOpOR)
	}

	return width, height, instances, expected
}

func textRegionRefineRoundTrip(t *testing.T, tc textRegionTestCase, huffman bool) {
	t.Helper()

	symbols := makeTextTestSymbols()
	width, height, instances, expected := buildTextRefineInstances(
		symbols, tc.refCorner, tc.transposed)

	sdData := EncodeSymbolDictSegment(symbols, 1)

	var trData []byte
	if huffman {
		var err error
		trData, err = EncodeTextRegionSegmentHuffman(
			width, height, 0, 0, instances, symbols,
			tc.refCorner, tc.transposed, bitmap.CombOpOR)
		if err != nil {
			t.Fatalf("encode failed: %v", err)
		}
	} else {
		trData = EncodeTextRegionSegment(
			width, height, 0, 0, instances, symbols,
			tc.refCorner, tc.transposed, bitmap.CombOpOR)
	}

	var stream []byte
	stream = WriteSegmentHeader(stream, 0, segSymbolDict, 0, nil, uint32(len(sdData)))
	stream = append(stream, sdData...)
	pageData := WritePageInfo(nil, width, height)
	stream = WriteSegmentHeader(stream, 1, segPageInfo, 1, nil, uint32(len(pageData)))
	stream = append(stream, pageData...)
	stream = WriteSegmentHeader(stream, 2, segImmediateTextRegion, 1, []uint32{0}, uint32(len(trData)))
	stream = append(stream, trData...)

	got, err := Decode(nil, stream)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if diff := cmp.Diff(expected.Pix, got.Pix); diff != "" {
		t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
	}
}

func TestTextRegionRefineRoundTrip(t *testing.T) {
	for _, tc := range textRegionTestCases {
		t.Run(tc.name, func(t *testing.T) {
			textRegionRefineRoundTrip(t, tc, false)
		})
	}
}

func TestTextRegionHuffmanRefineRoundTrip(t *testing.T) {
	for _, tc := range textRegionTestCases {
		t.Run(tc.name, func(t *testing.T) {
			textRegionRefineRoundTrip(t, tc, true)
		})
	}
}

func FuzzTextRegionRefineRoundTrip(f *testing.F) {
	symbols := makeTextTestSymbols()

	for _, tc := range textRegionTestCases {
		width, height, instances, _ := buildTextRefineInstances(
			symbols, tc.refCorner, tc.transposed)
		sdData := EncodeSymbolDictSegment(symbols, 1)
		trData := EncodeTextRegionSegment(
			width, height, 0, 0, instances, symbols,
			tc.refCorner, tc.transposed, bitmap.CombOpOR)

		var stream []byte
		stream = WriteSegmentHeader(stream, 0, segSymbolDict, 0, nil, uint32(len(sdData)))
		stream = append(stream, sdData...)
		pageData := WritePageInfo(nil, width, height)
		stream = WriteSegmentHeader(stream, 1, segPageInfo, 1, nil, uint32(len(pageData)))
		stream = append(stream, pageData...)
		stream = WriteSegmentHeader(stream, 2, segImmediateTextRegion, 1, []uint32{0}, uint32(len(trData)))
		stream = append(stream, trData...)
		f.Add(stream)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzBitmapRoundTrip(t, data)
	})
}

func FuzzTextRegionHuffmanRefineRoundTrip(f *testing.F) {
	symbols := makeTextTestSymbols()

	for _, tc := range textRegionTestCases {
		width, height, instances, _ := buildTextRefineInstances(
			symbols, tc.refCorner, tc.transposed)
		sdData := EncodeSymbolDictSegment(symbols, 1)
		trData, err := EncodeTextRegionSegmentHuffman(
			width, height, 0, 0, instances, symbols,
			tc.refCorner, tc.transposed, bitmap.CombOpOR)
		if err != nil {
			f.Fatalf("seed encode failed: %v", err)
		}

		var stream []byte
		stream = WriteSegmentHeader(stream, 0, segSymbolDict, 0, nil, uint32(len(sdData)))
		stream = append(stream, sdData...)
		pageData := WritePageInfo(nil, width, height)
		stream = WriteSegmentHeader(stream, 1, segPageInfo, 1, nil, uint32(len(pageData)))
		stream = append(stream, pageData...)
		stream = WriteSegmentHeader(stream, 2, segImmediateTextRegion, 1, []uint32{0}, uint32(len(trData)))
		stream = append(stream, trData...)
		f.Add(stream)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzBitmapRoundTrip(t, data)
	})
}

// TestIntermediateTextRegionRoundTrip tests intermediate text region
// (type 4) stored as auxiliary buffer, then referenced by a refinement.
func TestIntermediateTextRegionRoundTrip(t *testing.T) {
	symbols := makeTextTestSymbols()
	width, height, instances, expected := buildTextInstances(
		symbols, cornerBottomLeft, false)

	sdData := EncodeSymbolDictSegment(symbols, 1)
	trData := EncodeTextRegionSegment(
		width, height, 0, 0, instances, symbols,
		cornerBottomLeft, false, bitmap.CombOpOR)

	// encode a refinement of the text region bitmap (identity refinement)
	refinData := EncodeRefinementRegionSegment(expected, expected, 0, 0, 1, bitmap.CombOpOR)

	var stream []byte

	// segment 0: symbol dictionary (global)
	stream = WriteSegmentHeader(stream, 0, segSymbolDict, 0, nil, uint32(len(sdData)))
	stream = append(stream, sdData...)

	// segment 1: page info
	pageData := WritePageInfo(nil, width, height)
	stream = WriteSegmentHeader(stream, 1, segPageInfo, 1, nil, uint32(len(pageData)))
	stream = append(stream, pageData...)

	// segment 2: intermediate text region (type 4, NOT composited)
	stream = WriteSegmentHeader(stream, 2, segIntermediateTextRegion, 1, []uint32{0}, uint32(len(trData)))
	stream = append(stream, trData...)

	// segment 3: immediate refinement (type 42) referring to seg 2
	stream = WriteSegmentHeader(stream, 3, segImmediateRefinement, 1, []uint32{2}, uint32(len(refinData)))
	stream = append(stream, refinData...)

	got, err := Decode(nil, stream)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if diff := cmp.Diff(expected.Pix, got.Pix); diff != "" {
		t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
	}
}

func FuzzTextRegionRoundTrip(f *testing.F) {
	symbols := makeTextTestSymbols()

	// seed from test cases
	for _, tc := range textRegionTestCases {
		width, height, instances, _ := buildTextInstances(
			symbols, tc.refCorner, tc.transposed)
		sdData := EncodeSymbolDictSegment(symbols, 1)
		trData := EncodeTextRegionSegment(
			width, height, 0, 0, instances, symbols,
			tc.refCorner, tc.transposed, bitmap.CombOpOR)

		var stream []byte
		stream = WriteSegmentHeader(stream, 0, segSymbolDict, 0, nil, uint32(len(sdData)))
		stream = append(stream, sdData...)
		pageData := WritePageInfo(nil, width, height)
		stream = WriteSegmentHeader(stream, 1, segPageInfo, 1, nil, uint32(len(pageData)))
		stream = append(stream, pageData...)
		stream = WriteSegmentHeader(stream, 2, segImmediateTextRegion, 1, []uint32{0}, uint32(len(trData)))
		stream = append(stream, trData...)
		f.Add(stream)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzBitmapRoundTrip(t, data)
	})
}
