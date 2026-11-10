// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package standard

import (
	"math"
	"slices"
	"strings"
	"testing"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/internal/bundled"
	"seehuhn.de/go/pdf/font/pdfenc"
)

// TestGlyphLists tests that the glyph lists of the 14 standard PDF fonts are
// the character sets guaranteed by the specification.
func TestGlyphLists(t *testing.T) {
	for _, G := range All {
		metrics, err := G.data()
		if err != nil {
			t.Fatal(err)
		}

		var charset map[string]bool
		switch G {
		case Symbol:
			charset = pdfenc.Symbol.Has
		case ZapfDingbats:
			charset = pdfenc.ZapfDingbats.Has
		default:
			charset = pdfenc.StandardLatin.Has
		}
		for name := range metrics.Glyphs {
			if !charset[name] && name != ".notdef" {
				t.Errorf("%-22s unexpected glyph %q", G, name)
			}
		}
		for name := range charset {
			if _, ok := metrics.Glyphs[name]; !ok {
				t.Errorf("%-22s missing glyph %q", G, name)
			}
		}
	}
}

// TestGlyphWidths tests that the widths of the font instances are those in
// Adobe's AFM files.
func TestGlyphWidths(t *testing.T) {
	for _, G := range All {
		full, err := bundled.ReadAFM(string(G))
		if err != nil {
			t.Fatal(err)
		}
		F := font.Must(G.New())

		geom := F.GetGeometry()
		for gid, name := range F.GlyphNames {
			if name == ".notdef" {
				continue
			}
			want := full.Glyphs[name].WidthX / 1000
			if got := geom.Widths[gid]; math.Abs(got-want) > 1e-9 {
				t.Errorf("%-22s %-8s width=%g, want %g", G, name, got, want)
			}
		}
	}
}

// TestBlankGlyphs checks that the glyphs of the font instances which have an
// empty bounding box are exactly the white space glyphs.
func TestBlankGlyphs(t *testing.T) {
	for _, G := range All {
		F := font.Must(G.New())
		geom := F.GetGeometry()
		for gid, name := range F.GlyphNames {
			blank := geom.GlyphExtents[gid].IsZero()
			if name == ".notdef" {
				continue
			}
			if blank != (name == "space") {
				t.Errorf("%-22s %-8s blank=%v", G, name, blank)
			}
		}
	}
}

// TestEuroBBox checks the Euro bounding boxes against the Euro glyphs of the
// bundled Quire fonts, which draw the glyph.
func TestEuroBBox(t *testing.T) {
	quire := map[string]string{
		"Courier":   "QuireMono",
		"Helvetica": "QuireSans",
		"Times":     "QuireSerif",
	}
	for _, G := range All {
		if G == Symbol || G == ZapfDingbats {
			continue
		}
		bold, italic := G.Style()
		style := "Regular"
		switch {
		case bold && italic:
			style = "BoldItalic"
		case bold:
			style = "Bold"
		case italic:
			style = "Italic"
		}
		family, _, _ := strings.Cut(string(G), "-")
		qName := quire[family] + "-" + style

		q, err := bundled.ReadSfnt(qName)
		if err != nil {
			t.Fatal(err)
		}
		cm, err := q.CMapTable.GetBest()
		if err != nil {
			t.Fatal(err)
		}
		gid := cm.Lookup('€')
		if gid == 0 {
			t.Fatalf("%s: no Euro glyph", qName)
		}
		qBox := q.GlyphBBoxesPDF()[gid]
		want := rect.Rect{
			LLx: math.Floor(qBox.LLx),
			LLy: math.Floor(qBox.LLy),
			URx: math.Ceil(qBox.URx),
			URy: math.Ceil(qBox.URy),
		}

		F := font.Must(G.New())
		idx := slices.Index(F.GlyphNames, "Euro")
		if idx < 0 {
			t.Fatalf("%s: no Euro glyph", G)
		}
		ext := F.GetGeometry().GlyphExtents[idx]
		got := rect.Rect{LLx: ext.LLx * 1000, LLy: ext.LLy * 1000, URx: ext.URx * 1000, URy: ext.URy * 1000}
		if !rectNear(got, want) {
			t.Errorf("%s: Euro bbox %v, want %v", G, got, want)
		}

		// the Quire glyph must fit the advance width of the metrics
		if w := F.GetGeometry().Widths[idx] * 1000; math.Abs(q.GlyphWidthPDF(gid)-w) > 0.5 {
			t.Errorf("%s: Quire Euro width %g, want %g", G, q.GlyphWidthPDF(gid), w)
		}
	}
}

// TestNotdef checks the .notdef metrics against the .notdef glyphs of the
// bundled Quire fonts.
func TestNotdef(t *testing.T) {
	for _, G := range All {
		q, err := bundled.ReadSfnt(string(G))
		if err != nil {
			t.Fatal(err)
		}
		qBox := q.GlyphBBoxesPDF()[0]
		want := rect.Rect{
			LLx: math.Floor(qBox.LLx),
			LLy: math.Floor(qBox.LLy),
			URx: math.Ceil(qBox.URx),
			URy: math.Ceil(qBox.URy),
		}

		F := font.Must(G.New())
		if F.GlyphNames[0] != ".notdef" {
			t.Fatalf("%s: glyph 0 is %q", G, F.GlyphNames[0])
		}
		geom := F.GetGeometry()
		ext := geom.GlyphExtents[0]
		got := rect.Rect{LLx: ext.LLx * 1000, LLy: ext.LLy * 1000, URx: ext.URx * 1000, URy: ext.URy * 1000}
		if !rectNear(got, want) {
			t.Errorf("%s: .notdef bbox %v, want %v", G, got, want)
		}
		if w := geom.Widths[0] * 1000; w != math.Round(q.GlyphWidthPDF(0)) {
			t.Errorf("%s: .notdef width %g, want %g", G, w, math.Round(q.GlyphWidthPDF(0)))
		}
	}
}

func rectNear(a, b rect.Rect) bool {
	const eps = 1e-6
	return math.Abs(a.LLx-b.LLx) < eps && math.Abs(a.LLy-b.LLy) < eps &&
		math.Abs(a.URx-b.URx) < eps && math.Abs(a.URy-b.URy) < eps
}

// TestLigatures checks that letters are correctly combined into ligatures.
func TestLigatures(t *testing.T) {
	ligatures := []string{"ﬀ=ff", "ﬁ=fi", "ﬂ=fl", "ﬃ=ffi", "ﬄ=ffl"}
	for i, G := range All {
		F := font.Must(G.New())

		geom := F.GetGeometry()

		for _, lig := range ligatures {
			gg := F.Layout(nil, 10, lig)

			rr := []rune(lig)
			if gg.Seq[0].GID == 0 {
				// The ligature is not present in the font.
				continue
			}

			var ligIsUsed bool
			if len(gg.Seq) == 3 {
				// Glyphs have been combined.
				ligIsUsed = true
				if gg.Seq[0].GID != gg.Seq[2].GID {
					t.Errorf("font %d: ligature %q: unexpected GIDs: %d %d %d",
						i, lig, gg.Seq[0].GID, gg.Seq[1].GID, gg.Seq[2].GID)
				}
				if gg.Seq[0].Text != string(rr[0]) {
					t.Errorf("font %d: ligature %q: unexpected glyph for %q[0]: %q",
						i, lig, lig, gg.Seq[0].Text)
				}

				if gg.Seq[1].Text != "=" {
					// test is broken
					t.Fatalf("font %d: ligature %q: unexpected glyph for %q[1]: %q",
						i, lig, lig, gg.Seq[1].Text)
				}

				if gg.Seq[2].Text != string(rr[2:]) {
					t.Errorf("font %d: ligature %q: unexpected glyph for %q[2]: %q",
						i, lig, lig, gg.Seq[2].Text)
				}
			} else {
				// Glyphs have not been combined.
				ligIsUsed = false
			}

			if ligIsUsed != !geom.IsFixedPitch() {
				t.Errorf("font %d: ligature %q: isFixedPitch=%t but ligIsUsed=%t",
					i, lig, geom.IsFixedPitch(), ligIsUsed)
			}
		}
	}
}

// TestNewIndependentEncoders checks that two instances of the same standard
// font allocate character codes independently.  They share the font data, so
// this is what keeps each usable in a document of its own.
func TestNewIndependentEncoders(t *testing.T) {
	first := font.Must(Helvetica.New())
	second := font.Must(Helvetica.New())

	if first.Simple == second.Simple {
		t.Fatal("the two instances share their encoding state")
	}

	// lay out text with the first instance only
	gid := first.Layout(nil, 10, "A").Seq[0].GID
	if _, ok := first.Encode(gid, "A"); !ok {
		t.Fatal("no code was allocated")
	}
	if first.Simple.CodesRemaining() == second.Simple.CodesRemaining() {
		t.Error("a code allocated in one instance was taken from the other")
	}
}

// TestNewSharesFontData checks that the instances of a standard font draw on
// one copy of the font data, which is what makes [Font.New] cheap after the
// first call.
func TestNewSharesFontData(t *testing.T) {
	first := font.Must(Helvetica.New())
	second := font.Must(Helvetica.New())

	if first.Geometry != second.Geometry {
		t.Error("the geometry is not shared")
	}
}

// The standard fonts are restricted to the character set the spec guarantees,
// so the metrics must not keep kern pairs for the glyphs which were dropped:
// a pair naming a glyph the font no longer has describes nothing.
func TestKernMatchesGlyphs(t *testing.T) {
	for _, f := range All {
		metrics, err := f.data()
		if err != nil {
			t.Fatal(err)
		}
		for _, k := range metrics.Kern {
			if _, ok := metrics.Glyphs[k.Left]; !ok {
				t.Errorf("%s: kern pair for missing glyph %q", f, k.Left)
				break
			}
			if _, ok := metrics.Glyphs[k.Right]; !ok {
				t.Errorf("%s: kern pair for missing glyph %q", f, k.Right)
				break
			}
		}
	}
}

// TestNewUnknownFont checks that a name which is not one of the 14 standard
// fonts, and so has nothing to share, reports the missing data as an error.
func TestNewUnknownFont(t *testing.T) {
	if _, err := Font("NoSuchFont").New(); err == nil {
		t.Error("expected an error for an unknown font")
	}
}
