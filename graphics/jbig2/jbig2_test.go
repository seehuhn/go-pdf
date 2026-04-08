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

	"seehuhn.de/go/pdf/graphics/bitmap"
)

func TestGenericRegionRoundTrip(t *testing.T) {
	// create a test bitmap
	bm := bitmap.New(32, 16)
	for y := range 16 {
		for x := range 32 {
			if (x+y)%2 == 0 {
				bm.SetPixel(x, y, true)
			}
		}
	}

	enc := NewEncoder()
	page := NewPage(32, 16)
	page.AddGenericRegion(bm, 0, 0, nil)

	pageData, err := enc.EncodePage(page)
	if err != nil {
		t.Fatal(err)
	}

	got, err := Decode(nil, pageData)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if got.Width() != bm.Width() || got.Height() != bm.Height() {
		t.Fatalf("size mismatch: got %dx%d, want %dx%d", got.Width(), got.Height(), bm.Width(), bm.Height())
	}
	for y := 0; y < bm.Height(); y++ {
		for x := 0; x < bm.Width(); x++ {
			if got.GetPixel(x, y) != bm.GetPixel(x, y) {
				t.Fatalf("pixel mismatch at (%d,%d)", x, y)
			}
		}
	}
}

func TestSymbolDictRoundTrip(t *testing.T) {
	// create simple test symbols
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

	enc := NewEncoder()
	id0 := enc.AddSymbol(sym0)
	id1 := enc.AddSymbol(sym1)

	globals, err := enc.Globals()
	if err != nil {
		t.Fatal(err)
	}

	// create a page with text region placing symbols
	page := NewPage(40, 10)
	page.AddTextRegion(&TextRegion{
		Width:  40,
		Height: 10,
		Instances: []Instance{
			{SymbolID: id0, X: 0, Y: 8},
			{SymbolID: id1, X: 8, Y: 8},
			{SymbolID: id0, X: 18, Y: 8},
		},
	})

	pageData, err := enc.EncodePage(page)
	if err != nil {
		t.Fatal(err)
	}

	got, err := Decode(globals, pageData)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if got.Width() != 40 || got.Height() != 10 {
		t.Fatalf("size mismatch: %dx%d", got.Width(), got.Height())
	}

	// verify some pixels from the placed symbols
	// sym0 at (0, 8-7=1) with BOTTOMLEFT reference
	// The bottom-left of the 5×8 symbol is at (0, 8), so top-left is at (0, 1)
	if !got.GetPixel(0, 1) {
		t.Error("expected pixel at (0,1) from sym0")
	}
}

// TestSymbolReorderRoundTrip adds symbols in non-sorted order (tall before
// short) so that Globals() must reorder them. This verifies that SymbolIDs
// returned by AddSymbol remain valid after reordering.
func TestSymbolReorderRoundTrip(t *testing.T) {
	// sym0: tall narrow vertical bar (2×12)
	tall := bitmap.New(2, 12)
	for y := range 12 {
		tall.SetPixel(0, y, true)
		tall.SetPixel(1, y, true)
	}

	// sym1: short wide horizontal bar (8×4)
	short := bitmap.New(8, 4)
	for x := range 8 {
		short.SetPixel(x, 0, true)
		short.SetPixel(x, 3, true)
	}

	enc := NewEncoder()
	idTall := enc.AddSymbol(tall)   // id 0, height 12
	idShort := enc.AddSymbol(short) // id 1, height 4

	globals, err := enc.Globals()
	if err != nil {
		t.Fatal(err)
	}

	// place tall at left, short at right
	page := NewPage(20, 14)
	page.AddTextRegion(&TextRegion{
		Width:  20,
		Height: 14,
		Instances: []Instance{
			{SymbolID: idTall, X: 0, Y: 11},
			{SymbolID: idShort, X: 10, Y: 3},
		},
	})

	pageData, err := enc.EncodePage(page)
	if err != nil {
		t.Fatal(err)
	}

	got, err := Decode(globals, pageData)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	// build expected bitmap by manual placement
	want := bitmap.New(20, 14)
	want.Combine(tall, 0, 0, bitmap.CombOpOR)   // tall at (0, 11-11=0)
	want.Combine(short, 10, 0, bitmap.CombOpOR) // short at (10, 3-3=0)

	for y := range 14 {
		for x := range 20 {
			if got.GetPixel(x, y) != want.GetPixel(x, y) {
				t.Fatalf("pixel mismatch at (%d,%d): got %v, want %v",
					x, y, got.GetPixel(x, y), want.GetPixel(x, y))
			}
		}
	}
}
