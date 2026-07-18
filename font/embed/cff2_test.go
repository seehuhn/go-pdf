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

package embed_test

import (
	"math"
	"testing"

	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/embed"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/varfont"
	"seehuhn.de/go/pdf/internal/testfonts"
)

// embedSimpleCFF embeds F, lays out and encodes text, and reads back the
// resulting simple CFF font dictionary.
func embedSimpleCFF(t *testing.T, F font.Layouter, text string) *dict.Type1 {
	t.Helper()
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	ref, err := rm.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	gg := F.Layout(nil, 12, text)
	for _, g := range gg.Seq {
		if _, ok := F.Encode(g.GID, g.Text); !ok {
			t.Fatal("failed to encode glyph")
		}
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	dictObj, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := dictObj.(*dict.Type1)
	if !ok {
		t.Fatalf("wrong font dictionary type: %T", dictObj)
	}
	return d
}

// embedCompositeCFF embeds F, lays out and encodes text, and reads back the
// resulting CIDFontType0 font dictionary.
func embedCompositeCFF(t *testing.T, F font.Layouter, text string) *dict.CIDFontType0 {
	t.Helper()
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	ref, err := rm.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	gg := F.Layout(nil, 12, text)
	for _, g := range gg.Seq {
		if _, ok := F.Encode(g.GID, g.Text); !ok {
			t.Fatal("failed to encode glyph")
		}
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	dictObj, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := dictObj.(*dict.CIDFontType0)
	if !ok {
		t.Fatalf("wrong font dictionary type: %T", dictObj)
	}
	return d
}

// the embed dispatcher instances a variable CFF2 font before choosing the
// simple CFF embedder, landing in FontFile3/Type1C.
func TestEmbedCFF2SimpleVariable(t *testing.T) {
	coords := map[string]float64{"wght": 900}
	F, err := embed.OpenTypeFont(varfont.CFF2(), &embed.Options{Variations: coords})
	if err != nil {
		t.Fatal(err)
	}
	d := embedSimpleCFF(t, F, "A")

	if d.FontFile == nil || d.FontFile.Type != glyphdata.CFFSimple {
		t.Fatalf("FontFile type: got %v, want CFFSimple", d.FontFile)
	}

	inst, err := varfont.CFF2().Instantiate(coords)
	if err != nil {
		t.Fatal(err)
	}
	wantW := inst.GlyphWidthPDF(varfont.CFF2GIDBox)
	gotW, ok := d.GlyphWidth("A")
	if !ok {
		t.Fatal(`no width for "A"`)
	}
	if math.Abs(gotW*1000-wantW) > 0.5 {
		t.Errorf("width: got %v, want %v", gotW*1000, wantW)
	}

	// the embedded font file carries no CFF2 table and is not variable.
	back, err := cffglyphs.FromStream(d.FontFile)
	if err != nil {
		t.Fatal(err)
	}
	if back.Outlines.IsCIDKeyed() {
		t.Error("simple-path FontFile3 is CID-keyed; want a plain simple CFF")
	}
}

// the embed dispatcher instances a variable CFF2 font before choosing the
// composite CFF embedder, landing in FontFile3/CIDFontType0C.
func TestEmbedCFF2CompositeVariable(t *testing.T) {
	coords := map[string]float64{"wght": 900}
	F, err := embed.OpenTypeFont(varfont.CFF2(), &embed.Options{Variations: coords, Composite: true})
	if err != nil {
		t.Fatal(err)
	}
	// touch glyphs from both FDs of the fixture, so the CFF stays CID-keyed
	// rather than collapsing to a simple font.
	d := embedCompositeCFF(t, F, "AB")

	if d.FontFile == nil || d.FontFile.Type != glyphdata.CFF {
		t.Fatalf("FontFile type: got %v, want CFF (CIDFontType0C)", d.FontFile)
	}

	back, err := cffglyphs.FromStream(d.FontFile)
	if err != nil {
		t.Fatal(err)
	}
	if !back.Outlines.IsCIDKeyed() {
		t.Error("composite-path FontFile3 collapsed to a simple font; want CID-keyed")
	}
}

// a static (non-variable) CFF2 font reaching the embed dispatcher with nil
// Variations is converted to static CFF via ConvertCFF2, even though it has
// no axes to pin.
func TestEmbedCFF2SimpleStatic(t *testing.T) {
	F, err := embed.OpenTypeFont(varfont.StaticCFF2(), nil)
	if err != nil {
		t.Fatal(err)
	}
	d := embedSimpleCFF(t, F, "A")

	if d.FontFile == nil || d.FontFile.Type != glyphdata.CFFSimple {
		t.Fatalf("FontFile type: got %v, want CFFSimple", d.FontFile)
	}

	src := varfont.StaticCFF2()
	wantW := math.Round(src.GlyphWidthPDF(varfont.CFF2GIDBox))
	gotW, ok := d.GlyphWidth("A")
	if !ok {
		t.Fatal(`no width for "A"`)
	}
	if math.Abs(gotW*1000-wantW) > 0.5 {
		t.Errorf("width: got %v, want %v", gotW*1000, wantW)
	}

	back, err := cffglyphs.FromStream(d.FontFile)
	if err != nil {
		t.Fatal(err)
	}
	if back.Outlines.IsCIDKeyed() {
		t.Error("simple-path FontFile3 is CID-keyed; want a plain simple CFF")
	}
}

// a real variable CFF2 font (Adobe's VF prototype) embeds end to end at a
// non-default instance: subset tag and TN #5902 instance name in the
// PostScript name, a sane W array, and a non-empty re-parsed FontFile3.
func TestEmbedCFF2AdobeVFPrototype(t *testing.T) {
	path := testfonts.Path(t, "AdobeVFPrototype.otf")

	coords := map[string]float64{"wght": 900}
	F, err := embed.OpenTypeFile(path, &embed.Options{Variations: coords, Composite: true})
	if err != nil {
		t.Fatal(err)
	}
	d := embedCompositeCFF(t, F, "A")

	if len(d.SubsetTag) != 6 {
		t.Errorf("SubsetTag: got %q, want 6 letters", d.SubsetTag)
	}

	info, err := sfnt.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	inst, err := info.Instantiate(coords)
	if err != nil {
		t.Fatal(err)
	}
	wantPS := inst.PostScriptName()
	if d.PostScriptName != wantPS {
		t.Errorf("PostScriptName: got %q, want %q", d.PostScriptName, wantPS)
	}
	if want := d.SubsetTag + "+" + wantPS; d.Descriptor.FontName != want {
		t.Errorf("FontName: got %q, want %q", d.Descriptor.FontName, want)
	}

	if len(d.Width) == 0 {
		t.Error("empty W array")
	}
	for cidVal, width := range d.Width {
		if width < 0 || width > 10000 {
			t.Errorf("width for CID %d = %v, outside a sane range", cidVal, width)
		}
	}

	if d.FontFile == nil || d.FontFile.Type != glyphdata.CFF {
		t.Fatalf("FontFile type: got %v, want CFF (CIDFontType0C)", d.FontFile)
	}
	back, err := cffglyphs.FromStream(d.FontFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Outlines.Glyphs) == 0 {
		t.Fatal("re-parsed font has no glyphs")
	}
}
