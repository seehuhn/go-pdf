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

	"seehuhn.de/go/membudget"
	"seehuhn.de/go/pdf/graphics/bitmap"
	internaljbig2 "seehuhn.de/go/pdf/internal/filter/jbig2"
)

// decodeImage runs the internal JBIG2 decoder against the globals and
// page bytes produced by an [Image].  It is a test helper that checks
// the encoded output can be read back.
func decodeImage(t *testing.T, g *Globals, im *Image) *bitmap.Bitmap {
	t.Helper()
	var globalsData []byte
	if g != nil {
		d, err := g.encode()
		if err != nil {
			t.Fatalf("globals encode: %v", err)
		}
		globalsData = d
	}
	pageData, err := im.encode()
	if err != nil {
		t.Fatalf("image encode: %v", err)
	}
	bm, err := internaljbig2.Decode(globalsData, pageData, membudget.New(1<<30))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return bm
}

func TestGenericRegionRoundTrip(t *testing.T) {
	bm := bitmap.New(32, 16)
	for y := range 16 {
		for x := range 32 {
			if (x+y)%2 == 0 {
				bm.SetPixel(x, y, true)
			}
		}
	}

	im := NewImage(32, 16, nil)
	if err := im.AddGenericRegion(bm, 0, 0, nil); err != nil {
		t.Fatalf("AddGenericRegion: %v", err)
	}

	got := decodeImage(t, nil, im)
	if got.Width() != bm.Width() || got.Height() != bm.Height() {
		t.Fatalf("size mismatch: got %dx%d, want %dx%d",
			got.Width(), got.Height(), bm.Width(), bm.Height())
	}
	for y := 0; y < bm.Height(); y++ {
		for x := 0; x < bm.Width(); x++ {
			if got.GetPixel(x, y) != bm.GetPixel(x, y) {
				t.Fatalf("pixel mismatch at (%d,%d)", x, y)
			}
		}
	}
}

func TestGenericRegionMMR(t *testing.T) {
	bm := bitmap.New(16, 16)
	for y := 0; y < 16; y += 2 {
		for x := range 16 {
			bm.SetPixel(x, y, true)
		}
	}

	im := NewImage(16, 16, nil)
	if err := im.AddGenericRegion(bm, 0, 0, &GenericOptions{UseMMR: true}); err != nil {
		t.Fatalf("AddGenericRegion: %v", err)
	}

	got := decodeImage(t, nil, im)
	for y := range 16 {
		for x := range 16 {
			if got.GetPixel(x, y) != bm.GetPixel(x, y) {
				t.Fatalf("pixel mismatch at (%d,%d)", x, y)
			}
		}
	}
}

func TestSymbolDictRoundTrip(t *testing.T) {
	sym0 := bitmap.New(5, 8)
	for y := range 8 {
		sym0.SetPixel(0, y, true)
	}
	for x := range 5 {
		sym0.SetPixel(x, 0, true)
		sym0.SetPixel(x, 4, true)
	}

	sym1 := bitmap.New(6, 8)
	for y := range 8 {
		sym1.SetPixel(0, y, true)
		sym1.SetPixel(5, y, true)
	}

	g := NewGlobals()
	id0, err := g.AddSymbol(sym0)
	if err != nil {
		t.Fatalf("AddSymbol: %v", err)
	}
	id1, err := g.AddSymbol(sym1)
	if err != nil {
		t.Fatalf("AddSymbol: %v", err)
	}

	im := NewImage(40, 10, g)
	if err := im.AddTextRegion(&TextRegion{
		Width:  40,
		Height: 10,
		Instances: []TextRegionInstance{
			{SymbolID: id0, X: 0, Y: 8},
			{SymbolID: id1, X: 8, Y: 8},
			{SymbolID: id0, X: 18, Y: 8},
		},
	}); err != nil {
		t.Fatalf("AddTextRegion: %v", err)
	}

	got := decodeImage(t, g, im)
	if got.Width() != 40 || got.Height() != 10 {
		t.Fatalf("size mismatch: %dx%d", got.Width(), got.Height())
	}

	// sym0 at (0, 8) bottom-left, so top-left is (0, 1); pixel (0, 1) set
	if !got.GetPixel(0, 1) {
		t.Error("expected pixel at (0,1) from sym0")
	}
}

// TestSymbolReorderRoundTrip adds symbols in non-sorted order (tall
// before short) so that Globals must reorder them.  The SymbolIDs
// returned by AddSymbol must remain valid.
func TestSymbolReorderRoundTrip(t *testing.T) {
	tall := bitmap.New(2, 12)
	for y := range 12 {
		tall.SetPixel(0, y, true)
		tall.SetPixel(1, y, true)
	}

	short := bitmap.New(8, 4)
	for x := range 8 {
		short.SetPixel(x, 0, true)
		short.SetPixel(x, 3, true)
	}

	g := NewGlobals()
	idTall, err := g.AddSymbol(tall)
	if err != nil {
		t.Fatalf("AddSymbol: %v", err)
	}
	idShort, err := g.AddSymbol(short)
	if err != nil {
		t.Fatalf("AddSymbol: %v", err)
	}

	im := NewImage(20, 14, g)
	if err := im.AddTextRegion(&TextRegion{
		Width:  20,
		Height: 14,
		Instances: []TextRegionInstance{
			{SymbolID: idTall, X: 0, Y: 11},
			{SymbolID: idShort, X: 10, Y: 3},
		},
	}); err != nil {
		t.Fatalf("AddTextRegion: %v", err)
	}

	got := decodeImage(t, g, im)

	want := bitmap.New(20, 14)
	want.Combine(tall, 0, 0, bitmap.CombOpOR)
	want.Combine(short, 10, 0, bitmap.CombOpOR)

	for y := range 14 {
		for x := range 20 {
			if got.GetPixel(x, y) != want.GetPixel(x, y) {
				t.Fatalf("pixel mismatch at (%d,%d): got %v, want %v",
					x, y, got.GetPixel(x, y), want.GetPixel(x, y))
			}
		}
	}
}

// TestLocalSymbolRoundTrip verifies that page-local symbols (no
// globals) can be encoded and decoded round-trip.
func TestLocalSymbolRoundTrip(t *testing.T) {
	sym0 := bitmap.New(5, 8)
	for y := range 8 {
		sym0.SetPixel(0, y, true)
	}
	for x := range 5 {
		sym0.SetPixel(x, 0, true)
		sym0.SetPixel(x, 4, true)
	}

	sym1 := bitmap.New(6, 8)
	for y := range 8 {
		sym1.SetPixel(0, y, true)
		sym1.SetPixel(5, y, true)
	}

	im := NewImage(40, 10, nil)
	id0, err := im.AddSymbol(sym0)
	if err != nil {
		t.Fatalf("AddSymbol: %v", err)
	}
	id1, err := im.AddSymbol(sym1)
	if err != nil {
		t.Fatalf("AddSymbol: %v", err)
	}

	if err := im.AddTextRegion(&TextRegion{
		Width:  40,
		Height: 10,
		Instances: []TextRegionInstance{
			{SymbolID: id0, X: 0, Y: 8, Local: true},
			{SymbolID: id1, X: 8, Y: 8, Local: true},
			{SymbolID: id0, X: 18, Y: 8, Local: true},
		},
	}); err != nil {
		t.Fatalf("AddTextRegion: %v", err)
	}

	got := decodeImage(t, nil, im)
	if got.Width() != 40 || got.Height() != 10 {
		t.Fatalf("size mismatch: %dx%d", got.Width(), got.Height())
	}
	if !got.GetPixel(0, 1) {
		t.Error("expected pixel at (0,1) from sym0")
	}
}

// TestLocalSymbolReorder adds page-local symbols in non-sorted order
// (tall before short) and verifies reordering works correctly.
func TestLocalSymbolReorder(t *testing.T) {
	tall := bitmap.New(2, 12)
	for y := range 12 {
		tall.SetPixel(0, y, true)
		tall.SetPixel(1, y, true)
	}

	short := bitmap.New(8, 4)
	for x := range 8 {
		short.SetPixel(x, 0, true)
		short.SetPixel(x, 3, true)
	}

	im := NewImage(20, 14, nil)
	idTall, err := im.AddSymbol(tall)
	if err != nil {
		t.Fatalf("AddSymbol: %v", err)
	}
	idShort, err := im.AddSymbol(short)
	if err != nil {
		t.Fatalf("AddSymbol: %v", err)
	}

	if err := im.AddTextRegion(&TextRegion{
		Width:  20,
		Height: 14,
		Instances: []TextRegionInstance{
			{SymbolID: idTall, X: 0, Y: 11, Local: true},
			{SymbolID: idShort, X: 10, Y: 3, Local: true},
		},
	}); err != nil {
		t.Fatalf("AddTextRegion: %v", err)
	}

	got := decodeImage(t, nil, im)

	want := bitmap.New(20, 14)
	want.Combine(tall, 0, 0, bitmap.CombOpOR)
	want.Combine(short, 10, 0, bitmap.CombOpOR)

	for y := range 14 {
		for x := range 20 {
			if got.GetPixel(x, y) != want.GetPixel(x, y) {
				t.Fatalf("pixel mismatch at (%d,%d): got %v, want %v",
					x, y, got.GetPixel(x, y), want.GetPixel(x, y))
			}
		}
	}
}

// TestMixedGlobalLocalSymbols verifies that a text region can reference
// both global and page-local symbols.
func TestMixedGlobalLocalSymbols(t *testing.T) {
	// global symbol: vertical bar
	globalSym := bitmap.New(2, 8)
	for y := range 8 {
		globalSym.SetPixel(0, y, true)
		globalSym.SetPixel(1, y, true)
	}

	g := NewGlobals()
	gID, err := g.AddSymbol(globalSym)
	if err != nil {
		t.Fatalf("AddSymbol: %v", err)
	}

	// local symbol: horizontal bar
	localSym := bitmap.New(8, 2)
	for x := range 8 {
		localSym.SetPixel(x, 0, true)
		localSym.SetPixel(x, 1, true)
	}

	im := NewImage(20, 10, g)
	lID, err := im.AddSymbol(localSym)
	if err != nil {
		t.Fatalf("AddSymbol: %v", err)
	}

	if err := im.AddTextRegion(&TextRegion{
		Width:  20,
		Height: 10,
		Instances: []TextRegionInstance{
			{SymbolID: gID, X: 0, Y: 7, Local: false},
			{SymbolID: lID, X: 10, Y: 1, Local: true},
		},
	}); err != nil {
		t.Fatalf("AddTextRegion: %v", err)
	}

	got := decodeImage(t, g, im)

	want := bitmap.New(20, 10)
	want.Combine(globalSym, 0, 0, bitmap.CombOpOR)
	want.Combine(localSym, 10, 0, bitmap.CombOpOR)

	for y := range 10 {
		for x := range 20 {
			if got.GetPixel(x, y) != want.GetPixel(x, y) {
				t.Fatalf("pixel mismatch at (%d,%d): got %v, want %v",
					x, y, got.GetPixel(x, y), want.GetPixel(x, y))
			}
		}
	}
}

// TestImageEncodeEncodesGlobals verifies that calling Image.encode
// directly (without a prior Globals.encode call) still populates the
// globals state needed by text-region ops.  Previously this path
// would panic on a nil symIDMap.
func TestImageEncodeEncodesGlobals(t *testing.T) {
	sym := bitmap.New(4, 6)
	for y := range 6 {
		sym.SetPixel(0, y, true)
	}

	g := NewGlobals()
	id, err := g.AddSymbol(sym)
	if err != nil {
		t.Fatalf("AddSymbol: %v", err)
	}

	im := NewImage(10, 10, g)
	if err := im.AddTextRegion(&TextRegion{
		Width:  10,
		Height: 10,
		Instances: []TextRegionInstance{
			{SymbolID: id, X: 0, Y: 5},
		},
	}); err != nil {
		t.Fatalf("AddTextRegion: %v", err)
	}

	if _, err := im.encode(); err != nil {
		t.Fatalf("encode: %v", err)
	}
}

// TestPatternDictMMRRoundTrip verifies that an MMR-coded pattern
// dictionary, referenced by a halftone region, round-trips through
// the decoder.  The minimal case uses two solid patterns and a 2×1
// grid.
func TestPatternDictMMRRoundTrip(t *testing.T) {
	// two 4x4 patterns: all-white and all-black
	patW, patH := 4, 4
	patterns := []*bitmap.Bitmap{
		bitmap.New(patW, patH),
		bitmap.New(patW, patH),
	}
	for y := range patH {
		for x := range patW {
			patterns[1].SetPixel(x, y, true)
		}
	}

	g := NewGlobals()
	patID, err := g.AddPatternDict(patterns, &PatternDictOptions{UseMMR: true})
	if err != nil {
		t.Fatalf("AddPatternDict: %v", err)
	}

	// 2x1 grid: first cell white (0), second cell black (1)
	gridW, gridH := 2, 1
	grayValues := []int{0, 1}
	regionW := gridW * patW
	regionH := gridH * patH

	im := NewImage(regionW, regionH, g)
	if err := im.AddHalftoneRegion(&HalftoneRegion{
		Width:         regionW,
		Height:        regionH,
		PatternDictID: patID,
		GrayValues:    grayValues,
		GridWidth:     gridW,
		GridHeight:    gridH,
		GridVX:        patW,
	}); err != nil {
		t.Fatalf("AddHalftoneRegion: %v", err)
	}

	got := decodeImage(t, g, im)
	for x := range patW {
		for y := range patH {
			if got.GetPixel(x, y) {
				t.Errorf("expected white pixel at (%d,%d) in first cell", x, y)
			}
			if !got.GetPixel(patW+x, y) {
				t.Errorf("expected black pixel at (%d,%d) in second cell", patW+x, y)
			}
		}
	}
}

// TestPatternDictMMRMultiple verifies that an MMR-coded pattern
// dictionary with more than two patterns round-trips correctly, and
// that a multi-cell grid selects the right pattern per cell.  This
// exercises the gray-index bitplane encoding (needing >1 bit per
// cell) and the MMR collective-bitmap layout with non-trivial
// patterns.
func TestPatternDictMMRMultiple(t *testing.T) {
	patW, patH := 4, 4
	patterns := make([]*bitmap.Bitmap, 4)
	for i := range patterns {
		patterns[i] = bitmap.New(patW, patH)
	}
	// pattern 0: all white (left as constructed)
	// pattern 1: single pixel at (0,0)
	patterns[1].SetPixel(0, 0, true)
	// pattern 2: anti-diagonal
	for i := range patW {
		patterns[2].SetPixel(i, patH-1-i, true)
	}
	// pattern 3: all black
	for y := range patH {
		for x := range patW {
			patterns[3].SetPixel(x, y, true)
		}
	}

	g := NewGlobals()
	patID, err := g.AddPatternDict(patterns, &PatternDictOptions{UseMMR: true})
	if err != nil {
		t.Fatalf("AddPatternDict: %v", err)
	}

	// 2x2 grid touching all four patterns
	gridW, gridH := 2, 2
	grayValues := []int{
		0, 1,
		2, 3,
	}
	regionW := gridW * patW
	regionH := gridH * patH

	im := NewImage(regionW, regionH, g)
	if err := im.AddHalftoneRegion(&HalftoneRegion{
		Width:         regionW,
		Height:        regionH,
		PatternDictID: patID,
		GrayValues:    grayValues,
		GridWidth:     gridW,
		GridHeight:    gridH,
		GridVX:        patW,
	}); err != nil {
		t.Fatalf("AddHalftoneRegion: %v", err)
	}

	// reference: tile the grid with the selected patterns.
	want := bitmap.New(regionW, regionH)
	for gy := range gridH {
		for gx := range gridW {
			pat := patterns[grayValues[gy*gridW+gx]]
			want.Combine(pat, gx*patW, gy*patH, bitmap.CombOpOR)
		}
	}

	got := decodeImage(t, g, im)
	if got.Width() != regionW || got.Height() != regionH {
		t.Fatalf("size mismatch: got %dx%d, want %dx%d",
			got.Width(), got.Height(), regionW, regionH)
	}
	for y := range regionH {
		for x := range regionW {
			if got.GetPixel(x, y) != want.GetPixel(x, y) {
				t.Errorf("pixel mismatch at (%d,%d): got %v, want %v",
					x, y, got.GetPixel(x, y), want.GetPixel(x, y))
			}
		}
	}
}

// TestPatternDictMMRMixed verifies that arithmetic- and MMR-coded
// pattern dictionaries can coexist in the same Globals, with separate
// halftone regions referencing each.  This exercises the per-dict
// options path in Globals.encode and ensures the two encoders do not
// interfere with each other's segment framing.
func TestPatternDictMMRMixed(t *testing.T) {
	patW, patH := 4, 4

	// two patterns per dict: one white, one black.
	mkPatterns := func() []*bitmap.Bitmap {
		pw := bitmap.New(patW, patH)
		pb := bitmap.New(patW, patH)
		for y := range patH {
			for x := range patW {
				pb.SetPixel(x, y, true)
			}
		}
		return []*bitmap.Bitmap{pw, pb}
	}

	g := NewGlobals()
	arithID, err := g.AddPatternDict(mkPatterns(), nil)
	if err != nil {
		t.Fatalf("AddPatternDict (arith): %v", err)
	}
	mmrID, err := g.AddPatternDict(mkPatterns(), &PatternDictOptions{UseMMR: true})
	if err != nil {
		t.Fatalf("AddPatternDict (mmr): %v", err)
	}

	gridW, gridH := 2, 1
	grayValues := []int{0, 1}
	regionW := gridW * patW
	regionH := gridH * patH

	mkImage := func(patID int) *Image {
		im := NewImage(regionW, regionH, g)
		if err := im.AddHalftoneRegion(&HalftoneRegion{
			Width:         regionW,
			Height:        regionH,
			PatternDictID: patID,
			GrayValues:    grayValues,
			GridWidth:     gridW,
			GridHeight:    gridH,
			GridVX:        patW,
		}); err != nil {
			t.Fatalf("AddHalftoneRegion (patID=%d): %v", patID, err)
		}
		return im
	}

	// mark both dicts used before the first encode.
	imArith := mkImage(arithID)
	imMMR := mkImage(mmrID)

	// both images share the same Globals; each decode must produce
	// white-then-black.
	for _, tc := range []struct {
		name string
		im   *Image
	}{
		{"arith", imArith},
		{"mmr", imMMR},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := decodeImage(t, g, tc.im)
			for y := range patH {
				for x := range patW {
					if got.GetPixel(x, y) {
						t.Errorf("expected white at (%d,%d)", x, y)
					}
					if !got.GetPixel(patW+x, y) {
						t.Errorf("expected black at (%d,%d)", patW+x, y)
					}
				}
			}
		})
	}
}

// TestGlobalsFreezeAfterEncode verifies that adding symbols or
// patterns after encoding returns an error.
func TestGlobalsFreezeAfterEncode(t *testing.T) {
	g := NewGlobals()
	if _, err := g.AddSymbol(bitmap.New(2, 2)); err != nil {
		t.Fatalf("AddSymbol: %v", err)
	}
	if _, err := g.encode(); err != nil {
		t.Fatalf("encode: %v", err)
	}
	if _, err := g.AddSymbol(bitmap.New(3, 3)); err == nil {
		t.Error("expected error adding symbol to frozen Globals")
	}
	if _, err := g.AddPatternDict([]*bitmap.Bitmap{bitmap.New(4, 4)}, nil); err == nil {
		t.Error("expected error adding pattern dict to frozen Globals")
	}
}

// TestImageWithoutGlobals verifies that a self-contained image (no
// globals) can be encoded and decoded round-trip.
func TestImageWithoutGlobals(t *testing.T) {
	bm := bitmap.New(8, 8)
	for i := range 8 {
		bm.SetPixel(i, i, true)
	}

	im := NewImage(8, 8, nil)
	if err := im.AddGenericRegion(bm, 0, 0, nil); err != nil {
		t.Fatalf("AddGenericRegion: %v", err)
	}

	got := decodeImage(t, nil, im)
	for y := range 8 {
		for x := range 8 {
			if got.GetPixel(x, y) != bm.GetPixel(x, y) {
				t.Fatalf("pixel mismatch at (%d,%d)", x, y)
			}
		}
	}
}

// TestImageFreezeAfterEncode verifies that adding symbols or regions
// after encoding returns an error for every mutating method on Image.
func TestImageFreezeAfterEncode(t *testing.T) {
	sym := bitmap.New(4, 4)
	sym.SetPixel(0, 0, true)

	tests := []struct {
		name   string
		mutate func(im *Image) error
	}{
		{"AddSymbol", func(im *Image) error {
			_, err := im.AddSymbol(sym)
			return err
		}},
		{"AddGenericRegion", func(im *Image) error {
			return im.AddGenericRegion(bitmap.New(4, 4), 0, 0, nil)
		}},
		{"AddRefinementRegion", func(im *Image) error {
			return im.AddRefinementRegion(bitmap.New(4, 4), bitmap.New(4, 4), 0, 0, nil)
		}},
		{"AddTextRegion", func(im *Image) error {
			return im.AddTextRegion(&TextRegion{Width: 4, Height: 4})
		}},
		{"AddHalftoneRegion", func(im *Image) error {
			return im.AddHalftoneRegion(&HalftoneRegion{Width: 4, Height: 4})
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			im := NewImage(8, 8, nil)
			if err := im.AddGenericRegion(bitmap.New(4, 4), 0, 0, nil); err != nil {
				t.Fatalf("AddGenericRegion: %v", err)
			}
			if _, err := im.encode(); err != nil {
				t.Fatalf("encode: %v", err)
			}
			if err := tc.mutate(im); err == nil {
				t.Errorf("expected error mutating frozen Image via %s", tc.name)
			}
		})
	}
}

// TestImageSymbolTemplate verifies that a non-default page-local
// SymbolTemplate round-trips correctly.
func TestImageSymbolTemplate(t *testing.T) {
	sym := bitmap.New(6, 8)
	for y := range 8 {
		sym.SetPixel(0, y, true)
		sym.SetPixel(5, y, true)
	}
	for x := range 6 {
		sym.SetPixel(x, 0, true)
		sym.SetPixel(x, 7, true)
	}

	for _, tmpl := range []int{0, 1, 2, 3} {
		t.Run(fmt.Sprintf("template%d", tmpl), func(t *testing.T) {
			im := NewImage(20, 10, nil)
			im.SymbolTemplate = tmpl
			id, err := im.AddSymbol(sym)
			if err != nil {
				t.Fatalf("AddSymbol: %v", err)
			}
			if err := im.AddTextRegion(&TextRegion{
				Width:  20,
				Height: 10,
				Instances: []TextRegionInstance{
					// bottom-left corner at (X, Y) with h=8 puts top row at Y-7
					{SymbolID: id, X: 0, Y: 7, Local: true},
					{SymbolID: id, X: 10, Y: 7, Local: true},
				},
			}); err != nil {
				t.Fatalf("AddTextRegion: %v", err)
			}

			got := decodeImage(t, nil, im)

			want := bitmap.New(20, 10)
			want.Combine(sym, 0, 0, bitmap.CombOpOR)
			want.Combine(sym, 10, 0, bitmap.CombOpOR)

			for y := range 10 {
				for x := range 20 {
					if got.GetPixel(x, y) != want.GetPixel(x, y) {
						t.Fatalf("pixel mismatch at (%d,%d): got %v, want %v",
							x, y, got.GetPixel(x, y), want.GetPixel(x, y))
					}
				}
			}
		})
	}
}

// TestUnusedDictionariesSkipped verifies that symbol and pattern
// dictionaries added but never referenced are dropped from the output.
func TestUnusedDictionariesSkipped(t *testing.T) {
	t.Run("Globals symbols unreferenced", func(t *testing.T) {
		g := NewGlobals()
		if _, err := g.AddSymbol(bitmap.New(4, 4)); err != nil {
			t.Fatalf("AddSymbol: %v", err)
		}
		// image uses globals but only for a generic region, not a text region
		im := NewImage(8, 8, g)
		if err := im.AddGenericRegion(bitmap.New(8, 8), 0, 0, nil); err != nil {
			t.Fatalf("AddGenericRegion: %v", err)
		}
		data, err := g.encode()
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		if len(data) != 0 {
			t.Errorf("expected empty globals; got %d bytes", len(data))
		}
	})

	t.Run("Image local symbols unreferenced", func(t *testing.T) {
		// Build two images that differ only in whether they hold an
		// unreferenced page-local symbol.  Encoded output must match.
		mk := func(withSym bool) []byte {
			im := NewImage(8, 8, nil)
			if withSym {
				if _, err := im.AddSymbol(bitmap.New(4, 4)); err != nil {
					t.Fatalf("AddSymbol: %v", err)
				}
			}
			if err := im.AddGenericRegion(bitmap.New(8, 8), 0, 0, nil); err != nil {
				t.Fatalf("AddGenericRegion: %v", err)
			}
			data, err := im.encode()
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			return data
		}
		if !bytes.Equal(mk(true), mk(false)) {
			t.Error("unreferenced local symbol affected encoded output")
		}
	})
}

// TestTemplateOutOfRange verifies that out-of-range template values
// produce errors.
func TestTemplateOutOfRange(t *testing.T) {
	t.Run("GenericOptions.Template", func(t *testing.T) {
		im := NewImage(8, 8, nil)
		if err := im.AddGenericRegion(bitmap.New(4, 4), 0, 0, &GenericOptions{Template: 4}); err == nil {
			t.Error("expected error for Template=4")
		}
	})
	t.Run("RefinementOptions.Template", func(t *testing.T) {
		im := NewImage(8, 8, nil)
		if err := im.AddRefinementRegion(bitmap.New(4, 4), bitmap.New(4, 4), 0, 0, &RefinementOptions{Template: 2}); err == nil {
			t.Error("expected error for Template=2")
		}
	})
	t.Run("HalftoneRegion.Template", func(t *testing.T) {
		im := NewImage(8, 8, nil)
		if err := im.AddHalftoneRegion(&HalftoneRegion{Width: 8, Height: 8, Template: 4}); err == nil {
			t.Error("expected error for Template=4")
		}
	})
	t.Run("Globals.SymbolTemplate", func(t *testing.T) {
		g := NewGlobals()
		g.SymbolTemplate = 4
		id, err := g.AddSymbol(bitmap.New(2, 2))
		if err != nil {
			t.Fatalf("AddSymbol: %v", err)
		}
		// drive emission via a text region that references the symbol
		im := NewImage(8, 8, g)
		if err := im.AddTextRegion(&TextRegion{
			Width:  8,
			Height: 8,
			Instances: []TextRegionInstance{
				{SymbolID: id, X: 0, Y: 2},
			},
		}); err != nil {
			t.Fatalf("AddTextRegion: %v", err)
		}
		if _, err := g.encode(); err == nil {
			t.Error("expected error for SymbolTemplate=4")
		}
	})
	t.Run("Image.SymbolTemplate", func(t *testing.T) {
		im := NewImage(8, 8, nil)
		im.SymbolTemplate = 4
		id, err := im.AddSymbol(bitmap.New(2, 2))
		if err != nil {
			t.Fatalf("AddSymbol: %v", err)
		}
		// drive emission via a text region that references the local symbol
		if err := im.AddTextRegion(&TextRegion{
			Width:  8,
			Height: 8,
			Instances: []TextRegionInstance{
				{SymbolID: id, X: 0, Y: 2, Local: true},
			},
		}); err != nil {
			t.Fatalf("AddTextRegion: %v", err)
		}
		if _, err := im.encode(); err == nil {
			t.Error("expected error for Image.SymbolTemplate=4")
		}
	})
}

// TestTextRegionSymbolIDErrors verifies that out-of-range symbol IDs
// and missing dictionaries produce errors at encode time.
func TestTextRegionSymbolIDErrors(t *testing.T) {
	sym := bitmap.New(4, 4)
	sym.SetPixel(0, 0, true)

	tests := []struct {
		name  string
		setup func(t *testing.T) *Image
	}{
		{"local ID out of range", func(t *testing.T) *Image {
			im := NewImage(20, 10, nil)
			if _, err := im.AddSymbol(sym); err != nil {
				t.Fatalf("AddSymbol: %v", err)
			}
			if err := im.AddTextRegion(&TextRegion{
				Width:  20,
				Height: 10,
				Instances: []TextRegionInstance{
					{SymbolID: 5, X: 0, Y: 8, Local: true},
				},
			}); err != nil {
				t.Fatalf("AddTextRegion: %v", err)
			}
			return im
		}},
		{"global ID out of range", func(t *testing.T) *Image {
			g := NewGlobals()
			if _, err := g.AddSymbol(sym); err != nil {
				t.Fatalf("AddSymbol: %v", err)
			}
			im := NewImage(20, 10, g)
			if err := im.AddTextRegion(&TextRegion{
				Width:  20,
				Height: 10,
				Instances: []TextRegionInstance{
					{SymbolID: 5, X: 0, Y: 8},
				},
			}); err != nil {
				t.Fatalf("AddTextRegion: %v", err)
			}
			return im
		}},
		{"Local without local symbols", func(t *testing.T) *Image {
			g := NewGlobals()
			if _, err := g.AddSymbol(sym); err != nil {
				t.Fatalf("AddSymbol: %v", err)
			}
			im := NewImage(20, 10, g)
			if err := im.AddTextRegion(&TextRegion{
				Width:  20,
				Height: 10,
				Instances: []TextRegionInstance{
					{SymbolID: 0, X: 0, Y: 8, Local: true},
				},
			}); err != nil {
				t.Fatalf("AddTextRegion: %v", err)
			}
			return im
		}},
		{"global without Globals", func(t *testing.T) *Image {
			im := NewImage(20, 10, nil)
			if _, err := im.AddSymbol(sym); err != nil {
				t.Fatalf("AddSymbol: %v", err)
			}
			if err := im.AddTextRegion(&TextRegion{
				Width:  20,
				Height: 10,
				Instances: []TextRegionInstance{
					{SymbolID: 0, X: 0, Y: 8},
				},
			}); err != nil {
				t.Fatalf("AddTextRegion: %v", err)
			}
			return im
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			im := tc.setup(t)
			if _, err := im.encode(); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
