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

package extract

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/pattern"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/stdmtx"
)

func TestStandardFontFallbackNames(t *testing.T) {
	cases := []struct {
		name pdf.Name
		want string
	}{
		{"Helv", "Helvetica"},
		{"Helvetica", "Helvetica"},
		{"Arial,Bold", "Helvetica-Bold"},
		{"CourB", "Courier-Bold"},
		{"TimesNewRomanPS-ItalicMT", "Times-Italic"},
		{"ZaDb", "ZapfDingbats"},
		{"NoSuchFont", "Helvetica"},
		{"", "Helvetica"},
	}
	for _, c := range cases {
		got := StandardFontFallback(c.name)
		if got == nil {
			t.Fatalf("%q: nil instance", c.name)
		}
		if got.PostScriptName() != c.want {
			t.Errorf("%q: got %s, want %s", c.name, got.PostScriptName(), c.want)
		}
	}
}

func TestStandardFontFallbackShared(t *testing.T) {
	a := StandardFontFallback("Helv")
	b := StandardFontFallback("Arial")
	if a != b {
		t.Error("two names for the same font gave different instances")
	}
}

// TestStandardFontFallbackInstance checks that the substitute looks like a
// font read from a file: it has a descriptor, widths, and decodes codes
// through the font's built-in encoding.
func TestStandardFontFallbackInstance(t *testing.T) {
	F := StandardFontFallback("Helvetica")
	info, ok := F.FontInfo().(*dict.FontInfoSimple)
	if !ok {
		t.Fatalf("FontInfo() = %T, want *dict.FontInfoSimple", F.FontInfo())
	}
	if info.PostScriptName != "Helvetica" {
		t.Errorf("PostScriptName = %q", info.PostScriptName)
	}
	if info.FontFile != nil {
		t.Error("substitute claims an embedded font program")
	}

	var codes []font.Code
	for c := range F.Codes(pdf.String("a")) {
		codes = append(codes, c)
	}
	if len(codes) != 1 {
		t.Fatalf("got %d codes for one byte", len(codes))
	}
	if codes[0].Width != 0.556 {
		t.Errorf("width of a = %g, want 0.556", codes[0].Width)
	}
	if codes[0].Text != "a" {
		t.Errorf("text of a = %q", codes[0].Text)
	}

	// a symbolic font decodes through its own built-in encoding
	Z := StandardFontFallback("ZaDb")
	zapf := stdmtx.Metrics["ZapfDingbats"]
	want := zapf.Width[zapf.Encoding[0x34]] / 1000
	for c := range Z.Codes(pdf.String("4")) {
		if c.Width != want {
			t.Errorf("ZapfDingbats width of code 0x34 = %g, want %g", c.Width, want)
		}
	}
}

// TestResourcesInstallFallback checks that a resource dictionary read from a
// file carries the fallback, so that a content stream naming a missing font
// still gets one.
func TestResourcesInstallFallback(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	ref := w.Alloc()
	if err := w.Put(ref, pdf.Dict{"ProcSet": pdf.Array{pdf.Name("PDF")}}); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	res, err := pdf.Decode(pdf.NewCursor(w), ref, Resources)
	if err != nil {
		t.Fatal(err)
	}
	if res.FontFallback == nil {
		t.Fatal("no fallback installed")
	}

	s := content.NewState(content.Page, res)
	err = s.ApplyStateChanges(content.OpTextSetFont, []pdf.Object{pdf.Name("Missing"), pdf.Integer(10)})
	if err != nil {
		t.Fatal(err)
	}
	if s.GState.TextFont == nil || s.GState.TextFont.PostScriptName() != "Helvetica" {
		t.Errorf("missing font not substituted: %v", s.GState.TextFont)
	}
}

// TestSynthesisedResourcesCarryFallback checks the two read paths which
// normalise a missing Resources entry into an empty dictionary: the
// substitute has to reach them too, or an identical file would show its text
// or not depending on whether it spells out an empty /Resources.
func TestSynthesisedResourcesCarryFallback(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	formRef := w.Alloc()
	err := w.Put(formRef, &pdf.Stream{
		Dict: pdf.Dict{
			"Subtype": pdf.Name("Form"),
			"BBox":    pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(10), pdf.Integer(10)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	patRef := w.Alloc()
	err = w.Put(patRef, &pdf.Stream{
		Dict: pdf.Dict{
			"PatternType": pdf.Integer(1),
			"PaintType":   pdf.Integer(1),
			"TilingType":  pdf.Integer(1),
			"BBox":        pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(10), pdf.Integer(10)},
			"XStep":       pdf.Integer(10),
			"YStep":       pdf.Integer(10),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	c := pdf.NewCursor(w)

	f, err := pdf.Decode(c, formRef, Form)
	if err != nil {
		t.Fatal(err)
	}
	if f.Res == nil || f.Res.FontFallback == nil {
		t.Error("form without /Resources lost the fallback")
	}

	p, err := pdf.Decode(c, patRef, Pattern)
	if err != nil {
		t.Fatal(err)
	}
	tiling, ok := p.(*pattern.Type1)
	if !ok {
		t.Fatalf("Pattern() = %T, want *pattern.Type1", p)
	}
	if tiling.Res == nil || tiling.Res.FontFallback == nil {
		t.Error("tiling pattern without /Resources lost the fallback")
	}
}
