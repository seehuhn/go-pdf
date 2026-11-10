// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package jbig2

import (
	"fmt"
	"testing"

	"seehuhn.de/go/membudget"
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

// testPool returns a bitmapPool with a generous memory budget for tests.
func testPool() *bitmapPool {
	return &bitmapPool{budget: membudget.New(1 << 30)}
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
				got, err := decodeGenericRegion(testPool(), dec, p, nil)
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

func TestGenericRegionTPGDON(t *testing.T) {
	// horizontal stripes have many duplicate rows, so TPGDON should
	// produce smaller output
	bm := makeHStripes(64, 64)

	for _, tmpl := range []int{0, 1, 2, 3} {
		t.Run(fmt.Sprintf("T%d", tmpl), func(t *testing.T) {
			with := EncodeGenericRegionSegment(bm, 0, 0, tmpl, bitmap.CombOpOR, true, false)

			// round-trip the TPGDON-encoded version
			var stream []byte
			pageData := WritePageInfo(nil, bm.Width(), bm.Height())
			stream = WriteSegmentHeader(stream, 0, segPageInfo, 1, nil, uint32(len(pageData)))
			stream = append(stream, pageData...)
			stream = WriteSegmentHeader(stream, 1, segImmediateGeneric, 1, nil, uint32(len(with)))
			stream = append(stream, with...)

			got, err := Decode(nil, stream, testBudget())
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			if !bitmapsEqual(got, bm) {
				t.Errorf("round-trip mismatch with TPGDON")
			}
		})
	}

	// verify TPGDON reduces size for an image with many duplicate rows:
	// thick horizontal stripes (8 rows per stripe)
	big := bitmap.New(128, 128)
	for y := range 128 {
		if (y/8)%2 == 0 {
			for x := range 128 {
				big.SetPixel(x, y, true)
			}
		}
	}
	without := EncodeGenericRegionSegment(big, 0, 0, 0, bitmap.CombOpOR, false, false)
	with := EncodeGenericRegionSegment(big, 0, 0, 0, bitmap.CombOpOR, true, false)
	if len(with) >= len(without) {
		t.Errorf("TPGDON did not reduce size for thick stripes: %d >= %d",
			len(with), len(without))
	}
}

func TestGenericRegionExtTemplate(t *testing.T) {
	patterns := []*bitmap.Bitmap{
		makeDiagonal(32, 32),
		makeCheckerboard(32, 32),
		makeCenterBlock(32, 32),
	}

	for _, bm := range patterns {
		// round-trip with extended template
		segData := EncodeGenericRegionSegment(bm, 0, 0, 0, bitmap.CombOpOR, false, true)

		var stream []byte
		pageData := WritePageInfo(nil, bm.Width(), bm.Height())
		stream = WriteSegmentHeader(stream, 0, segPageInfo, 1, nil, uint32(len(pageData)))
		stream = append(stream, pageData...)
		stream = WriteSegmentHeader(stream, 1, segImmediateGeneric, 1, nil, uint32(len(segData)))
		stream = append(stream, segData...)

		got, err := Decode(nil, stream, testBudget())
		if err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if !bitmapsEqual(got, bm) {
			t.Errorf("round-trip mismatch with ExtTemplate")
		}
	}
}

// fuzzBitmapRoundTrip verifies that a bitmap decoded from fuzzed input
// survives a generic-region encode/decode cycle unchanged.
func fuzzBitmapRoundTrip(t *testing.T, data []byte) {
	t.Helper()

	// the first decode processes untrusted input, so it gets the same
	// input-proportional memory and work budgets a real caller would impose
	d1 := &decoder{
		segments:         make(map[uint32]segmentResult),
		prescannedHeight: prescanPageHeight(data),
		pool:             bitmapPool{budget: fuzzBudget(len(data)), work: fuzzWorkBudget(len(data))},
	}
	if err := d1.processStream(data); err != nil || d1.pageBitmap == nil {
		return
	}
	bm1 := d1.pageBitmap
	if bm1.Width() == 0 || bm1.Height() == 0 {
		return
	}

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
			segData := EncodeGenericRegionSegment(pat.bm, 0, 0, tmpl, bitmap.CombOpOR, false, false)

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
		{"mq_image_template0", 0, makeDiagonal(16, 16), "D2 4E 0B D3 4B 60 EF FF AC"},
		{"mq_image_template1", 1, makeDiagonal(16, 16), "CD 97 C1 AD 90 3F FF AC"},
		{"mq_image_template2", 2, makeDiagonal(16, 16), "C4 2E D6 C1 99 FF AC"},
		{"mq_image_template3", 3, makeDiagonal(16, 16), "D2 4E 0E 39 FF 7F FF AC"},
		{"mq_image_templates_zeros", 0, makeAllZeros(16, 16), "AB FF AC"},
		{"mq_image_pattern_zeros", 1, makeAllZeros(16, 16), "AB FF AC"},
		{"mq_image_pattern_checkerboard", 1, makeCheckerboard(16, 16), "C0 F8 48 AF E0 04 7F FF AC"},
		{"mq_image_pattern_hstripes", 1, makeHStripes(16, 16), "FE D5 EB 8B B1 FF 7F F8"},
		{"mq_image_pattern_vstripes", 1, makeVStripes(16, 16), "C0 8E 33 36 7B D7 FF AC"},
		{"mq_image_pattern_center_block", 1, makeCenterBlock(16, 16), "A0 03 D9 4E 16 66 42 3F 95 F0 4F FF AC"},
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

// TestGenericDecodeStreamEnd checks that the decoder gives the same image
// whether the arithmetic-coded data end with the terminating marker
// 0xFF 0xAC, with a bare 0xFF, or without any part of the marker.  In all
// three cases the decoder must supply 1-bits once the data are used up.
func TestGenericDecodeStreamEnd(t *testing.T) {
	tests := []struct {
		name     string
		template int
		bm       *bitmap.Bitmap
		data     string // without terminating marker
	}{
		{"template0_diagonal", 0, makeDiagonal(16, 16), "D2 4E 0B D3 4B 60 EF"},
		{"template1_checkerboard", 1, makeCheckerboard(16, 16), "C0 F8 48 AF E0 04 7F"},
		{"template1_hstripes", 1, makeHStripes(16, 16), "FE D5 EB 8B B1 FF 7F F8"},
		{"template1_center", 1, makeCenterBlock(16, 16), "A0 03 D9 4E 16 66 42 3F 95 F0 4F"},
		{"template2_diagonal", 2, makeDiagonal(16, 16), "C4 2E D6 C1 99"},
		{"template3_diagonal", 3, makeDiagonal(16, 16), "D2 4E 0E 39 FF 7F"},
	}
	endings := []struct {
		name   string
		suffix []byte
	}{
		{"no marker", nil},
		{"0xFF", []byte{0xFF}},
		{"0xFF 0xAC", []byte{0xFF, 0xAC}},
	}

	for _, tt := range tests {
		p := &genericRegionParams{
			Width:    tt.bm.Width(),
			Height:   tt.bm.Height(),
			Template: tt.template,
		}
		if tt.template == 0 {
			p.ATX = [12]int8{3, -3, 2, -2}
			p.ATY = [12]int8{-1, -1, -2, -2}
		} else {
			p.ATX[0] = 2
			if tt.template == 1 {
				p.ATX[0] = 3
			}
			p.ATY[0] = -1
		}
		for _, e := range endings {
			t.Run(tt.name+"/"+e.name, func(t *testing.T) {
				base := hexBytes(tt.data)
				data := make([]byte, len(base), len(base)+len(e.suffix))
				copy(data, base)
				data = append(data, e.suffix...)

				got, err := decodeGenericRegion(testPool(), newMQDecoder(data), p, nil)
				if err != nil {
					t.Fatal(err)
				}
				if !bitmapsEqual(got, tt.bm) {
					t.Errorf("decoded bitmap differs")
				}
			})
		}
	}
}
