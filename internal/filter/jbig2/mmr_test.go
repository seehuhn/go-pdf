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

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf/graphics/bitmap"
)

func TestEncodeMMRRoundTrip(t *testing.T) {
	patterns := []struct {
		name string
		bm   *bitmap.Bitmap
	}{
		{"diagonal_16", makeDiagonal(16, 16)},
		{"zeros_16", makeAllZeros(16, 16)},
		{"checker_16", makeCheckerboard(16, 16)},
		{"hstripes_16", makeHStripes(16, 16)},
		{"vstripes_16", makeVStripes(16, 16)},
		{"center_16", makeCenterBlock(16, 16)},
		{"diagonal_64x32", makeDiagonal(64, 32)},
	}

	for _, tc := range patterns {
		t.Run(tc.name, func(t *testing.T) {
			data, err := encodeMMR(tc.bm)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			got, _, err := decodeMMR(data, tc.bm.Width(), tc.bm.Height())
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if diff := cmp.Diff(tc.bm.Pix, got.Pix); diff != "" {
				t.Errorf("round trip failed (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEncodeMMRNonByteAligned(t *testing.T) {
	widths := []int{1, 7, 13, 25, 31, 33}
	for _, w := range widths {
		t.Run(fmt.Sprintf("w%d", w), func(t *testing.T) {
			bm := bitmap.New(w, 10)
			// diagonal pattern
			for y := range 10 {
				for x := range w {
					if (x+y)%3 == 0 {
						bm.SetPixel(x, y, true)
					}
				}
			}

			data, err := encodeMMR(bm)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			got, _, err := decodeMMR(data, w, 10)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if diff := cmp.Diff(bm.Pix, got.Pix); diff != "" {
				t.Errorf("round trip failed (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGenericRegionMMRRoundTrip(t *testing.T) {
	patterns := []struct {
		name string
		bm   *bitmap.Bitmap
	}{
		{"diagonal_16", makeDiagonal(16, 16)},
		{"zeros_16", makeAllZeros(16, 16)},
		{"checker_16", makeCheckerboard(16, 16)},
		{"hstripes_16", makeHStripes(16, 16)},
		{"vstripes_16", makeVStripes(16, 16)},
		{"center_16", makeCenterBlock(16, 16)},
		{"diagonal_64x32", makeDiagonal(64, 32)},
	}

	for _, tc := range patterns {
		t.Run(tc.name, func(t *testing.T) {
			segData, err := EncodeGenericRegionSegmentMMR(tc.bm, 0, 0, bitmap.CombOpOR)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}

			// wrap in segment stream
			var stream []byte
			pageData := WritePageInfo(nil, tc.bm.Width(), tc.bm.Height())
			stream = WriteSegmentHeader(stream, 0, segPageInfo, 1, nil, uint32(len(pageData)))
			stream = append(stream, pageData...)
			stream = WriteSegmentHeader(stream, 1, segImmediateGeneric, 1, nil, uint32(len(segData)))
			stream = append(stream, segData...)

			got, err := Decode(nil, stream)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if diff := cmp.Diff(tc.bm.Pix, got.Pix); diff != "" {
				t.Errorf("round trip failed (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGrayScaleMMREncodeRoundTrip(t *testing.T) {
	gsw, gsh := 4, 3
	grayValues := []int{
		0, 1, 2, 3,
		3, 2, 1, 0,
		1, 3, 0, 2,
	}

	encoded, err := encodeGrayScaleImageMMR(grayValues, gsw, gsh)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := decodeGrayScaleImage(encoded, true, 0, 2, gsw, gsh, false, nil)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for i := range grayValues {
		if decoded[i] != grayValues[i] {
			t.Errorf("gray[%d]: got %d, want %d", i, decoded[i], grayValues[i])
		}
	}
}

func TestPatternDictMMRRoundTrip(t *testing.T) {
	pw, ph := 8, 8
	patterns := []*bitmap.Bitmap{
		makeAllZeros(pw, ph),
		makeCheckerboard(pw, ph),
		makeDiagonal(pw, ph),
		makeCenterBlock(pw, ph),
	}

	patData, err := encodePatternDictSegmentMMR(patterns)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	// wrap in segment stream
	var stream []byte
	stream = WriteSegmentHeader(stream, 0, segPatternDict, 0, nil, uint32(len(patData)))
	stream = append(stream, patData...)

	pageData := WritePageInfo(nil, 1, 1)
	stream = WriteSegmentHeader(stream, 1, segPageInfo, 1, nil, uint32(len(pageData)))
	stream = append(stream, pageData...)

	d := &decoder{
		segments:  make(map[uint32]segmentResult),
		inputSize: len(stream),
	}
	if err := d.processStream(stream); err != nil {
		t.Fatalf("decode: %v", err)
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

func TestHalftoneMMRRoundTrip(t *testing.T) {
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
	hry := 0
	hgx := 0
	hgy := 0
	width := gsw * pw
	height := gsh * ph

	// encode pattern dictionary (MMR)
	patData, err := encodePatternDictSegmentMMR(patterns)
	if err != nil {
		t.Fatalf("encode pattern dict: %v", err)
	}

	// encode halftone region (MMR)
	htData, err := encodeHalftoneRegionSegmentMMR(
		width, height,
		grayValues, gsw, gsh,
		hgx, hgy, hrx, hry,
		bitmap.CombOpOR,
	)
	if err != nil {
		t.Fatalf("encode halftone: %v", err)
	}

	// wrap in segment stream
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
		t.Fatalf("decode: %v", err)
	}

	// build expected bitmap
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
}

func FuzzMMRRoundTrip(f *testing.F) {
	// seed with pixel data for small bitmaps
	f.Add(8, 8, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
	f.Add(16, 4, []byte{0xAA, 0x55, 0x55, 0xAA, 0xAA, 0x55, 0x55, 0xAA})
	f.Add(13, 3, []byte{0xA5, 0xA0, 0x5A, 0x40, 0xA5, 0xA0})

	f.Fuzz(func(t *testing.T, width, height int, pixData []byte) {
		if width <= 0 || width > 256 || height <= 0 || height > 256 {
			return
		}
		stride := (width + 7) / 8
		if len(pixData) < stride*height {
			return
		}

		// build bitmap from fuzz data
		bm := bitmap.New(width, height)
		for y := range height {
			copy(bm.Pix[y*bm.Stride:], pixData[y*stride:(y+1)*stride])
		}
		// clear padding bits
		if width%8 != 0 {
			mask := byte(0xFF) << (8 - width%8)
			for y := range height {
				bm.Pix[y*bm.Stride+stride-1] &= mask
			}
		}

		data, err := encodeMMR(bm)
		if err != nil {
			t.Fatalf("encode failed: %v", err)
		}
		got, _, err := decodeMMR(data, width, height)
		if err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if diff := cmp.Diff(bm.Pix, got.Pix); diff != "" {
			t.Errorf("round trip failed (-want +got):\n%s", diff)
		}
	})
}
