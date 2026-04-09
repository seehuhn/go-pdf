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
	"bytes"
	"fmt"
	"testing"

	"seehuhn.de/go/pdf/graphics/bitmap"
	"seehuhn.de/go/pdf/internal/filter/ccittfax"
)

func TestGrayScaleSingleBitplane(t *testing.T) {
	gsw, gsh := 4, 3
	grayValues := []int{
		0, 1, 0, 1,
		1, 0, 1, 0,
		0, 1, 0, 1,
	}
	tmpl := 1
	encoded := encodeGrayScaleImage(grayValues, gsw, gsh, 1, tmpl, nil)
	decoded, err := decodeGrayScaleImage(testBudget(), encoded, false, tmpl, 1, gsw, gsh, false, nil)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	for i := range grayValues {
		if decoded[i] != grayValues[i] {
			t.Errorf("gray[%d]: got %d, want %d", i, decoded[i], grayValues[i])
		}
	}
}

// TestGrayScaleMMRRoundTrip verifies that MMR-coded bitplanes are decoded
// correctly when multiple bitplanes are concatenated. Each bitplane consumes
// a different number of bytes; the decoder must advance past each one.
func TestGrayScaleMMRRoundTrip(t *testing.T) {
	gsw, gsh := 4, 3
	grayValues := []int{
		0, 1, 2, 3,
		3, 2, 1, 0,
		1, 3, 0, 2,
	}

	// determine bits per gray value
	gsbpp := 2

	// extract bitplanes and apply Gray code (same as encodeGrayScaleImage)
	planes := make([]*bitmap.Bitmap, gsbpp)
	for j := range gsbpp {
		planes[j] = bitmap.New(gsw, gsh)
		for y := range gsh {
			for x := range gsw {
				if grayValues[y*gsw+x]&(1<<j) != 0 {
					planes[j].SetPixel(x, y, true)
				}
			}
		}
	}
	for j := range gsbpp - 1 {
		for y := range gsh {
			for x := range gsw {
				above := planes[j+1].GetPixel(x, y)
				cur := planes[j].GetPixel(x, y)
				planes[j].SetPixel(x, y, above != cur)
			}
		}
	}

	// encode each bitplane as MMR (Group 4), MSB first
	var mmrData []byte
	stride := (gsw + 7) / 8
	for j := gsbpp - 1; j >= 0; j-- {
		var buf bytes.Buffer
		params := &ccittfax.Params{
			Columns:  gsw,
			K:        -1, // Group 4
			BlackIs1: true,
		}
		w, err := ccittfax.NewWriter(&buf, params)
		if err != nil {
			t.Fatalf("ccittfax.NewWriter: %v", err)
		}
		for y := range gsh {
			row := planes[j].Pix[y*planes[j].Stride : y*planes[j].Stride+stride]
			if _, err := w.Write(row); err != nil {
				t.Fatalf("ccittfax write: %v", err)
			}
		}
		if err := w.Close(); err != nil {
			t.Fatalf("ccittfax close: %v", err)
		}
		mmrData = append(mmrData, buf.Bytes()...)
	}

	// decode using MMR mode
	decoded, err := decodeGrayScaleImage(testBudget(), mmrData, true, 0, gsbpp, gsw, gsh, false, nil)
	if err != nil {
		t.Fatalf("decodeGrayScaleImage: %v", err)
	}

	for i := range grayValues {
		if decoded[i] != grayValues[i] {
			t.Errorf("gray[%d]: got %d, want %d", i, decoded[i], grayValues[i])
		}
	}
}

func TestGrayScaleRoundTrip(t *testing.T) {
	gsw, gsh := 4, 3
	grayValues := []int{
		0, 1, 2, 3,
		3, 2, 1, 0,
		1, 3, 0, 2,
	}

	for _, tmpl := range []int{0, 1, 2, 3} {
		t.Run(fmt.Sprintf("T%d", tmpl), func(t *testing.T) {
			encoded := encodeGrayScaleImage(grayValues, gsw, gsh, 2, tmpl, nil)
			decoded, err := decodeGrayScaleImage(testBudget(), encoded, false, tmpl, 2, gsw, gsh, false, nil)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			for i := range grayValues {
				if decoded[i] != grayValues[i] {
					t.Errorf("gray[%d]: got %d, want %d", i, decoded[i], grayValues[i])
				}
			}
		})
	}
}

func TestPatternDictRoundTrip(t *testing.T) {
	pw, ph := 8, 8
	patterns := []*bitmap.Bitmap{
		makeAllZeros(pw, ph),
		makeCheckerboard(pw, ph),
		makeDiagonal(pw, ph),
		makeCenterBlock(pw, ph),
	}

	for _, tmpl := range []int{0, 1, 2, 3} {
		t.Run(fmt.Sprintf("T%d", tmpl), func(t *testing.T) {
			patternDictRoundTrip(t, patterns, tmpl)
		})
	}
}

func patternDictRoundTrip(t *testing.T, patterns []*bitmap.Bitmap, tmpl int) {
	t.Helper()

	patData := encodePatternDictSegment(patterns, tmpl)

	var stream []byte
	stream = WriteSegmentHeader(stream, 0, segPatternDict, 0, nil, uint32(len(patData)))
	stream = append(stream, patData...)

	pageData := WritePageInfo(nil, 1, 1)
	stream = WriteSegmentHeader(stream, 1, segPageInfo, 1, nil, uint32(len(pageData)))
	stream = append(stream, pageData...)

	d := &decoder{
		segments:  make(map[uint32]segmentResult),
		inputSize: len(stream),
		memBudget: 1 << 30,
	}
	if err := d.processStream(stream); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	seg, ok := d.segments[0]
	if !ok || seg.patterns == nil {
		t.Fatalf("no pattern dictionary decoded")
	}
	if len(seg.patterns) != len(patterns) {
		t.Fatalf("got %d patterns, want %d", len(seg.patterns), len(patterns))
	}
	for i, want := range patterns {
		got := seg.patterns[i]
		if !bitmapsEqual(got, want) {
			t.Errorf("pattern %d mismatch", i)
		}
	}
}

func FuzzPatternDictRoundTrip(f *testing.F) {
	pw, ph := 8, 8
	patterns := []*bitmap.Bitmap{
		makeAllZeros(pw, ph),
		makeCheckerboard(pw, ph),
		makeDiagonal(pw, ph),
		makeCenterBlock(pw, ph),
	}

	for _, tmpl := range []int{0, 1, 2, 3} {
		patData := encodePatternDictSegment(patterns, tmpl)
		var stream []byte
		stream = WriteSegmentHeader(stream, 0, segPatternDict, 0, nil, uint32(len(patData)))
		stream = append(stream, patData...)
		pageData := WritePageInfo(nil, 1, 1)
		stream = WriteSegmentHeader(stream, 1, segPageInfo, 1, nil, uint32(len(pageData)))
		stream = append(stream, pageData...)
		f.Add(stream)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		d1 := &decoder{
			segments:  make(map[uint32]segmentResult),
			inputSize: len(data),
		}
		if err := d1.processStream(data); err != nil {
			return
		}
		seg1, ok := d1.segments[0]
		if !ok || len(seg1.patterns) == 0 {
			return
		}

		// re-encode with template 1
		reEncoded := encodePatternDictSegment(seg1.patterns, 1)
		var stream []byte
		stream = WriteSegmentHeader(stream, 0, segPatternDict, 0, nil, uint32(len(reEncoded)))
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
		if !ok || seg2.patterns == nil {
			t.Fatalf("no patterns after re-decode")
		}

		if len(seg2.patterns) != len(seg1.patterns) {
			return
		}
		for i := range seg1.patterns {
			if !bitmapsEqual(seg1.patterns[i], seg2.patterns[i]) {
				t.Errorf("pattern %d round-trip failed", i)
			}
		}
	})
}

func TestHalftoneRoundTrip(t *testing.T) {
	// create 4 test patterns (8x8 each)
	pw, ph := 8, 8
	patterns := []*bitmap.Bitmap{
		makeAllZeros(pw, ph),     // pattern 0: all white
		makeCheckerboard(pw, ph), // pattern 1: checkerboard
		makeDiagonal(pw, ph),     // pattern 2: diagonal
		makeCenterBlock(pw, ph),  // pattern 3: center block
	}

	// gray-scale image: 4x3 grid referencing patterns 0-3
	gsw, gsh := 4, 3
	grayValues := []int{
		0, 1, 2, 3,
		3, 2, 1, 0,
		1, 3, 0, 2,
	}

	// grid origin and step: each pattern placed at pw-pixel intervals
	// hrx, hry are in 1/256 units
	hrx := pw * 256
	hry := 0
	hgx := 0
	hgy := 0

	// region dimensions
	width := gsw * pw
	height := gsh * ph

	for _, tmpl := range []int{0, 1, 2, 3} {
		t.Run(fmt.Sprintf("T%d", tmpl), func(t *testing.T) {
			// encode pattern dictionary segment
			patData := encodePatternDictSegment(patterns, tmpl)

			// encode halftone region segment
			htData := encodeHalftoneRegionSegment(
				width, height,
				grayValues, gsw, gsh,
				hgx, hgy, hrx, hry,
				len(patterns),
				tmpl, bitmap.CombOpOR,
				false, 0, 0,
			)

			// wrap into a JBIG2 page stream:
			// segment 0: pattern dictionary
			// segment 1: page info
			// segment 2: immediate halftone region (refs segment 0)
			var stream []byte

			// segment 0: pattern dictionary
			stream = WriteSegmentHeader(stream, 0, segPatternDict, 0, nil, uint32(len(patData)))
			stream = append(stream, patData...)

			// segment 1: page info
			pageData := WritePageInfo(nil, width, height)
			stream = WriteSegmentHeader(stream, 1, segPageInfo, 1, nil, uint32(len(pageData)))
			stream = append(stream, pageData...)

			// segment 2: immediate halftone region (refers to segment 0)
			stream = WriteSegmentHeader(stream, 2, segImmediateHalftone, 1, []uint32{0}, uint32(len(htData)))
			stream = append(stream, htData...)

			// decode
			got, err := Decode(nil, stream)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			// build expected bitmap by manually placing patterns
			expected := bitmap.New(width, height)
			for mg := range gsh {
				for ng := range gsw {
					gi := grayValues[mg*gsw+ng]
					pat := patterns[gi]
					x := (hgx + mg*hry + ng*hrx) >> 8
					y := (hgy + mg*hrx - ng*hry) >> 8
					expected.Combine(pat, x, y, bitmap.CombOpOR)
				}
			}

			if !bitmapsEqual(got, expected) {
				t.Errorf("round-trip mismatch")
				// show first differing pixel
				for y := range height {
					for x := range width {
						if got.GetPixel(x, y) != expected.GetPixel(x, y) {
							t.Errorf("first diff at (%d, %d): got %v, want %v",
								x, y, got.GetPixel(x, y), expected.GetPixel(x, y))
							return
						}
					}
				}
			}
		})
	}
}

// TestHalftoneRoundTripSparseGray tests that the encoder correctly handles
// gray values that don't span the full pattern range. Before the fix,
// the encoder would derive the bitplane count from max(grayValues) while
// the decoder derives it from the pattern count, causing a mismatch.
func TestHalftoneRoundTripSparseGray(t *testing.T) {
	pw, ph := 8, 8
	patterns := []*bitmap.Bitmap{
		makeAllZeros(pw, ph),
		makeCheckerboard(pw, ph),
		makeDiagonal(pw, ph),
		makeCenterBlock(pw, ph),
	}

	// only use patterns 0 and 1 (not 2 or 3)
	gsw, gsh := 4, 3
	grayValues := []int{
		0, 1, 0, 1,
		1, 0, 1, 0,
		0, 1, 0, 1,
	}

	hrx := pw * 256
	width := gsw * pw
	height := gsh * ph

	patData := encodePatternDictSegment(patterns, 1)
	htData := encodeHalftoneRegionSegment(
		width, height,
		grayValues, gsw, gsh,
		0, 0, hrx, 0,
		len(patterns),
		1, bitmap.CombOpOR,
		false, 0, 0,
	)

	var stream []byte
	stream = WriteSegmentHeader(stream, 0, segPatternDict, 0, nil, uint32(len(patData)))
	stream = append(stream, patData...)
	pageData := WritePageInfo(nil, width, height)
	stream = WriteSegmentHeader(stream, 1, segPageInfo, 1, nil, uint32(len(pageData)))
	stream = append(stream, pageData...)
	stream = WriteSegmentHeader(stream, 2, segImmediateHalftone, 1, []uint32{0}, uint32(len(htData)))
	stream = append(stream, htData...)

	got, err := Decode(nil, stream)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	expected := bitmap.New(width, height)
	for mg := range gsh {
		for ng := range gsw {
			gi := grayValues[mg*gsw+ng]
			pat := patterns[gi]
			expected.Combine(pat, ng*pw, mg*ph, bitmap.CombOpOR)
		}
	}

	if !bitmapsEqual(got, expected) {
		t.Error("round-trip mismatch for sparse gray values")
	}
}

// TestIntermediateHalftoneRoundTrip tests intermediate halftone region
// (type 20) stored as auxiliary buffer, then referenced by a refinement.
func TestIntermediateHalftoneRoundTrip(t *testing.T) {
	pw, ph := 8, 8
	patterns := []*bitmap.Bitmap{
		makeAllZeros(pw, ph),
		makeCheckerboard(pw, ph),
		makeDiagonal(pw, ph),
		makeCenterBlock(pw, ph),
	}

	gsw, gsh := 4, 3
	grayValues := []int{0, 1, 2, 3, 3, 2, 1, 0, 1, 3, 0, 2}
	hrx := pw * 256
	width := gsw * pw
	height := gsh * ph

	patData := encodePatternDictSegment(patterns, 1)
	htData := encodeHalftoneRegionSegment(
		width, height, grayValues, gsw, gsh,
		0, 0, hrx, 0, len(patterns),
		1, bitmap.CombOpOR,
		false, 0, 0)

	// build expected bitmap
	expected := bitmap.New(width, height)
	for mg := range gsh {
		for ng := range gsw {
			gi := grayValues[mg*gsw+ng]
			pat := patterns[gi]
			x := ng * pw
			y := mg * ph
			expected.Combine(pat, x, y, bitmap.CombOpOR)
		}
	}

	// encode a refinement of the halftone bitmap (identity refinement)
	refinData := EncodeRefinementRegionSegment(expected, expected, 0, 0, 1, bitmap.CombOpOR, false)

	var stream []byte

	// segment 0: pattern dictionary (global)
	stream = WriteSegmentHeader(stream, 0, segPatternDict, 0, nil, uint32(len(patData)))
	stream = append(stream, patData...)

	// segment 1: page info
	pageData := WritePageInfo(nil, width, height)
	stream = WriteSegmentHeader(stream, 1, segPageInfo, 1, nil, uint32(len(pageData)))
	stream = append(stream, pageData...)

	// segment 2: intermediate halftone region (type 20, NOT composited)
	stream = WriteSegmentHeader(stream, 2, segIntermediateHalftone, 1, []uint32{0}, uint32(len(htData)))
	stream = append(stream, htData...)

	// segment 3: immediate refinement (type 42) referring to seg 2
	stream = WriteSegmentHeader(stream, 3, segImmediateRefinement, 1, []uint32{2}, uint32(len(refinData)))
	stream = append(stream, refinData...)

	got, err := Decode(nil, stream)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if !bitmapsEqual(got, expected) {
		t.Errorf("round-trip mismatch")
	}
}

func FuzzHalftoneRoundTrip(f *testing.F) {
	pw, ph := 8, 8
	patterns := []*bitmap.Bitmap{
		makeAllZeros(pw, ph),
		makeCheckerboard(pw, ph),
		makeDiagonal(pw, ph),
		makeCenterBlock(pw, ph),
	}

	gsw, gsh := 4, 3
	grayValues := []int{
		0, 1, 2, 3,
		3, 2, 1, 0,
		1, 3, 0, 2,
	}

	hrx := pw * 256
	width := gsw * pw
	height := gsh * ph

	for _, tmpl := range []int{0, 1, 2, 3} {
		patData := encodePatternDictSegment(patterns, tmpl)
		htData := encodeHalftoneRegionSegment(
			width, height, grayValues, gsw, gsh,
			0, 0, hrx, 0, len(patterns),
			tmpl, bitmap.CombOpOR,
			false, 0, 0,
		)

		var stream []byte
		stream = WriteSegmentHeader(stream, 0, segPatternDict, 0, nil, uint32(len(patData)))
		stream = append(stream, patData...)
		pageData := WritePageInfo(nil, width, height)
		stream = WriteSegmentHeader(stream, 1, segPageInfo, 1, nil, uint32(len(pageData)))
		stream = append(stream, pageData...)
		stream = WriteSegmentHeader(stream, 2, segImmediateHalftone, 1, []uint32{0}, uint32(len(htData)))
		stream = append(stream, htData...)
		f.Add(stream)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzBitmapRoundTrip(t, data)
	})
}
