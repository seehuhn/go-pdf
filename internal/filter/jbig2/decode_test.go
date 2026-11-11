// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package jbig2

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"seehuhn.de/go/membudget"
	"seehuhn.de/go/pdf/graphics/bitmap"
	"seehuhn.de/go/pdf/internal/limits"
)

// The test cases in testdata/decode built from a generated text page are
// written by the generate program; see testdata/README.md.
//go:generate go run ./generate/

const testdataDir = "testdata/decode"

func TestDecodeGenericRegions(t *testing.T) {
	// test cases that contain only generic region segments (no symbol dict/text)
	genericTests := []string{
		"test_enc_generic_border",
		"test_enc_generic_plain",
		"test_enc_generic_tpgd",
		"test_enc_page_generic",
		"test_enc_text_defpixel",
		"test_enc_text_tpgd",
		"test_g4_page_mmr",
		"test_t88_gen_ext",
		"test_t88_gen_t0_at",
		"test_t88_gen_t1",
		"test_t88_gen_t2",
		"test_t88_gen_t3",
	}

	for _, name := range genericTests {
		t.Run(name, func(t *testing.T) {
			globals, page, expected := loadTestCase(t, name)
			got, err := Decode(globals, page, testBudget())
			if err != nil {
				t.Fatalf("decode error: %v", err)
			}
			if !bitmapsEqual(got, expected) {
				t.Errorf("decoded bitmap differs from expected (%dx%d vs %dx%d)",
					got.Width(), got.Height(), expected.Width(), expected.Height())
				if got.Width() == expected.Width() && got.Height() == expected.Height() {
					diffCount := countDiffs(got, expected)
					t.Errorf("%d pixels differ", diffCount)
				}
			}
		})
	}
}

func loadTestCase(t *testing.T, name string) (globals, page []byte, expected *bitmap.Bitmap) {
	t.Helper()

	globalsPath := filepath.Join(testdataDir, name+".globals")
	pagePath := filepath.Join(testdataDir, name+".page")
	bmpPath := filepath.Join(testdataDir, name+".bmp")

	var err error
	globals, err = os.ReadFile(globalsPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("reading globals: %v", err)
	}

	page, err = os.ReadFile(pagePath)
	if err != nil {
		t.Fatalf("reading page: %v", err)
	}

	f, err := os.Open(bmpPath)
	if err != nil {
		t.Fatalf("reading bmp: %v", err)
	}
	defer f.Close()
	expected, err = bitmap.ReadBMP(f)
	if err != nil {
		t.Fatalf("parsing bmp: %v", err)
	}

	return globals, page, expected
}

func countDiffs(a, b *bitmap.Bitmap) int {
	count := 0
	for y := 0; y < a.Height(); y++ {
		for x := 0; x < a.Width(); x++ {
			if a.GetPixel(x, y) != b.GetPixel(x, y) {
				count++
			}
		}
	}
	return count
}

func TestCheckBitmapSize(t *testing.T) {
	cases := []struct {
		name      string
		w, h      int
		wantError bool
	}{
		{"valid square", 100, 100, false},
		{"zero width", 0, 100, false},
		{"zero height", 100, 0, false},
		{"both zero", 0, 0, false},
		{"negative width", -1, 100, true},
		{"negative height", 100, -1, true},
		{"width at cap", maxPixels, 1, false},
		{"width above cap", maxPixels + 1, 1, true},
		{"height above cap", 1, maxPixels + 1, true},
		{"pixel count above cap", maxPixels, maxPixels, true},
		// the cap check must precede the zero-area early return
		{"huge width zero height", math.MaxInt, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkBitmapSize(tc.w, tc.h)
			if (err != nil) != tc.wantError {
				t.Errorf("checkBitmapSize(%d, %d) error = %v, wantError = %v",
					tc.w, tc.h, err, tc.wantError)
			}
		})
	}
}

func TestDecodeAllTestCases(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(testdataDir, "*.page"))
	if err != nil {
		t.Fatal(err)
	}

	for _, pagePath := range files {
		base := strings.TrimSuffix(filepath.Base(pagePath), ".page")
		t.Run(base, func(t *testing.T) {
			globals, page, expected := loadTestCase(t, base)
			got, err := Decode(globals, page, testBudget())
			if err != nil {
				t.Fatalf("decode error: %v", err)
			}
			if !bitmapsEqual(got, expected) {
				diffCount := 0
				if got.Width() == expected.Width() && got.Height() == expected.Height() {
					diffCount = countDiffs(got, expected)
				}
				t.Errorf("decoded bitmap differs (%dx%d, %d pixels differ)",
					got.Width(), got.Height(), diffCount)
			}
		})
	}
}

// TestDecodeScanSizedPage decodes full pages of 600 dpi scans, under the
// memory budget a PDF stream of the encoded size receives.  A blank page
// compresses to a few hundred bytes, so it is the hardest case for limits
// sized from the input length.
func TestDecodeScanSizedPage(t *testing.T) {
	type page struct {
		name string
		w, h int
		text bool
		mmr  bool
	}
	pages := []page{
		{"A4 blank", 4961, 7016, false, false},
		{"A4 text", 4961, 7016, true, false},
		{"letter blank MMR", 5100, 6600, false, true},
		{"letter text MMR", 5100, 6600, true, true},
	}
	for _, p := range pages {
		t.Run(p.name, func(t *testing.T) {
			bm := bitmap.New(p.w, p.h)
			if p.text {
				scanText(bm)
			}

			var region []byte
			if p.mmr {
				var err error
				region, err = EncodeGenericRegionSegmentMMR(bm, 0, 0, bitmap.CombOpOR)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				region = EncodeGenericRegionSegment(bm, 0, 0, 0, bitmap.CombOpOR, true, false)
			}
			info := WritePageInfo(nil, p.w, p.h)
			stream := WriteSegmentHeader(nil, 0, segPageInfo, 1, nil, uint32(len(info)))
			stream = append(stream, info...)
			stream = WriteSegmentHeader(stream, 1, segImmediateGeneric, 1, nil, uint32(len(region)))
			stream = append(stream, region...)

			// the JBIG2Decode filter charges the stream data before decoding
			budget := membudget.New(limits.StreamBudget(int64(len(stream))))
			if err := budget.Charge(len(stream)); err != nil {
				t.Fatal(err)
			}
			got, err := Decode(nil, stream, budget)
			if err != nil {
				t.Fatalf("%d bytes of stream: %v", len(stream), err)
			}
			if !bitmapsEqual(got, bm) {
				t.Error("round trip failed")
			}
		})
	}
}

// scanText fills bm with rows of glyph-like strokes, as on a 600 dpi scan
// of a text page with 1 inch margins.
func scanText(bm *bitmap.Bitmap) {
	const margin, lineH, xh, stem = 600, 100, 45, 8
	w, h := bm.Width(), bm.Height()
	fill := func(x0, y0, x1, y1 int) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				bm.SetPixel(x, y, true)
			}
		}
	}
	n := 0
	for base := margin + lineH; base < h-margin; base += lineH {
		for x := margin; x < w-margin-60; n++ {
			gw := 25 + (n*37)%30
			fill(x, base-xh, x+stem, base)
			switch n % 3 {
			case 0:
				fill(x+gw-stem, base-xh, x+gw, base)
				fill(x, base-xh, x+gw, base-xh+stem)
			case 1:
				fill(x, base-stem, x+gw, base)
				fill(x+gw-stem, base-xh, x+gw, base)
			default:
				fill(x, base-xh/2, x+gw, base-xh/2+stem)
			}
			x += gw + 8
			if n%6 == 5 {
				x += 30
			}
		}
	}
}

// TestCompositeWholePage composites a region covering the whole page with
// every combination operator onto pages of either default pixel value.  The
// region becomes the page where the result equals the region.
func TestCompositeWholePage(t *testing.T) {
	const w, h = 37, 21
	region := bitmap.New(w, h)
	for y := range h {
		for x := range w {
			region.SetPixel(x, y, (x*y+x)%3 == 0)
		}
	}
	ops := []bitmap.CombOp{bitmap.CombOpOR, bitmap.CombOpAND, bitmap.CombOpXOR,
		bitmap.CombOpXNOR, bitmap.CombOpReplace}
	for _, op := range ops {
		for _, def := range []bool{false, true} {
			want := bitmap.New(w, h)
			for y := range h {
				for x := range w {
					want.SetPixel(x, y, def)
				}
			}
			want.Combine(region, 0, 0, op)

			info := WritePageInfo(nil, w, h)
			if def {
				info[16] |= 0x04
			}
			seg := EncodeGenericRegionSegment(region, 0, 0, 0, op, false, false)
			stream := WriteSegmentHeader(nil, 0, segPageInfo, 1, nil, uint32(len(info)))
			stream = append(stream, info...)
			stream = WriteSegmentHeader(stream, 1, segImmediateGeneric, 1, nil, uint32(len(seg)))
			stream = append(stream, seg...)

			d := &decoder{
				segments: make(map[uint32]segmentResult),
				pool:     bitmapPool{budget: testBudget()},
			}
			if err := d.processStream(stream); err != nil {
				t.Fatal(err)
			}
			got, err := d.page()
			if err != nil {
				t.Fatal(err)
			}
			if !bitmapsEqual(got, want) {
				t.Errorf("op %d, default %t: wrong page", op, def)
			}

			// an adopted region leaves a single bitmap at the peak
			adopted := d.pool.peak == len(want.Pix)
			if adopted != keepsRegion(op, def) {
				t.Errorf("op %d, default %t: adopted %t, want %t",
					op, def, adopted, keepsRegion(op, def))
			}
		}
	}
}
