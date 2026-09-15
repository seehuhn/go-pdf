// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package simpleenc

import (
	"sync"
	"testing"
	"time"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/internal/debug/race"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/sfnt/glyph"
)

func TestDefaultWidth(t *testing.T) {
	const defaultWidth = 999
	type entry struct {
		code  byte
		width float64
	}
	type testCase struct {
		name     string
		entries  []entry
		expected float64
	}
	cases := []testCase{
		{"empty", nil, defaultWidth},
		{"single left", []entry{{0, 600}}, defaultWidth},
		{"single middle", []entry{{255, 600}}, defaultWidth},
		{"single right", []entry{{255, 600}}, defaultWidth},
		{"three left", []entry{{0, 3}, {1, 3}, {2, 3}, {255, 1}}, 3},
		{"three right", []entry{{0, 1}, {253, 3}, {254, 3}, {255, 3}}, 3},
		{"single left, right", []entry{{0, 600}, {255, 600}}, 600},
		{"no savings", []entry{{0, 1}, {255, 2}}, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gd := NewSimple(defaultWidth, "", &pdfenc.WinAnsi)
			for _, e := range tc.entries {
				gd.info[e.code] = &codeInfo{Width: e.width}
			}

			if w := gd.DefaultWidth(); w != tc.expected {
				t.Errorf("got %g; expected %g", w, tc.expected)
			}
		})
	}
}

// The font name decides which glyph list a glyph name is looked up in:
// ZapfDingbats names the pointing hand "a12", and that name means nothing in
// any other font.  A font program taken out of a PDF file names the subset it
// is, so the tag must not reach the decision.
func TestNewSimpleIgnoresSubsetTag(t *testing.T) {
	const pointingHand = "\u261e"

	for _, name := range []string{"ZapfDingbats", "AAAAAA+ZapfDingbats"} {
		enc := NewSimple(0, name, &pdfenc.Standard)

		// The text is what the glyph name already implies, so no ToUnicode
		// entry is needed: that is only true if the dingbats glyph list was
		// used for both the code choice and the mapping.
		if _, err := enc.Encode(1, "a12", pointingHand, 100); err != nil {
			t.Fatal(err)
		}
		if tu := enc.ToUnicode(); tu != nil {
			t.Errorf("%s: %q needed a ToUnicode entry, so the glyph name did not imply it",
				name, pointingHand)
		}
	}
}

// TestConcurrentReaders checks that readers can use the table while codes
// are still being allocated: an edit session types into a font while a
// renderer draws the text already set in it.
func TestConcurrentReaders(t *testing.T) {
	if !race.Enabled {
		// Only the race detector can see the reads and writes overlap, so
		// without it the test would burn CPU without being able to fail.
		t.Skip("needs the race detector; run go test -race")
	}

	tab := NewSimple(500, "Test", &pdfenc.Standard)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for gid := range glyph.ID(200) {
			text := string(rune('A' + gid%26))
			_, _ = tab.Encode(gid+1, "", text, float64(gid))

			// Pace the writer.  Without this it allocates every code well
			// inside a single reader pass, and the readers spend nearly all
			// their time on a table which is already final.
			time.Sleep(50 * time.Microsecond)
		}
	}()

	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			s := make(pdf.String, 256)
			for i := range s {
				s[i] = byte(i)
			}
			for {
				select {
				case <-done:
					return
				default:
				}
				for range tab.Codes(s) {
				}
				for range tab.MappedCodes() {
				}
				_ = tab.GID(7)
				_ = tab.Width(7)
				_ = tab.GlyphName(3)
				_ = tab.Glyphs()
				_ = tab.Encoding()
				_ = tab.IsSymbolic()
				_ = tab.CodesRemaining()
				_ = tab.DefaultWidth()
				_ = tab.Error()
				_ = tab.ToUnicode()
				_, _ = tab.GetCode(3, "C")
			}
		})
	}
	wg.Wait()
}

// TestEncodeRepeat checks that encoding a glyph a second time yields the code
// it already has, so that a lookup and an allocation need not be one atomic
// step for the caller.
func TestEncodeRepeat(t *testing.T) {
	tab := NewSimple(500, "Test", &pdfenc.Standard)
	c1, err := tab.Encode(5, "A", "A", 600)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := tab.Encode(5, "A", "A", 600)
	if err != nil {
		t.Fatal(err)
	}
	if c1 != c2 {
		t.Errorf("second Encode gave code %d, want %d", c2, c1)
	}
	if tab.CodesRemaining() != 255 {
		t.Errorf("CodesRemaining = %d, want 255", tab.CodesRemaining())
	}
}

// TestGlyphID checks the CID scheme [Simple.GlyphID] implements: CID 0 is the
// notdef glyph, CID c+1 is the glyph of code c, and a CID outside that range
// or without an allocated code has no glyph.
func TestGlyphID(t *testing.T) {
	enc := NewSimple(0, "Test", &pdfenc.WinAnsi)

	// CID 0 is the notdef glyph, whether or not any code is allocated
	if gid, ok := enc.GlyphID(0); !ok || gid != 0 {
		t.Errorf("CID 0 selected glyph %d, ok %v; want glyph 0, ok true", gid, ok)
	}
	// before anything is allocated, no other CID has a glyph
	for _, c := range []cid.CID{1, 2, 256, 257, 1 << 20} {
		if _, ok := enc.GlyphID(c); ok {
			t.Errorf("CID %d has a glyph before any code is allocated", c)
		}
	}

	code, err := enc.Encode(42, "A", "A", 100)
	if err != nil {
		t.Fatal(err)
	}

	// the allocated code is reachable as CID code+1, and nothing else moved
	if gid, ok := enc.GlyphID(cid.CID(code) + 1); !ok || gid != 42 {
		t.Errorf("CID %d selected glyph %d, ok %v; want glyph 42, ok true",
			cid.CID(code)+1, gid, ok)
	}
	// Fill the table, so that code 0 has a glyph too.  CID 257 would reach it
	// if the upper bound were off by one, since byte(257-1) is 0.
	for i := range glyph.ID(255) {
		if _, err := enc.Encode(i+100, "", string(rune('a'+i%26))+string(rune('0'+i/26)), 100); err != nil {
			t.Fatal(err)
		}
	}
	if enc.GID(0) == 0 || enc.GID(255) == 0 {
		t.Fatal("the code table did not fill up")
	}
	// the last code is CID 256, the largest CID a simple font has
	if gid, ok := enc.GlyphID(256); !ok || gid != enc.GID(255) {
		t.Errorf("CID 256 selected glyph %d, ok %v; want glyph %d, ok true",
			gid, ok, enc.GID(255))
	}
	for _, c := range []cid.CID{257, 258, 1 << 20} {
		if gid, ok := enc.GlyphID(c); ok {
			t.Errorf("CID %d is outside the code range but selects glyph %d", c, gid)
		}
	}
}
