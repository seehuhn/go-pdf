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

	"seehuhn.de/go/membudget"
	"seehuhn.de/go/pdf/graphics/bitmap"
	"seehuhn.de/go/pdf/internal/limits"
)

func newTestPool(remaining int64) *bitmapPool {
	return &bitmapPool{budget: membudget.New(remaining)}
}

// testBudget returns a budget large enough for tests to never trip
// the cap.
func testBudget() *membudget.Budget {
	return membudget.New(1 << 30)
}

// fuzzBudget returns the memory budget used when decoding untrusted input
// in a fuzz target.  It is input-proportional, like the production budget
// [limits.StreamBudget], but with a smaller base and a hard ceiling.
//
// The unrealistically large [testBudget] (1 GiB) must not be used on fuzz
// input: because JBIG2 is a compression format, a tiny stream (MQ data
// padded past its end) can decode billions of pixels, which never happens
// in production and only manifests as fuzzing-engine timeouts.  Even the
// production 8 MiB floor is too generous here, because the round-trip
// targets decode→encode→decode in a single execution (three passes over
// the bitmap); the ceiling keeps that cycle well within the fuzzing
// engine's hang detector while still exercising multi-symbol and
// multi-region code paths.
func fuzzBudget(dataLen int) *membudget.Budget {
	const fuzzBudgetBase = 1 << 20 // 1 MiB
	const fuzzBudgetMax = 2 << 20  // 2 MiB
	b := fuzzBudgetBase + int64(limits.StreamBudgetMultiplier)*int64(dataLen)
	return membudget.New(min(b, fuzzBudgetMax))
}

func TestAllocBitmap(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		p := newTestPool(10000)
		bm, err := p.allocBitmap(16, 8)
		if err != nil {
			t.Fatal(err)
		}
		if bm.Width() != 16 || bm.Height() != 8 {
			t.Fatalf("got %dx%d, want 16x8", bm.Width(), bm.Height())
		}
		// 16 pixels wide = 2 bytes stride, 8 rows = 16 bytes
		if p.live != 16 || p.peak != 16 {
			t.Fatalf("live=%d peak=%d, want 16/16", p.live, p.peak)
		}
	})

	t.Run("exceeded", func(t *testing.T) {
		p := newTestPool(10)
		_, err := p.allocBitmap(100, 100)
		if err == nil {
			t.Fatal("expected error")
		}
		if p.live != 0 {
			t.Fatalf("live = %d on failure, want 0", p.live)
		}
	})

	t.Run("negative_dims", func(t *testing.T) {
		p := newTestPool(10000)
		_, err := p.allocBitmap(-1, 10)
		if err == nil {
			t.Fatal("expected error for negative width")
		}
	})

	t.Run("zero_dims", func(t *testing.T) {
		p := newTestPool(10000)
		bm, err := p.allocBitmap(0, 10)
		if err != nil {
			t.Fatal(err)
		}
		if bm.Width() != 0 {
			t.Fatalf("got width %d, want 0", bm.Width())
		}
		if p.live != 0 {
			t.Fatalf("live changed to %d for zero-size bitmap", p.live)
		}
	})
}

func TestFreeBitmap(t *testing.T) {
	p := newTestPool(10000)
	bm, err := p.allocBitmap(16, 8)
	if err != nil {
		t.Fatal(err)
	}
	p.freeBitmap(bm)
	if p.live != 0 {
		t.Fatalf("live after free = %d, want 0", p.live)
	}
	// peak survives so a second alloc of the same size charges nothing
	if p.peak != 16 {
		t.Fatalf("peak after free = %d, want 16", p.peak)
	}
	if _, err := p.allocBitmap(16, 8); err != nil {
		t.Fatalf("re-alloc after free: %v", err)
	}
}

func TestAllocInts(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		p := newTestPool(1000)
		s, err := p.allocInts(10)
		if err != nil {
			t.Fatal(err)
		}
		if len(s) != 10 {
			t.Fatalf("got len %d, want 10", len(s))
		}
		if p.live != 80 {
			t.Fatalf("live = %d, want 80", p.live)
		}
	})

	t.Run("exceeded", func(t *testing.T) {
		p := newTestPool(10)
		_, err := p.allocInts(100)
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
		segments: make(map[uint32]segmentResult),
		pool:     bitmapPool{budget: membudget.New(100)}, // way too small
	}
	err := d.processStream(stream)
	if err == nil {
		t.Fatal("expected budget exceeded error")
	}
}

func TestBudgetSufficient_Normal(t *testing.T) {
	// verify that Decode works with a generous budget
	bm := makeCheckerboard(32, 32)
	segData := EncodeGenericRegionSegment(bm, 0, 0, 1, bitmap.CombOpOR, false, false)
	pageData := WritePageInfo(nil, 32, 32)

	var stream []byte
	stream = WriteSegmentHeader(stream, 0, segPageInfo, 1, nil, uint32(len(pageData)))
	stream = append(stream, pageData...)
	stream = WriteSegmentHeader(stream, 1, segImmediateGeneric, 1, nil, uint32(len(segData)))
	stream = append(stream, segData...)

	got, err := Decode(nil, stream, membudget.New(8<<20))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if !bitmapsEqual(got, bm) {
		t.Error("round-trip mismatch")
	}
}

func TestMaxBitplanes(t *testing.T) {
	// verify that gsbpp > maxBitplanes is rejected
	p := newTestPool(1 << 30)
	_, err := decodeGrayScaleImage(p, nil, false, 0, 17, 4, 4, false, nil)
	if err == nil {
		t.Fatal("expected error for 17 bitplanes")
	}
}
