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

// TestUnknownPageHeightRoundTrip tests decoding a page with unknown height
// (0xFFFFFFFF) split into multiple stripes with end-of-stripe segments.
func TestUnknownPageHeightRoundTrip(t *testing.T) {
	// create a 32×48 bitmap with a recognisable pattern
	fullBm := makeDiagonal(32, 48)

	stripeHeight := 16
	width := fullBm.Width()
	height := fullBm.Height()
	nStripes := height / stripeHeight

	var stream []byte
	segNum := uint32(0)

	// segment 0: page info with unknown height
	pageData := WritePageInfoStripe(nil, width, stripeHeight)
	stream = WriteSegmentHeader(stream, segNum, segPageInfo, 1, nil, uint32(len(pageData)))
	stream = append(stream, pageData...)
	segNum++

	// encode each stripe as a generic region + end-of-stripe
	for i := range nStripes {
		y := i * stripeHeight

		// extract stripe bitmap
		stripe := bitmap.New(width, stripeHeight)
		for sy := range stripeHeight {
			for sx := range width {
				stripe.SetPixel(sx, sy, fullBm.GetPixel(sx, y+sy))
			}
		}

		// immediate generic region at (0, y)
		segData := EncodeGenericRegionSegment(stripe, 0, y, 1, bitmap.CombOpOR)
		stream = WriteSegmentHeader(stream, segNum, segImmediateGeneric, 1, nil, uint32(len(segData)))
		stream = append(stream, segData...)
		segNum++

		// end-of-stripe
		eosData := WriteEndOfStripe(nil, y+stripeHeight-1)
		stream = WriteSegmentHeader(stream, segNum, segEndOfStripe, 1, nil, uint32(len(eosData)))
		stream = append(stream, eosData...)
		segNum++
	}

	got, err := Decode(nil, stream)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if got.Width() != width || got.Height() != height {
		t.Fatalf("dimensions: got %dx%d, want %dx%d", got.Width(), got.Height(), width, height)
	}

	if !bitmapsEqual(got, fullBm) {
		t.Errorf("round-trip mismatch")
	}
}

// TestUnknownPageHeightSingleStripe tests the edge case of a single
// stripe covering the entire page.
func TestUnknownPageHeightSingleStripe(t *testing.T) {
	bm := makeCheckerboard(16, 16)

	var stream []byte

	// segment 0: page info with unknown height
	pageData := WritePageInfoStripe(nil, 16, 16)
	stream = WriteSegmentHeader(stream, 0, segPageInfo, 1, nil, uint32(len(pageData)))
	stream = append(stream, pageData...)

	// segment 1: immediate generic region
	segData := EncodeGenericRegionSegment(bm, 0, 0, 1, bitmap.CombOpOR)
	stream = WriteSegmentHeader(stream, 1, segImmediateGeneric, 1, nil, uint32(len(segData)))
	stream = append(stream, segData...)

	// segment 2: end-of-stripe
	eosData := WriteEndOfStripe(nil, 15)
	stream = WriteSegmentHeader(stream, 2, segEndOfStripe, 1, nil, uint32(len(eosData)))
	stream = append(stream, eosData...)

	got, err := Decode(nil, stream)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if !bitmapsEqual(got, bm) {
		t.Errorf("round-trip mismatch")
	}
}
