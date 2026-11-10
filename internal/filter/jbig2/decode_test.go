// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package jbig2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"seehuhn.de/go/pdf/graphics/bitmap"
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
		// Dimensions that defeat the multiplication-based checks: the
		// pixel-count multiplication wraps mod 2^64 to a small value,
		// and for 2^33 x 2^33 the byte-cost multiplication wraps too;
		// the zero-area case bypasses both via the early return.
		{"overflow 2^33 x 2^33", 1 << 33, 1 << 33, true},
		{"overflow huge width zero height", 1 << 31, 0, true},
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
