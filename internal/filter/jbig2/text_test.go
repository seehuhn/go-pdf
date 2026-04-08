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
		width, height, 0, 0, instances, len(symbols),
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

func FuzzTextRegionRoundTrip(f *testing.F) {
	symbols := makeTextTestSymbols()

	// seed from test cases
	for _, tc := range textRegionTestCases {
		width, height, instances, _ := buildTextInstances(
			symbols, tc.refCorner, tc.transposed)
		sdData := EncodeSymbolDictSegment(symbols, 1)
		trData := EncodeTextRegionSegment(
			width, height, 0, 0, instances, len(symbols),
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
		// first read
		bm1, err := Decode(nil, data)
		if err != nil || bm1 == nil {
			return
		}
		if bm1.Width() == 0 || bm1.Height() == 0 {
			return
		}

		// re-encode as a generic region (lossless bitmap round-trip)
		reEncoded := EncodeGenericRegionSegment(bm1, 0, 0, 1, bitmap.CombOpOR)

		var stream []byte
		pageData := WritePageInfo(nil, bm1.Width(), bm1.Height())
		stream = WriteSegmentHeader(stream, 0, segPageInfo, 1, nil, uint32(len(pageData)))
		stream = append(stream, pageData...)
		stream = WriteSegmentHeader(stream, 1, segImmediateGeneric, 1, nil, uint32(len(reEncoded)))
		stream = append(stream, reEncoded...)

		// second read
		bm2, err := Decode(nil, stream)
		if err != nil {
			t.Fatalf("re-decode failed: %v", err)
		}

		if diff := cmp.Diff(bm1.Pix, bm2.Pix); diff != "" {
			t.Errorf("round-trip failed (-want +got):\n%s", diff)
		}
	})
}
