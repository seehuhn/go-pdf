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

package type1_test

import (
	"math"
	"slices"
	"testing"

	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/embed"
	"seehuhn.de/go/pdf/font/loader"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func helveticaMetrics(t *testing.T) *afm.Metrics {
	t.Helper()
	r, err := loader.NewFontLoader().Open("Helvetica", loader.FontTypeAFM)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	m, err := afm.Read(r)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestMetricsOnlyNew(t *testing.T) {
	if _, err := type1.New(nil, nil); err == nil {
		t.Error("New(nil, nil) succeeded")
	}

	m := helveticaMetrics(t)
	F, err := type1.New(nil, m)
	if err != nil {
		t.Fatal(err)
	}

	// the instance always has .notdef as glyph 0
	names := m.GlyphList()
	if names[0] != ".notdef" {
		names = slices.Insert(names, 0, ".notdef")
	}
	if len(F.GlyphNames) != len(names) {
		t.Fatalf("wrong number of glyphs: got %d, want %d", len(F.GlyphNames), len(names))
	}
	for i, name := range names {
		if F.GlyphNames[i] != name {
			t.Fatalf("glyph %d: got %q, want %q", i, F.GlyphNames[i], name)
		}
		if want := m.GlyphWidthPDF(name) / 1000; math.Abs(F.Widths[i]-want) > 1e-9 {
			t.Errorf("width of %q: got %g, want %g", name, F.Widths[i], want)
		}
		if got := F.GlyphWidthPDF(name); got != m.GlyphWidthPDF(name) {
			t.Errorf("GlyphWidthPDF(%q) = %g, want %g", name, got, m.GlyphWidthPDF(name))
		}
	}

	d := F.Descriptor
	// a height the AFM file omits is taken from the font bounding box
	wantAscent, wantDescent := m.Ascent, m.Descent
	if wantAscent <= 0 {
		wantAscent = m.FontBBoxPDF().URy
	}
	if wantDescent == 0 {
		wantDescent = m.FontBBoxPDF().LLy
	}
	if d.Ascent != math.Round(wantAscent) || d.Descent != math.Round(wantDescent) {
		t.Errorf("wrong ascent/descent: %g/%g", d.Ascent, d.Descent)
	}
	if d.CapHeight != math.Round(m.CapHeight) || d.XHeight != math.Round(m.XHeight) {
		t.Errorf("wrong cap/x height: %g/%g", d.CapHeight, d.XHeight)
	}
	if d.ItalicAngle != pdf.Round(m.ItalicAngle, 1) {
		t.Errorf("wrong italic angle: %g", d.ItalicAngle)
	}
	if d.IsFixedPitch != m.IsFixedPitch {
		t.Errorf("wrong IsFixedPitch: %v", d.IsFixedPitch)
	}
	if d.FontBBox != m.FontBBoxPDF().Rounded() {
		t.Errorf("wrong font bbox: %v", d.FontBBox)
	}
	if F.PostScriptName() != m.FontName {
		t.Errorf("wrong name: %q", F.PostScriptName())
	}
}

func gidOf(F *type1.Instance, name string) glyph.ID {
	for i, n := range F.GlyphNames {
		if n == name {
			return glyph.ID(i)
		}
	}
	return 0
}

func TestMetricsOnlyLayout(t *testing.T) {
	m := helveticaMetrics(t)
	F, err := type1.New(nil, m)
	if err != nil {
		t.Fatal(err)
	}

	var kern *afm.KernPair
	for i, k := range m.Kern {
		if k.Left == "A" && k.Right == "V" {
			kern = &m.Kern[i]
			break
		}
	}
	if kern == nil {
		t.Fatal("no AV kern pair in the metrics")
	}
	seq := F.Layout(nil, 1000, "AV")
	want := m.GlyphWidthPDF("A") + kern.Adjust
	if got := seq.Seq[0].Advance; math.Abs(got-want) > 1e-6 {
		t.Errorf("kerned advance of A: got %g, want %g", got, want)
	}

	if m.Glyphs["f"].Ligatures["i"] != "fi" {
		t.Fatal("metrics lack the fi ligature")
	}
	repl := m.Glyphs["f"].Ligatures["i"]
	seq = F.Layout(nil, 10, "fi")
	if len(seq.Seq) != 1 || seq.Seq[0].GID != gidOf(F, repl) {
		t.Errorf("fi not ligated: %v", seq.Seq)
	}
}

func TestMetricsOnlyNotdef(t *testing.T) {
	F, err := type1.New(nil, helveticaMetrics(t))
	if err != nil {
		t.Fatal(err)
	}
	seq := F.Layout(nil, 10, "\u4e2d")
	if len(seq.Seq) != 1 || seq.Seq[0].GID != 0 {
		t.Fatalf("unmapped rune: got %v, want GID 0", seq.Seq)
	}
	if F.GlyphNames[0] != ".notdef" {
		t.Errorf("GID 0 is %q, want .notdef", F.GlyphNames[0])
	}
}

func TestMetricsOnlyEmptyGlyphs(t *testing.T) {
	m := &afm.Metrics{FontName: "Empty", Glyphs: map[string]*afm.GlyphInfo{}}
	F, err := type1.New(nil, m)
	if err != nil {
		t.Fatal(err)
	}
	if len(F.GlyphNames) != 1 || F.GlyphNames[0] != ".notdef" || F.Widths[0] != 0 {
		t.Errorf("wrong glyphs %q, widths %v", F.GlyphNames, F.Widths)
	}
	F.Layout(nil, 10, "A")
	F.Clone()
}

func TestMetricsOnlyEmbed(t *testing.T) {
	m := helveticaMetrics(t)
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		F, err := type1.New(nil, m)
		if err != nil {
			t.Fatal(err)
		}
		w, _ := memfile.NewPDFWriter(t, v, nil)
		rm := pdf.NewResourceManager(w)
		ref, err := rm.Embed(F)
		if err != nil {
			t.Fatal(err)
		}
		seq := F.Layout(nil, 10, "AV fi")
		var codes pdf.String
		for _, g := range seq.Seq {
			c, ok := F.Encode(g.GID, g.Text)
			if !ok {
				t.Fatalf("no code for %q", g.Text)
			}
			codes = append(codes, byte(c))
		}
		if err := rm.Close(); err != nil {
			t.Fatal(err)
		}

		x := pdf.NewExtractor(w)
		dictObj, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
		if err != nil {
			t.Fatal(err)
		}
		fontDict, ok := dictObj.(*dict.Type1)
		if !ok {
			t.Fatalf("%s: wrong font dictionary type %T", v, dictObj)
		}
		if fontDict.FontFile != nil {
			t.Errorf("%s: unexpected font data", v)
		}
		if fontDict.PostScriptName != m.FontName || fontDict.SubsetTag != "" {
			t.Errorf("%s: wrong name %q+%q", v, fontDict.SubsetTag, fontDict.PostScriptName)
		}
		for i, g := range seq.Seq {
			got := fontDict.Width[codes[i]]
			want := math.Round(m.GlyphWidthPDF(F.GlyphNames[g.GID]))
			if math.Abs(got-want) > 0.5 {
				t.Errorf("%s: width of %q: got %g, want %g", v, g.Text, got, want)
			}
		}
	}
}

// TestMetricsOnlySubsetTag checks that a font without font program is written
// without a subset tag, even if the metrics name it with one.
func TestMetricsOnlySubsetTag(t *testing.T) {
	m := helveticaMetrics(t)
	m.FontName = "ABCDEF+Helvetica"
	F, err := type1.New(nil, m)
	if err != nil {
		t.Fatal(err)
	}
	if got := F.PostScriptName(); got != "Helvetica" {
		t.Errorf("wrong PostScript name %q", got)
	}

	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	dictObj, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	fontDict, ok := dictObj.(*dict.Type1)
	if !ok {
		t.Fatalf("wrong font dictionary type %T", dictObj)
	}
	if fontDict.SubsetTag != "" || fontDict.PostScriptName != "Helvetica" {
		t.Errorf("wrong name %q+%q", fontDict.SubsetTag, fontDict.PostScriptName)
	}
	if fontDict.Descriptor.FontName != "Helvetica" {
		t.Errorf("wrong descriptor name %q", fontDict.Descriptor.FontName)
	}
}

func TestMetricsOnlyOutline(t *testing.T) {
	F, err := type1.New(nil, helveticaMetrics(t))
	if err != nil {
		t.Fatal(err)
	}
	var o font.Outliner = F
	for gid := range F.GlyphNames {
		for range o.Outline(glyph.ID(gid)) {
			t.Fatalf("glyph %q has a non-empty outline", F.GlyphNames[gid])
		}
	}
}

func TestMetricsOnlyEmbedPackage(t *testing.T) {
	F, err := embed.Type1Font(nil, helveticaMetrics(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	if F == nil {
		t.Fatal("no font")
	}
	if _, err := embed.Type1Font(nil, nil, nil); err == nil {
		t.Error("no error without font and metrics")
	}
}

func TestMetricsOnlyClone(t *testing.T) {
	F, err := type1.New(nil, helveticaMetrics(t))
	if err != nil {
		t.Fatal(err)
	}
	c := F.Clone()
	if c.PostScriptName() != F.PostScriptName() {
		t.Error("clone name differs")
	}

	// the two allocate character codes independently
	if c.Simple == F.Simple {
		t.Fatal("clone shares the encoding state")
	}
	gid := F.Layout(nil, 10, "A").Seq[0].GID
	if _, ok := F.Encode(gid, "A"); !ok {
		t.Fatal("no code was allocated")
	}
	if F.Simple.CodesRemaining() == c.Simple.CodesRemaining() {
		t.Error("a code allocated in the original was taken from the clone")
	}
}

func loadMetrics(t *testing.T, name string) *afm.Metrics {
	t.Helper()
	r, err := loader.NewFontLoader().Open(name, loader.FontTypeAFM)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	m, err := afm.Read(r)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// TestMetricsOnlyBuiltinEncoding checks that a metrics-only font with a
// font-specific encoding is written as a symbolic font, using the codes of its
// built-in encoding, so that no /Encoding entry is needed unless a glyph is
// missing from the built-in encoding.
func TestMetricsOnlyBuiltinEncoding(t *testing.T) {
	type testCase struct {
		font   string
		glyphs []string // glyph names to show
		text   []string // text for each glyph
		hasEnc bool     // whether an /Encoding entry is expected
	}
	cases := []testCase{
		{"Symbol", []string{"alpha", "beta", "Euro", "space"}, []string{"α", "β", "€", " "}, false},
		{"Symbol", []string{"alpha", "apple"}, []string{"α", ""}, true},
		{"ZapfDingbats", []string{"a1", "a20", "space"}, []string{"✁", "✔", " "}, false},
	}
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for _, tc := range cases {
			m := loadMetrics(t, tc.font)
			builtinCode := make(map[string]byte)
			for code, name := range m.Encoding {
				builtinCode[name] = byte(code)
			}

			F, err := type1.New(nil, m)
			if err != nil {
				t.Fatal(err)
			}
			w, _ := memfile.NewPDFWriter(t, v, nil)
			rm := pdf.NewResourceManager(w)
			ref, err := rm.Embed(F)
			if err != nil {
				t.Fatal(err)
			}
			var codes pdf.String
			for i, name := range tc.glyphs {
				gid := gidOf(F, name)
				if gid == 0 {
					t.Fatalf("%s has no glyph %q", tc.font, name)
				}
				c, ok := F.Encode(gid, tc.text[i])
				if !ok {
					t.Fatalf("no code for %q", name)
				}
				if bc, ok := builtinCode[name]; ok && byte(c) != bc {
					t.Errorf("%s %s: %q has code %d, want built-in code %d",
						v, tc.font, name, c, bc)
				}
				codes = append(codes, byte(c))
			}
			if err := rm.Close(); err != nil {
				t.Fatal(err)
			}

			obj, err := pdf.Resolve(w, ref)
			if err != nil {
				t.Fatal(err)
			}
			fontObj := obj.(pdf.Dict)
			if _, hasEnc := fontObj["Encoding"]; hasEnc != tc.hasEnc {
				t.Errorf("%s %s %v: /Encoding present = %t, want %t",
					v, tc.font, tc.glyphs, hasEnc, tc.hasEnc)
			}
			if encDict, ok := fontObj["Encoding"].(pdf.Dict); ok {
				if _, hasBase := encDict["BaseEncoding"]; hasBase {
					t.Errorf("%s %s: unexpected /BaseEncoding", v, tc.font)
				}
			}

			x := pdf.NewExtractor(w)
			dictObj, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
			if err != nil {
				t.Fatal(err)
			}
			fontDict, ok := dictObj.(*dict.Type1)
			if !ok {
				t.Fatalf("wrong font dictionary type %T", dictObj)
			}
			if !fontDict.Descriptor.IsSymbolic {
				t.Errorf("%s %s: font is not marked symbolic", v, tc.font)
			}
			i := 0
			for code := range fontDict.MakeFont().Codes(codes) {
				name := tc.glyphs[i]
				if code.Text != tc.text[i] {
					t.Errorf("%s %s: text of %q: got %q, want %q",
						v, tc.font, name, code.Text, tc.text[i])
				}
				want := m.GlyphWidthPDF(name) / 1000
				if math.Abs(code.Width-want) > 1e-6 {
					t.Errorf("%s %s: width of %q: got %g, want %g",
						v, tc.font, name, code.Width, want)
				}
				i++
			}
		}
	}
}
