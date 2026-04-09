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

// makeDiagonal creates a 16x16 diagonal bitmap (pixel set when x == y).
func makeDiagonal(w, h int) *bitmap.Bitmap {
	bm := bitmap.New(w, h)
	for i := 0; i < min(w, h); i++ {
		bm.SetPixel(i, i, true)
	}
	return bm
}

func makeAllZeros(w, h int) *bitmap.Bitmap {
	return bitmap.New(w, h)
}

func makeCheckerboard(w, h int) *bitmap.Bitmap {
	bm := bitmap.New(w, h)
	for y := range h {
		for x := range w {
			if (x+y)%2 == 0 {
				bm.SetPixel(x, y, true)
			}
		}
	}
	return bm
}

func makeHStripes(w, h int) *bitmap.Bitmap {
	bm := bitmap.New(w, h)
	for y := range h {
		if y%2 == 0 {
			for x := range w {
				bm.SetPixel(x, y, true)
			}
		}
	}
	return bm
}

func makeVStripes(w, h int) *bitmap.Bitmap {
	bm := bitmap.New(w, h)
	for y := range h {
		for x := range w {
			if x%2 == 0 {
				bm.SetPixel(x, y, true)
			}
		}
	}
	return bm
}

func makeCenterBlock(w, h int) *bitmap.Bitmap {
	bm := bitmap.New(w, h)
	x0, y0 := (w-8)/2, (h-8)/2
	for y := y0; y < y0+8; y++ {
		for x := x0; x < x0+8; x++ {
			bm.SetPixel(x, y, true)
		}
	}
	return bm
}

func bitmapsEqual(a, b *bitmap.Bitmap) bool {
	if a.Width() != b.Width() || a.Height() != b.Height() {
		return false
	}
	for y := 0; y < a.Height(); y++ {
		for x := 0; x < a.Width(); x++ {
			if a.GetPixel(x, y) != b.GetPixel(x, y) {
				return false
			}
		}
	}
	return true
}

func TestGenericRegionRoundTrip(t *testing.T) {
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

	for _, tmpl := range []int{0, 1, 2, 3} {
		for _, pat := range patterns {
			name := fmt.Sprintf("T%d/%s", tmpl, pat.name)
			t.Run(name, func(t *testing.T) {
				p := &genericRegionParams{
					Width:    pat.bm.Width(),
					Height:   pat.bm.Height(),
					Template: tmpl,
				}
				// set nominal AT pixel positions
				switch tmpl {
				case 0:
					p.ATX[0] = 3
					p.ATY[0] = -1
					p.ATX[1] = -3
					p.ATY[1] = -1
					p.ATX[2] = 2
					p.ATY[2] = -2
					p.ATX[3] = -2
					p.ATY[3] = -2
				case 1:
					p.ATX[0] = 3
					p.ATY[0] = -1
				case 2:
					p.ATX[0] = 2
					p.ATY[0] = -1
				case 3:
					p.ATX[0] = 2
					p.ATY[0] = -1
				}

				enc := newMQEncoder()
				encodeGenericRegion(enc, pat.bm, p, nil)
				enc.flush()
				data := enc.bytes()

				dec := newMQDecoder(data)
				got, err := decodeGenericRegion(dec, p, nil)
				if err != nil {
					t.Fatalf("decode error: %v", err)
				}

				if !bitmapsEqual(got, pat.bm) {
					t.Errorf("round-trip failed for %s (data=%d bytes)", name, len(data))
				}
			})
		}
	}
}

// fuzzBitmapRoundTrip verifies that a bitmap decoded from fuzzed input
// survives a generic-region encode/decode cycle unchanged.
func fuzzBitmapRoundTrip(t *testing.T, data []byte) {
	t.Helper()

	bm1, err := Decode(nil, data)
	if err != nil || bm1 == nil {
		return
	}
	if bm1.Width() == 0 || bm1.Height() == 0 {
		return
	}

	segData := EncodeGenericRegionSegment(bm1, 0, 0, 1, bitmap.CombOpOR)
	var stream []byte
	pageData := WritePageInfo(nil, bm1.Width(), bm1.Height())
	stream = WriteSegmentHeader(stream, 0, segPageInfo, 1, nil, uint32(len(pageData)))
	stream = append(stream, pageData...)
	stream = WriteSegmentHeader(stream, 1, segImmediateGeneric, 1, nil, uint32(len(segData)))
	stream = append(stream, segData...)

	bm2, err := Decode(nil, stream)
	if err != nil {
		t.Fatalf("re-decode failed: %v", err)
	}

	if !bitmapsEqual(bm1, bm2) {
		t.Errorf("round-trip failed")
	}
}

func FuzzGenericRegionMQRoundTrip(f *testing.F) {
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

	// seed from test cases across all templates
	for _, tmpl := range []int{0, 1, 2, 3} {
		for _, pat := range patterns {
			segData := EncodeGenericRegionSegment(pat.bm, 0, 0, tmpl, bitmap.CombOpOR)

			var stream []byte
			pageData := WritePageInfo(nil, pat.bm.Width(), pat.bm.Height())
			stream = WriteSegmentHeader(stream, 0, segPageInfo, 1, nil, uint32(len(pageData)))
			stream = append(stream, pageData...)
			stream = WriteSegmentHeader(stream, 1, segImmediateGeneric, 1, nil, uint32(len(segData)))
			stream = append(stream, segData...)
			f.Add(stream)
		}
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzBitmapRoundTrip(t, data)
	})
}

func TestGenericImageVectors(t *testing.T) {
	// from mq_test_vectors.txt
	tests := []struct {
		name     string
		template int
		bm       *bitmap.Bitmap
		expected string
	}{
		{"mq_image_template0", 0, makeDiagonal(16, 16), "D2 4E 0B D3 4B 60 EF FF"},
		{"mq_image_template1", 1, makeDiagonal(16, 16), "CD 97 C1 AD 90 3F FF"},
		{"mq_image_template2", 2, makeDiagonal(16, 16), "C4 2E D6 C1 99 FF"},
		{"mq_image_template3", 3, makeDiagonal(16, 16), "D2 4E 0E 39 FF 7F FF"},
		{"mq_image_templates_zeros", 0, makeAllZeros(16, 16), "AB FF"},
		{"mq_image_pattern_zeros", 1, makeAllZeros(16, 16), "AB FF"},
		{"mq_image_pattern_checkerboard", 1, makeCheckerboard(16, 16), "C0 F8 48 AF E0 04 7F FF"},
		{"mq_image_pattern_hstripes", 1, makeHStripes(16, 16), "FE D5 EB 8B B1 FF 7F F8"},
		{"mq_image_pattern_vstripes", 1, makeVStripes(16, 16), "C0 8E 33 36 7B D7 FF"},
		{"mq_image_pattern_center_block", 1, makeCenterBlock(16, 16), "A0 03 D9 4E 16 66 42 3F 95 F0 4F FF"},
		{"mq_image_pattern_large_diagonal", 1, makeDiagonal(64, 32), "CD 9A 58 4A 89 1A 7F FF 7F F8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &genericRegionParams{
				Width:    tt.bm.Width(),
				Height:   tt.bm.Height(),
				Template: tt.template,
			}
			// nominal AT positions
			switch tt.template {
			case 0:
				p.ATX[0] = 3
				p.ATY[0] = -1
				p.ATX[1] = -3
				p.ATY[1] = -1
				p.ATX[2] = 2
				p.ATY[2] = -2
				p.ATX[3] = -2
				p.ATY[3] = -2
			case 1:
				p.ATX[0] = 3
				p.ATY[0] = -1
			case 2:
				p.ATX[0] = 2
				p.ATY[0] = -1
			case 3:
				p.ATX[0] = 2
				p.ATY[0] = -1
			}

			enc := newMQEncoder()
			encodeGenericRegion(enc, tt.bm, p, nil)
			enc.flush()
			got := enc.bytes()
			expected := hexBytes(tt.expected)

			if fmt.Sprintf("%X", got) != fmt.Sprintf("%X", expected) {
				t.Errorf("%s:\n  got  %X\n  want %X", tt.name, got, expected)
			}
		})
	}
}
