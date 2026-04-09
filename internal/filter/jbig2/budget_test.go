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

func TestAllocBitmap(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		budget := int64(10000)
		bm, err := allocBitmap(&budget, 16, 8)
		if err != nil {
			t.Fatal(err)
		}
		if bm.Width() != 16 || bm.Height() != 8 {
			t.Fatalf("got %dx%d, want 16x8", bm.Width(), bm.Height())
		}
		// 16 pixels wide = 2 bytes stride, 8 rows = 16 bytes
		if budget != 10000-16 {
			t.Fatalf("budget = %d, want %d", budget, 10000-16)
		}
	})

	t.Run("exceeded", func(t *testing.T) {
		budget := int64(10)
		_, err := allocBitmap(&budget, 100, 100)
		if err == nil {
			t.Fatal("expected error")
		}
		// budget unchanged on failure
		if budget != 10 {
			t.Fatalf("budget changed to %d on failure", budget)
		}
	})

	t.Run("negative_dims", func(t *testing.T) {
		budget := int64(10000)
		_, err := allocBitmap(&budget, -1, 10)
		if err == nil {
			t.Fatal("expected error for negative width")
		}
	})

	t.Run("zero_dims", func(t *testing.T) {
		budget := int64(10000)
		bm, err := allocBitmap(&budget, 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		if bm.Width() != 0 {
			t.Fatalf("got width %d, want 0", bm.Width())
		}
		if budget != 10000 {
			t.Fatalf("budget changed for zero-size bitmap")
		}
	})
}

func TestFreeBitmap(t *testing.T) {
	budget := int64(10000)
	bm, err := allocBitmap(&budget, 16, 8)
	if err != nil {
		t.Fatal(err)
	}
	after := budget
	freeBitmap(&budget, bm)
	if budget != 10000 {
		t.Fatalf("budget after free = %d, want 10000 (was %d after alloc)", budget, after)
	}
}

func TestAllocInts(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		budget := int64(1000)
		s, err := allocInts(&budget, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(s) != 10 {
			t.Fatalf("got len %d, want 10", len(s))
		}
		if budget != 1000-80 {
			t.Fatalf("budget = %d, want %d", budget, 1000-80)
		}
	})

	t.Run("exceeded", func(t *testing.T) {
		budget := int64(10)
		_, err := allocInts(&budget, 100)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestBudgetExceeded_LargeRegion(t *testing.T) {
	// craft a minimal JBIG2 stream with a page bitmap that exceeds the
	// budget for this input size
	bm := bitmap.New(256, 256) // 8 KB bitmap
	segData := EncodeGenericRegionSegment(bm, 0, 0, 1, bitmap.CombOpOR, false, false)
	pageData := WritePageInfo(nil, 256, 256)

	var stream []byte
	stream = WriteSegmentHeader(stream, 0, segPageInfo, 1, nil, uint32(len(pageData)))
	stream = append(stream, pageData...)
	stream = WriteSegmentHeader(stream, 1, segImmediateGeneric, 1, nil, uint32(len(segData)))
	stream = append(stream, segData...)

	// give a budget that's too small for the 256x256 page + 256x256 region
	d := &decoder{
		segments:  make(map[uint32]segmentResult),
		inputSize: len(stream),
		memBudget: 100, // way too small
	}
	err := d.processStream(stream)
	if err == nil {
		t.Fatal("expected budget exceeded error")
	}
}

func TestBudgetSufficient_Normal(t *testing.T) {
	// verify that Decode works with the normal budget formula
	bm := makeCheckerboard(32, 32)
	segData := EncodeGenericRegionSegment(bm, 0, 0, 1, bitmap.CombOpOR, false, false)
	pageData := WritePageInfo(nil, 32, 32)

	var stream []byte
	stream = WriteSegmentHeader(stream, 0, segPageInfo, 1, nil, uint32(len(pageData)))
	stream = append(stream, pageData...)
	stream = WriteSegmentHeader(stream, 1, segImmediateGeneric, 1, nil, uint32(len(segData)))
	stream = append(stream, segData...)

	got, err := Decode(nil, stream)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if !bitmapsEqual(got, bm) {
		t.Error("round-trip mismatch")
	}
}

func TestMaxBitplanes(t *testing.T) {
	// verify that gsbpp > maxBitplanes is rejected
	budget := int64(1 << 30)
	_, err := decodeGrayScaleImage(&budget, nil, false, 0, 17, 4, 4, false, nil)
	if err == nil {
		t.Fatal("expected error for 17 bitplanes")
	}
}
