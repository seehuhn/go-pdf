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
	bm, err := internaljbig2.Decode(globalsData, pageData)
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
	im.AddGenericRegion(bm, 0, 0, nil)

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
	im.AddGenericRegion(bm, 0, 0, &GenericOptions{UseMMR: true})

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
		t.Fatal(err)
	}
	id1, err := g.AddSymbol(sym1)
	if err != nil {
		t.Fatal(err)
	}

	im := NewImage(40, 10, g)
	im.AddTextRegion(&TextRegion{
		Width:  40,
		Height: 10,
		Instances: []TextRegionInstance{
			{SymbolID: id0, X: 0, Y: 8},
			{SymbolID: id1, X: 8, Y: 8},
			{SymbolID: id0, X: 18, Y: 8},
		},
	})

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
	idTall, _ := g.AddSymbol(tall)
	idShort, _ := g.AddSymbol(short)

	im := NewImage(20, 14, g)
	im.AddTextRegion(&TextRegion{
		Width:  20,
		Height: 14,
		Instances: []TextRegionInstance{
			{SymbolID: idTall, X: 0, Y: 11},
			{SymbolID: idShort, X: 10, Y: 3},
		},
	})

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

// TestGlobalsFreezeAfterEncode verifies that adding symbols after
// encoding returns an error.
func TestGlobalsFreezeAfterEncode(t *testing.T) {
	g := NewGlobals()
	g.AddSymbol(bitmap.New(2, 2))
	if _, err := g.encode(); err != nil {
		t.Fatalf("encode: %v", err)
	}
	if _, err := g.AddSymbol(bitmap.New(3, 3)); err == nil {
		t.Error("expected error adding symbol to frozen Globals")
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
	im.AddGenericRegion(bm, 0, 0, nil)

	got := decodeImage(t, nil, im)
	for y := range 8 {
		for x := range 8 {
			if got.GetPixel(x, y) != bm.GetPixel(x, y) {
				t.Fatalf("pixel mismatch at (%d,%d)", x, y)
			}
		}
	}
}
