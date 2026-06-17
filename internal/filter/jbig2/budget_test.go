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

// fuzzWorkBudget returns the cumulative pixel-decode work budget for untrusted
// fuzz input.  Like [workLimit] it is input-proportional, but with a much
// smaller base and ceiling: a compressible JBIG2 stream can drive billions of
// decode operations from a few bytes, which only manifests as a fuzzing-engine
// timeout, so the work cap must keep each pathological input fast.
func fuzzWorkBudget(dataLen int) *membudget.Budget {
	const fuzzWorkBase = 1 << 20 // 1M pixel-ops
	const fuzzWorkMax = 8 << 20  // 8M pixel-ops
	b := fuzzWorkBase + int64(workBudgetPerByte)*int64(dataLen)
	return membudget.New(min(b, fuzzWorkMax))
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

// genericRegionStream builds a JBIG2 page stream of an n-pixel-wide square
// page followed by count generic-region segments, each decoding bm.
func genericRegionStream(bm *bitmap.Bitmap, count int) []byte {
	segData := EncodeGenericRegionSegment(bm, 0, 0, 1, bitmap.CombOpOR, false, false)
	pageData := WritePageInfo(nil, bm.Width(), bm.Height())

	var stream []byte
	stream = WriteSegmentHeader(stream, 0, segPageInfo, 1, nil, uint32(len(pageData)))
	stream = append(stream, pageData...)
	for i := range count {
		stream = WriteSegmentHeader(stream, uint32(i+1), segImmediateGeneric, 1, nil, uint32(len(segData)))
		stream = append(stream, segData...)
	}
	return stream
}

func TestWorkBudget(t *testing.T) {
	// each 256x256 region costs 65536 units of decode work
	bm := makeCheckerboard(256, 256)

	t.Run("exceeded", func(t *testing.T) {
		// two regions; the work budget admits only one.  The second region's
		// up-front work charge fails before its decode loop runs, so this
		// asserts the cap deterministically without relying on wall-clock
		// timing.  Removing the chargeWork call makes both regions decode and
		// fails this test via the t.Fatal below.
		stream := genericRegionStream(bm, 2)
		d := &decoder{
			segments: make(map[uint32]segmentResult),
			pool:     bitmapPool{budget: membudget.New(1 << 20), work: membudget.New(70000)},
		}
		if err := d.processStream(stream); err == nil {
			t.Fatal("expected work budget exceeded error")
		}
	})

	t.Run("sufficient", func(t *testing.T) {
		// a single region within the same budget decodes without error,
		// confirming the cap does not reject legitimate input
		stream := genericRegionStream(bm, 1)
		d := &decoder{
			segments: make(map[uint32]segmentResult),
			pool:     bitmapPool{budget: membudget.New(1 << 20), work: membudget.New(70000)},
		}
		if err := d.processStream(stream); err != nil {
			t.Fatalf("single region within budget: %v", err)
		}
	})
}

// textRegionStream builds a JBIG2 page stream with a one-symbol dictionary
// followed by an immediate text region that composites that symbol count
// times.  All instances reference the single decoded symbol, so the only
// work that grows with count is the per-instance composite.
func textRegionStream(sym *bitmap.Bitmap, count int) []byte {
	symbols := []*bitmap.Bitmap{sym}
	instances := make([]SymbolInstance, count)
	for i := range instances {
		instances[i] = SymbolInstance{SymID: 0, Wi: sym.Width(), Hi: sym.Height()}
	}
	sdData := EncodeSymbolDictSegment(symbols, 1)
	trData := EncodeTextRegionSegment(
		sym.Width(), sym.Height(), 0, 0, instances, symbols,
		cornerTopLeft, false, bitmap.CombOpOR, 1, 0, 0)

	var stream []byte
	stream = WriteSegmentHeader(stream, 0, segSymbolDict, 0, nil, uint32(len(sdData)))
	stream = append(stream, sdData...)
	pageData := WritePageInfo(nil, sym.Width(), sym.Height())
	stream = WriteSegmentHeader(stream, 1, segPageInfo, 1, nil, uint32(len(pageData)))
	stream = append(stream, pageData...)
	stream = WriteSegmentHeader(stream, 2, segImmediateTextRegion, 1, []uint32{0}, uint32(len(trData)))
	stream = append(stream, trData...)
	return stream
}

func TestTextRegionWorkBudget(t *testing.T) {
	// 128x128 symbol = 16384 units of composite work per instance
	sym := makeCheckerboard(128, 128)
	const symArea = 128 * 128

	t.Run("exceeded", func(t *testing.T) {
		// many instances of the one symbol; each composite charges the symbol
		// area, so the cumulative work exceeds the budget even though the
		// symbol is decoded only once.  Dropping the per-instance chargeWork
		// in decodeTextRegion makes this decode succeed and fails this test.
		stream := textRegionStream(sym, 200)
		d := &decoder{
			segments: make(map[uint32]segmentResult),
			pool:     bitmapPool{budget: membudget.New(8 << 20), work: membudget.New(50 * symArea)},
		}
		if err := d.processStream(stream); err == nil {
			t.Fatal("expected work budget exceeded error")
		}
	})

	t.Run("sufficient", func(t *testing.T) {
		// a few instances within the same budget decode without error
		stream := textRegionStream(sym, 4)
		d := &decoder{
			segments: make(map[uint32]segmentResult),
			pool:     bitmapPool{budget: membudget.New(8 << 20), work: membudget.New(50 * symArea)},
		}
		if err := d.processStream(stream); err != nil {
			t.Fatalf("few instances within budget: %v", err)
		}
	})
}

// halftoneRegionStream builds a JBIG2 page stream with a two-pattern
// dictionary followed by an immediate halftone region.  The grid steps by a
// single pixel (hrx = 256, hry = 0), so the patterns overlap heavily and the
// region stays small while each of the gsw*gsh cells still composites a full
// pattern.
func halftoneRegionStream(patterns []*bitmap.Bitmap, gsw, gsh int) []byte {
	pw, ph := patterns[0].Width(), patterns[0].Height()
	grayValues := make([]int, gsw*gsh)
	for i := range grayValues {
		grayValues[i] = i & 1
	}
	const hrx, hry = 256, 0
	width := gsw + pw
	height := gsh + ph

	patData := EncodePatternDictSegment(patterns, 1)
	htData := EncodeHalftoneRegionSegment(
		width, height, grayValues, gsw, gsh,
		0, 0, hrx, hry, len(patterns), 1, bitmap.CombOpOR, false, 0, 0)

	var stream []byte
	stream = WriteSegmentHeader(stream, 0, segPatternDict, 0, nil, uint32(len(patData)))
	stream = append(stream, patData...)
	pageData := WritePageInfo(nil, width, height)
	stream = WriteSegmentHeader(stream, 1, segPageInfo, 1, nil, uint32(len(pageData)))
	stream = append(stream, pageData...)
	stream = WriteSegmentHeader(stream, 2, segImmediateHalftone, 1, []uint32{0}, uint32(len(htData)))
	stream = append(stream, htData...)
	return stream
}

func TestHalftoneWorkBudget(t *testing.T) {
	// 64x64 patterns => 4096 units of composite work per grid cell
	patterns := []*bitmap.Bitmap{
		makeAllZeros(64, 64),
		makeCheckerboard(64, 64),
	}
	// 32x32 grid => 1024 cells.  The loop iterates only 1024 times and the
	// single gray bitplane charges just gsw*gsh = 1024, but placement
	// composites a full 64x64 pattern per cell for 1024*4096 ~= 4.2M units
	// of real work.  A budget that admits the bitplane and pattern-dict
	// decodes therefore still rejects placement once it is charged the
	// pattern-area cost.  Charging only the gsw*gsh grid cells (ignoring the
	// per-cell pattern area) would let this stream decode within the budget,
	// so this test pins the full-product charge.
	const gsw, gsh = 32, 32
	stream := halftoneRegionStream(patterns, gsw, gsh)

	t.Run("exceeded", func(t *testing.T) {
		d := &decoder{
			segments: make(map[uint32]segmentResult),
			pool:     bitmapPool{budget: membudget.New(1 << 20), work: membudget.New(100000)},
		}
		if err := d.processStream(stream); err == nil {
			t.Fatal("expected work budget exceeded error")
		}
	})

	t.Run("sufficient", func(t *testing.T) {
		d := &decoder{
			segments: make(map[uint32]segmentResult),
			pool:     bitmapPool{budget: membudget.New(1 << 20), work: membudget.New(8 << 20)},
		}
		if err := d.processStream(stream); err != nil {
			t.Fatalf("halftone within budget: %v", err)
		}
	})
}

func TestWorkLimit(t *testing.T) {
	if got := workLimit(0); got != workBudgetBase {
		t.Errorf("workLimit(0) = %d, want %d", got, workBudgetBase)
	}
	if got := workLimit(1000); got != workBudgetBase+workBudgetPerByte*1000 {
		t.Errorf("workLimit(1000) = %d, want %d", got, workBudgetBase+workBudgetPerByte*1000)
	}
	if got := workLimit(1 << 40); got != workBudgetHardCap {
		t.Errorf("workLimit(huge) = %d, want hard cap %d", got, workBudgetHardCap)
	}
	if got := workLimit(-1); got != workBudgetBase {
		t.Errorf("workLimit(-1) = %d, want %d", got, workBudgetBase)
	}
}
