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
	"fmt"
	"testing"

	"seehuhn.de/go/pdf/graphics/bitmap"
)

func TestRefinementDecodeTemplate1(t *testing.T) {
	// from mq_test_vectors.txt: mq_refinement_template1
	// reference=diagonal 16x16, target=checkerboard 16x16
	// template=1, ref_dx=1, ref_dy=-2
	ref := makeDiagonal(16, 16)
	expected := makeCheckerboard(16, 16)

	data := hexBytes("BB D0 31 DE B9 FF")
	dec := newMQDecoder(data)

	p := &refinementParams{
		Width:     16,
		Height:    16,
		Template:  1,
		Reference: ref,
		RefDX:     1,
		RefDY:     -2,
	}
	got, err := decodeRefinementRegion(testPool(), dec, p, nil)
	if err != nil {
		t.Fatal(err)
	}

	if !bitmapsEqual(got, expected) {
		t.Errorf("refinement template 1 decode failed")
	}
}

func TestRefinementRoundTrip(t *testing.T) {
	ref := makeDiagonal(16, 16)
	target := makeCheckerboard(16, 16)

	for _, tmpl := range []int{0, 1} {
		t.Run(fmt.Sprintf("template%d", tmpl), func(t *testing.T) {
			refinementRoundTrip(t, ref, target, tmpl)
		})
	}
}

func refinementRoundTrip(t *testing.T, ref, target *bitmap.Bitmap, tmpl int) {
	t.Helper()

	p := &refinementParams{
		Width:     target.Width(),
		Height:    target.Height(),
		Template:  tmpl,
		Reference: ref,
		RefDX:     0,
		RefDY:     0,
		ATX:       [2]int8{-1, -1},
		ATY:       [2]int8{-1, -1},
	}

	data := encodeRefinementRegion(target, p)

	dec := newMQDecoder(data)
	got, err := decodeRefinementRegion(testPool(), dec, p, nil)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if !bitmapsEqual(got, target) {
		t.Errorf("round-trip failed (data=%d bytes)", len(data))
	}
}

// TestRefinementMQFlush verifies that the MQ encode/decode round-trip
// works for a bitmap produced by decoding fuzzed data with heavy MQ padding.
func TestRefinementMQFlush(t *testing.T) {
	ref := makeDiagonal(16, 16)
	fuzzData := []byte("800010\xc3\xc3\xc3\xc3\xff\xff")

	p := &refinementParams{
		Width:     16,
		Height:    16,
		Template:  1,
		Reference: ref,
		RefDX:     0,
		RefDY:     0,
	}
	dec := newMQDecoder(fuzzData)
	bm1, err := decodeRefinementRegion(testPool(), dec, p, nil)
	if err != nil {
		t.Fatalf("initial decode failed: %v", err)
	}

	encoded := encodeRefinementRegion(bm1, p)

	dec2 := newMQDecoder(encoded)
	bm2, err := decodeRefinementRegion(testPool(), dec2, p, nil)
	if err != nil {
		t.Fatalf("re-decode failed: %v", err)
	}

	// extract the MQ decisions and contexts from the encoding
	type decision struct {
		ctx uint16
		bit int
	}
	var decisions []decision
	cx := make([]byte, 1<<10) // template 1
	for y := range bm1.Height() {
		for x := range bm1.Width() {
			context := buildRefContext(bm1, p, x, y)
			d := getPixel(bm1, x, y)
			decisions = append(decisions, decision{context, d})
			// mirror what the encoder does to cx
			// (we don't actually need to do this since we
			// captured the context before encoding)
			_ = cx[context]
		}
	}

	// test pure MQ round-trip with the same decisions
	enc := newMQEncoder()
	cx = make([]byte, 1<<10)
	for _, d := range decisions {
		enc.encode(&cx[d.ctx], d.bit)
	}
	enc.flush()
	mqData := enc.bytes()

	// trace encoder state for the last few decisions
	enc2 := newMQEncoder()
	cx2 := make([]byte, 1<<10)
	for i, d := range decisions {
		if i >= 253 {
			t.Logf("ENC[%d] ctx=%d bit=%d  A=%04X C=%08X CT=%d bp=%d",
				i, d.ctx, d.bit, enc2.a, enc2.c, enc2.ct, enc2.bp)
		}
		enc2.encode(&cx2[d.ctx], d.bit)
	}
	t.Logf("ENC pre-flush  A=%04X C=%08X CT=%d bp=%d", enc2.a, enc2.c, enc2.ct, enc2.bp)
	enc2.flush()
	t.Logf("ENC post-flush bp=%d bytes=%X", enc2.bp, enc2.bytes())

	mqDec := newMQDecoder(mqData)
	cx = make([]byte, 1<<10)
	for i, want := range decisions {
		if i >= 253 {
			t.Logf("DEC[%d] ctx=%d  A=%04X C=%08X CT=%d bp=%d exhausted=%v",
				i, want.ctx, mqDec.a, mqDec.c, mqDec.ct, mqDec.bp, mqDec.exhausted)
		}
		got := mqDec.decode(&cx[want.ctx])
		if got != want.bit {
			t.Errorf("MQ decision %d (pixel %d,%d): got %d, want %d",
				i, i%16, i/16, got, want.bit)
			t.Errorf("context was %d, mqData: %X (%d bytes)",
				want.ctx, mqData, len(mqData))
			break
		}
	}

	for y := range bm1.Height() {
		for x := range bm1.Width() {
			if bm1.GetPixel(x, y) != bm2.GetPixel(x, y) {
				t.Errorf("pixel (%d, %d) differs: bm1=%v, bm2=%v",
					x, y, bm1.GetPixel(x, y), bm2.GetPixel(x, y))
				t.Errorf("bm1.Pix: %X", bm1.Pix)
				t.Errorf("bm2.Pix: %X", bm2.Pix)
				t.Errorf("encoded: %X (%d bytes)", encoded, len(encoded))
				return
			}
		}
	}
}

func FuzzRefinementRoundTrip(f *testing.F) {
	ref := makeDiagonal(16, 16)
	target := makeCheckerboard(16, 16)

	// seed from both templates
	for _, tmpl := range []int{0, 1} {
		p := &refinementParams{
			Width:     16,
			Height:    16,
			Template:  tmpl,
			Reference: ref,
			RefDX:     0,
			RefDY:     0,
			ATX:       [2]int8{-1, -1},
			ATY:       [2]int8{-1, -1},
		}
		data := encodeRefinementRegion(target, p)
		f.Add(data)
	}

	// targeted debug cases from fuzz
	f.Add([]byte{0xbb, 0x37, 0xff, 0xb5})
	f.Add([]byte("800010\xc3\xc3\xc3\xc3\xff\xff"))

	f.Fuzz(func(t *testing.T, data []byte) {
		p := &refinementParams{
			Width:     16,
			Height:    16,
			Template:  1,
			Reference: ref,
			RefDX:     0,
			RefDY:     0,
		}
		dec := newMQDecoder(data)
		bm1, err := decodeRefinementRegion(testPool(), dec, p, nil)
		if err != nil {
			return
		}

		encoded := encodeRefinementRegion(bm1, p)

		dec2 := newMQDecoder(encoded)
		bm2, err := decodeRefinementRegion(testPool(), dec2, p, nil)
		if err != nil {
			t.Fatalf("re-decode failed: %v", err)
		}

		if !bitmapsEqual(bm1, bm2) {
			t.Errorf("round-trip failed")
		}
	})
}

func TestRefinementRegionTPGRON(t *testing.T) {
	// use a target that closely matches the reference (many typical pixels)
	ref := makeDiagonal(32, 32)
	target := makeDiagonal(32, 32)
	// flip a few pixels so it's not identical
	target.SetPixel(5, 5, !target.GetPixel(5, 5))
	target.SetPixel(10, 10, !target.GetPixel(10, 10))

	for _, tmpl := range []int{0, 1} {
		t.Run(fmt.Sprintf("T%d", tmpl), func(t *testing.T) {
			var stream []byte
			pageData := WritePageInfo(nil, 32, 32)
			stream = WriteSegmentHeader(stream, 0, segPageInfo, 1, nil, uint32(len(pageData)))
			stream = append(stream, pageData...)

			refData := EncodeGenericRegionSegment(ref, 0, 0, 1, bitmap.CombOpOR, false, false)
			stream = WriteSegmentHeader(stream, 1, segIntermediateGeneric, 1, nil, uint32(len(refData)))
			stream = append(stream, refData...)

			refinData := EncodeRefinementRegionSegment(target, ref, 0, 0, tmpl, bitmap.CombOpOR, true)
			stream = WriteSegmentHeader(stream, 2, segImmediateRefinement, 1, []uint32{1}, uint32(len(refinData)))
			stream = append(stream, refinData...)

			got, err := Decode(nil, stream, testBudget())
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			if !bitmapsEqual(got, target) {
				t.Errorf("round-trip mismatch with TPGRON")
			}
		})
	}
}

// TestRefinementRegionSegmentRoundTrip tests case d from T.88 §7.4.8.6:
// an immediate refinement region segment referring to an intermediate
// generic region segment.
func TestRefinementRegionSegmentRoundTrip(t *testing.T) {
	ref := makeDiagonal(16, 16)
	target := makeCheckerboard(16, 16)

	for _, tmpl := range []int{0, 1} {
		t.Run(fmt.Sprintf("T%d", tmpl), func(t *testing.T) {
			// segment 0: page info
			var stream []byte
			pageData := WritePageInfo(nil, 16, 16)
			stream = WriteSegmentHeader(stream, 0, segPageInfo, 1, nil, uint32(len(pageData)))
			stream = append(stream, pageData...)

			// segment 1: intermediate generic region (reference bitmap)
			refData := EncodeGenericRegionSegment(ref, 0, 0, 1, bitmap.CombOpOR, false, false)
			stream = WriteSegmentHeader(stream, 1, segIntermediateGeneric, 1, nil, uint32(len(refData)))
			stream = append(stream, refData...)

			// segment 2: immediate refinement region referring to segment 1
			refinData := EncodeRefinementRegionSegment(target, ref, 0, 0, tmpl, bitmap.CombOpOR, false)
			stream = WriteSegmentHeader(stream, 2, segImmediateRefinement, 1, []uint32{1}, uint32(len(refinData)))
			stream = append(stream, refinData...)

			got, err := Decode(nil, stream, testBudget())
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			if !bitmapsEqual(got, target) {
				t.Errorf("round-trip mismatch")
			}
		})
	}
}

// TestRefinementRegionPageBuffer tests case c from T.88 §7.4.8.6:
// an immediate refinement region segment with no referred-to segments,
// refining a portion of the page buffer.
func TestRefinementRegionPageBuffer(t *testing.T) {
	ref := makeDiagonal(16, 16)
	target := makeCheckerboard(16, 16)

	for _, tmpl := range []int{0, 1} {
		t.Run(fmt.Sprintf("T%d", tmpl), func(t *testing.T) {
			// segment 0: page info
			var stream []byte
			pageData := WritePageInfo(nil, 16, 16)
			stream = WriteSegmentHeader(stream, 0, segPageInfo, 1, nil, uint32(len(pageData)))
			stream = append(stream, pageData...)

			// segment 1: immediate generic region (places reference on page)
			genData := EncodeGenericRegionSegment(ref, 0, 0, 1, bitmap.CombOpOR, false, false)
			stream = WriteSegmentHeader(stream, 1, segImmediateGeneric, 1, nil, uint32(len(genData)))
			stream = append(stream, genData...)

			// segment 2: immediate refinement with no refs (refines page buffer)
			refinData := EncodeRefinementRegionSegment(target, ref, 0, 0, tmpl, bitmap.CombOpOR, false)
			stream = WriteSegmentHeader(stream, 2, segImmediateRefinement, 1, nil, uint32(len(refinData)))
			stream = append(stream, refinData...)

			got, err := Decode(nil, stream, testBudget())
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			if !bitmapsEqual(got, target) {
				t.Errorf("round-trip mismatch")
			}
		})
	}
}

func FuzzRefinementRegionSegmentRoundTrip(f *testing.F) {
	ref := makeDiagonal(16, 16)
	target := makeCheckerboard(16, 16)

	for _, tmpl := range []int{0, 1} {
		// seed: intermediate ref + immediate refinement
		var stream []byte
		pageData := WritePageInfo(nil, 16, 16)
		stream = WriteSegmentHeader(stream, 0, segPageInfo, 1, nil, uint32(len(pageData)))
		stream = append(stream, pageData...)

		refData := EncodeGenericRegionSegment(ref, 0, 0, 1, bitmap.CombOpOR, false, false)
		stream = WriteSegmentHeader(stream, 1, segIntermediateGeneric, 1, nil, uint32(len(refData)))
		stream = append(stream, refData...)

		refinData := EncodeRefinementRegionSegment(target, ref, 0, 0, tmpl, bitmap.CombOpOR, false)
		stream = WriteSegmentHeader(stream, 2, segImmediateRefinement, 1, []uint32{1}, uint32(len(refinData)))
		stream = append(stream, refinData...)

		f.Add(stream)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		bm1, err := Decode(nil, data, testBudget())
		if err != nil || bm1 == nil {
			return
		}
		if bm1.Width() == 0 || bm1.Height() == 0 {
			return
		}

		// re-encode as a simple generic region and decode again
		segData := EncodeGenericRegionSegment(bm1, 0, 0, 1, bitmap.CombOpOR, false, false)
		var stream []byte
		pageData := WritePageInfo(nil, bm1.Width(), bm1.Height())
		stream = WriteSegmentHeader(stream, 0, segPageInfo, 1, nil, uint32(len(pageData)))
		stream = append(stream, pageData...)
		stream = WriteSegmentHeader(stream, 1, segImmediateGeneric, 1, nil, uint32(len(segData)))
		stream = append(stream, segData...)

		bm2, err := Decode(nil, stream, testBudget())
		if err != nil {
			t.Fatalf("re-decode failed: %v", err)
		}

		if !bitmapsEqual(bm1, bm2) {
			t.Errorf("round-trip failed")
		}
	})
}

// TestIntermediateRefinementRoundTrip tests the chain:
// intermediate generic (type 36) → intermediate refinement (type 40) →
// immediate refinement (type 42).
func TestIntermediateRefinementRoundTrip(t *testing.T) {
	initial := makeDiagonal(16, 16)
	middle := makeCheckerboard(16, 16)
	final := makeHStripes(16, 16)

	for _, tmpl := range []int{0, 1} {
		t.Run(fmt.Sprintf("T%d", tmpl), func(t *testing.T) {
			var stream []byte

			// segment 0: page info
			pageData := WritePageInfo(nil, 16, 16)
			stream = WriteSegmentHeader(stream, 0, segPageInfo, 1, nil, uint32(len(pageData)))
			stream = append(stream, pageData...)

			// segment 1: intermediate generic region (initial bitmap)
			genData := EncodeGenericRegionSegment(initial, 0, 0, 1, bitmap.CombOpOR, false, false)
			stream = WriteSegmentHeader(stream, 1, segIntermediateGeneric, 1, nil, uint32(len(genData)))
			stream = append(stream, genData...)

			// segment 2: intermediate refinement (type 40) refining seg 1 → middle
			refin1 := EncodeRefinementRegionSegment(middle, initial, 0, 0, tmpl, bitmap.CombOpOR, false)
			stream = WriteSegmentHeader(stream, 2, segIntermediateRefinement, 1, []uint32{1}, uint32(len(refin1)))
			stream = append(stream, refin1...)

			// segment 3: immediate refinement (type 42) refining seg 2 → final
			refin2 := EncodeRefinementRegionSegment(final, middle, 0, 0, tmpl, bitmap.CombOpOR, false)
			stream = WriteSegmentHeader(stream, 3, segImmediateRefinement, 1, []uint32{2}, uint32(len(refin2)))
			stream = append(stream, refin2...)

			got, err := Decode(nil, stream, testBudget())
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			if !bitmapsEqual(got, final) {
				t.Errorf("round-trip mismatch")
			}
		})
	}
}
