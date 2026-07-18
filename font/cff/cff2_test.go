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

package cff_test

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/geom/path"
	"seehuhn.de/go/geom/vec"

	sfntcff "seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/varfont"
)

type pathStep struct {
	Cmd path.Command
	Pts []vec.Vec2
}

// collectPathSteps materializes a path.Path iterator for comparison with
// cmp.Diff, mirroring go-sfnt's cff2_test.go helper of the same name.
func collectPathSteps(p path.Path) []pathStep {
	var steps []pathStep
	for cmd, pts := range p {
		steps = append(steps, pathStep{Cmd: cmd, Pts: append([]vec.Vec2(nil), pts...)})
	}
	return steps
}

// a variable CFF2 font embedded via the simple path is instanced (at the
// requested coordinates) and converted to static CFF; the CID-keyed
// instance's glyph carries no name of its own (instanceCFF2 in go-sfnt
// leaves Glyph.Name empty), so the simple-font glyph-name machinery in
// simpleenc must synthesize one from the Unicode text instead. This
// confirms that path works rather than rejecting CID-keyed single-FD
// outlines.
func TestCFF2SimpleVariable(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	coords := map[string]float64{"wght": 900}
	F, err := cff.NewSimple(varfont.CFF2(), &cff.OptionsSimple{Variations: coords})
	if err != nil {
		t.Fatal(err)
	}

	ref, err := rm.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	gg := F.Layout(nil, 12, "A")
	var gid glyph.ID
	for _, g := range gg.Seq {
		gid = g.GID
		if _, ok := F.Encode(g.GID, g.Text); !ok {
			t.Fatal("failed to encode glyph")
		}
	}

	// the synthesized glyph name comes from the Unicode text, since the
	// CID-keyed instance's glyph has no name of its own.
	if name := F.GlyphName(gid); name != "A" {
		t.Errorf("synthesized glyph name = %q, want %q", name, "A")
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

	// the embedded font file is a plain static CFF font: no CFF2 table
	// (cffglyphs only ever produces *sfntcff.Font), and not CID-keyed.
	back, err := cffglyphs.FromStream(d.FontFile)
	if err != nil {
		t.Fatal(err)
	}
	if back.Outlines.IsCIDKeyed() {
		t.Error("simple-path FontFile3 is CID-keyed; want a plain simple CFF")
	}
	if len(back.Outlines.Glyphs) == 0 {
		t.Fatal("re-parsed font has no glyphs")
	}
}

// a variable CFF2 font embedded via the composite path is instanced and
// converted to static CID-keyed CFF; touching glyphs from both FDs of the
// fixture forces a genuine CIDFontType0C, rather than the automatic
// simple-font collapse that makeDict applies for single-FD subsets.
func TestCFF2CompositeVariable(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	coords := map[string]float64{"wght": 900}
	F, err := cff.NewComposite(varfont.CFF2(), &cff.OptionsComposite{Variations: coords})
	if err != nil {
		t.Fatal(err)
	}

	ref, err := rm.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	gg := F.Layout(nil, 12, "AB")
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
}

// a static (non-variable) CFF2 font embedded via the simple path is
// converted to static CFF via ConvertCFF2, even though it has no axes to
// pin; the resulting glyph paths match the CFF2 default instance exactly
// (no blends to evaluate).
func TestCFF2SimpleStatic(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	src := varfont.StaticCFF2()
	F, err := cff.NewSimple(src, nil)
	if err != nil {
		t.Fatal(err)
	}

	ref, err := rm.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	gg := F.Layout(nil, 12, "A")
	var gid glyph.ID
	for _, g := range gg.Seq {
		gid = g.GID
		if _, ok := F.Encode(g.GID, g.Text); !ok {
			t.Fatal("failed to encode glyph")
		}
	}
	name := F.GlyphName(gid)
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
	if d.FontFile == nil || d.FontFile.Type != glyphdata.CFFSimple {
		t.Fatalf("FontFile type: got %v, want CFFSimple", d.FontFile)
	}

	back, err := cffglyphs.FromStream(d.FontFile)
	if err != nil {
		t.Fatal(err)
	}

	var backGID glyph.ID
	found := false
	for i, g := range back.Outlines.Glyphs {
		if g.Name == name {
			backGID = glyph.ID(i)
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("glyph %q not found in re-parsed font", name)
	}

	origOutlines := src.Outlines.(*sfntcff.OutlinesCFF2)
	wantSteps := collectPathSteps(origOutlines.Path(varfont.CFF2GIDBox))
	gotSteps := collectPathSteps(back.Outlines.Path(backGID))
	if diff := cmp.Diff(wantSteps, gotSteps); diff != "" {
		t.Errorf("glyph path differs from the CFF2 default path (-want +got):\n%s", diff)
	}

	wantW := math.Round(src.GlyphWidthPDF(varfont.CFF2GIDBox))
	gotW, ok := d.GlyphWidth("A")
	if !ok {
		t.Fatal(`no width for "A"`)
	}
	if math.Abs(gotW*1000-wantW) > 0.5 {
		t.Errorf("width: got %v, want %v", gotW*1000, wantW)
	}
}
