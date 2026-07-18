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
	"bytes"
	"math"
	"testing"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/parser"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/embed"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/varfont"
)

// embedSimple embeds F, lays out and encodes "A", and reads back the
// resulting simple TrueType font dictionary.
func embedSimple(t *testing.T, F font.Layouter) *dict.TrueType {
	t.Helper()
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	ref, err := rm.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	gg := F.Layout(nil, 12, "A")
	for _, g := range gg.Seq {
		_, _ = F.Encode(g.GID, g.Text)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	dictObj, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := dictObj.(*dict.TrueType)
	if !ok {
		t.Fatalf("wrong font dictionary type: %T", dictObj)
	}
	return d
}

// readbackFontFile parses the embedded FontFile2 back into an sfnt.Font.
func readbackFontFile(t *testing.T, s *glyphdata.Stream) *sfnt.Font {
	t.Helper()
	var buf bytes.Buffer
	if err := s.WriteTo(&buf, &glyphdata.Lengths{}); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	info, err := sfnt.Read(bytes.NewReader(data), parser.NewBudget(int64(len(data))))
	if err != nil {
		t.Fatal(err)
	}
	return info
}

// checkInstanced verifies the round-tripped dictionary of a variable font
// embedded at the given axis coordinates.
func checkInstanced(t *testing.T, d *dict.TrueType, coords map[string]float64) {
	t.Helper()

	inst, err := varfont.Glyf().Instantiate(coords)
	if err != nil {
		t.Fatal(err)
	}

	// widths reflect the instanced advance (GlyphWidth is in text space)
	wantW := math.Round(inst.GlyphWidthPDF(varfont.GIDRect))
	gotW, ok := d.GlyphWidth("A")
	if !ok {
		t.Fatal(`no width for "A"`)
	}
	if math.Abs(gotW*1000-wantW) > 0.5 {
		t.Errorf("width: got %v, want %v", gotW*1000, wantW)
	}

	// descriptor carries the subset tag and the instance PostScript name
	if inst.IsVariable() {
		t.Fatal("instanced font is still variable")
	}
	wantPS := inst.PostScriptName()
	if d.PostScriptName != wantPS {
		t.Errorf("PostScriptName: got %q, want %q", d.PostScriptName, wantPS)
	}
	if len(d.SubsetTag) != 6 {
		t.Errorf("SubsetTag: got %q, want 6 letters", d.SubsetTag)
	}
	if want := d.SubsetTag + "+" + wantPS; d.Descriptor.FontName != want {
		t.Errorf("FontName: got %q, want %q", d.Descriptor.FontName, want)
	}

	// the embedded font file carries no variation tables
	back := readbackFontFile(t, d.FontFile)
	if back.IsVariable() {
		t.Error("embedded font file is still variable")
	}
	if back.Fvar != nil || back.Gvar != nil {
		t.Error("embedded font file retains fvar/gvar tables")
	}
}

// a glyf variable font embedded with explicit variations is instanced.
func TestVarFontSimpleInstanced(t *testing.T) {
	coords := map[string]float64{"wght": 700}
	F, err := truetype.NewSimple(varfont.Glyf(), &truetype.OptionsSimple{Variations: coords})
	if err != nil {
		t.Fatal(err)
	}
	d := embedSimple(t, F)
	checkInstanced(t, d, coords)

	// the instanced width differs from the default
	def, _ := varfont.Glyf().Instantiate(nil)
	gotW, _ := d.GlyphWidth("A")
	if math.Round(def.GlyphWidthPDF(varfont.GIDRect)) == math.Round(gotW*1000) {
		t.Error("instanced width equals the default width; variations had no effect")
	}
}

// a glyf variable font with nil variations is instanced at its defaults.
func TestVarFontSimpleDefault(t *testing.T) {
	F, err := truetype.NewSimple(varfont.Glyf(), nil)
	if err != nil {
		t.Fatal(err)
	}
	d := embedSimple(t, F)
	checkInstanced(t, d, nil)
}

// the opentype dispatcher instances the font before the outline branch.
func TestVarFontOpenTypeDispatcher(t *testing.T) {
	coords := map[string]float64{"wght": 700}
	F, err := opentype.NewSimple(varfont.Glyf(), &opentype.OptionsSimple{Variations: coords})
	if err != nil {
		t.Fatal(err)
	}
	d := embedSimple(t, F)
	checkInstanced(t, d, coords)
}

// the embed dispatcher instances the font before the outline branch.
func TestVarFontEmbedDispatcher(t *testing.T) {
	coords := map[string]float64{"wght": 700}
	F, err := embed.OpenTypeFont(varfont.Glyf(), &embed.Options{Variations: coords})
	if err != nil {
		t.Fatal(err)
	}
	d := embedSimple(t, F)
	checkInstanced(t, d, coords)
}

// an unknown axis tag is rejected by the constructor.
func TestVarFontUnknownAxis(t *testing.T) {
	_, err := truetype.NewSimple(varfont.Glyf(),
		&truetype.OptionsSimple{Variations: map[string]float64{"zzzz": 1}})
	if err == nil {
		t.Error("expected error for unknown variation axis")
	}
}

// embedComposite embeds F, lays out and encodes "A", and reads back the
// resulting CIDFontType2 dictionary.
func embedComposite(t *testing.T, F font.Layouter) *dict.CIDFontType2 {
	t.Helper()
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	ref, err := rm.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	gg := F.Layout(nil, 12, "A")
	for _, g := range gg.Seq {
		_, _ = F.Encode(g.GID, g.Text)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	dictObj, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := dictObj.(*dict.CIDFontType2)
	if !ok {
		t.Fatalf("wrong font dictionary type: %T", dictObj)
	}
	return d
}

// the composite path embeds a variable font without error, and produces a
// dictionary whose naming and widths reflect the instanced font (mirroring
// the simple-font checks in checkInstanced).
func TestVarFontComposite(t *testing.T) {
	coords := map[string]float64{"wght": 700}
	F, err := truetype.NewComposite(varfont.Glyf(), &truetype.OptionsComposite{Variations: coords})
	if err != nil {
		t.Fatal(err)
	}
	d := embedComposite(t, F)

	// the font was pinned to a static instance
	if F.PostScriptName() == "" {
		t.Error("empty PostScript name after instancing")
	}

	inst, err := varfont.Glyf().Instantiate(coords)
	if err != nil {
		t.Fatal(err)
	}

	// descriptor carries the subset tag and the instance PostScript name
	wantPS := inst.PostScriptName()
	if d.PostScriptName != wantPS {
		t.Errorf("PostScriptName: got %q, want %q", d.PostScriptName, wantPS)
	}
	if len(d.SubsetTag) != 6 {
		t.Errorf("SubsetTag: got %q, want 6 letters", d.SubsetTag)
	}
	if want := d.SubsetTag + "+" + wantPS; d.Descriptor.FontName != want {
		t.Errorf("FontName: got %q, want %q", d.Descriptor.FontName, want)
	}

	// widths reflect the instanced advance
	wantW := math.Round(inst.GlyphWidthPDF(varfont.GIDRect))
	gotW, ok := d.GlyphWidth("A")
	if !ok {
		t.Fatal(`no width for "A"`)
	}
	if math.Abs(gotW*1000-wantW) > 0.5 {
		t.Errorf("width: got %v, want %v", gotW*1000, wantW)
	}

	// the instanced width differs from the default
	def, _ := varfont.Glyf().Instantiate(nil)
	if math.Round(def.GlyphWidthPDF(varfont.GIDRect)) == wantW {
		t.Error("instanced width equals the default width; variations had no effect")
	}
}

// the FontDescriptor's font-wide metrics (here CapHeight, via an MVAR
// table) come from the instanced font, not the variable original.
func TestVarFontMVARCapHeight(t *testing.T) {
	coords := map[string]float64{"wght": 700}
	F, err := truetype.NewSimple(varfont.Glyf(), &truetype.OptionsSimple{Variations: coords})
	if err != nil {
		t.Fatal(err)
	}
	d := embedSimple(t, F)

	inst, err := varfont.Glyf().Instantiate(coords)
	if err != nil {
		t.Fatal(err)
	}

	wantCapHeight := float64(inst.CapHeight)
	if d.Descriptor.CapHeight != wantCapHeight {
		t.Errorf("CapHeight: got %v, want %v", d.Descriptor.CapHeight, wantCapHeight)
	}

	def, _ := varfont.Glyf().Instantiate(nil)
	if float64(def.CapHeight) == wantCapHeight {
		t.Error("instanced CapHeight equals the default; the MVAR delta had no effect")
	}
}

// the constructor builds Geometry after instancing: the glyph widths exposed
// through font.Layouter already reflect the pinned instance, not the
// variable original.  This pins the invariant that nothing caches
// pre-instance geometry.
func TestVarFontGeometryInstanced(t *testing.T) {
	coords := map[string]float64{"wght": 700}
	F, err := truetype.NewSimple(varfont.Glyf(), &truetype.OptionsSimple{Variations: coords})
	if err != nil {
		t.Fatal(err)
	}

	inst, err := varfont.Glyf().Instantiate(coords)
	if err != nil {
		t.Fatal(err)
	}

	wantWidth := inst.GlyphWidthPDF(varfont.GIDRect) / 1000
	gotWidth := F.Geometry.Widths[varfont.GIDRect]
	if math.Abs(gotWidth-wantWidth) > 1e-9 {
		t.Errorf("geometry width: got %v, want %v", gotWidth, wantWidth)
	}

	def, _ := varfont.Glyf().Instantiate(nil)
	defWidth := def.GlyphWidthPDF(varfont.GIDRect) / 1000
	if gotWidth == defWidth {
		t.Error("geometry width equals the default width; variations had no effect")
	}
}
