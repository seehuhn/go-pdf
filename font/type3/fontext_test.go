// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package type3_test

import (
	"math"
	"strings"
	"testing"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/squarefont"
	"seehuhn.de/go/sfnt/glyph"
)

// TestDescriptorGlyphSpace checks that the descriptor of a Type 3 font is
// written in the font's own glyph space, as the Widths array is, whatever
// the font matrix.
func TestDescriptorGlyphSpace(t *testing.T) {
	for _, s := range squarefont.All {
		if !strings.HasPrefix(s.Label, "Type3") {
			continue
		}
		t.Run(s.Label, func(t *testing.T) {
			F := s.MakeFont()

			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)
			ref, err := rm.Embed(F)
			if err != nil {
				t.Fatal(err)
			}
			for _, g := range F.Layout(nil, 10, "A").Seq {
				F.Encode(g.GID, g.Text)
			}
			if err := rm.Close(); err != nil {
				t.Fatal(err)
			}

			obj, err := extract.Dict(pdf.CursorAt(pdf.NewExtractor(w), nil), ref, false)
			if err != nil {
				t.Fatal(err)
			}
			d := obj.(*dict.Type3)
			fd := d.Descriptor

			// squarefont's values are in PDF glyph space, 1/1000 text space
			qh := 1 / (1000 * d.FontMatrix[0])
			qv := 1 / (1000 * d.FontMatrix[3])
			cases := []struct {
				name      string
				got, want float64
			}{
				{"MissingWidth", fd.MissingWidth, squarefont.NotdefWidth * qh},
				{"Ascent", fd.Ascent, squarefont.Ascent * qv},
				{"Descent", fd.Descent, squarefont.Descent * qv},
				{"Leading", fd.Leading, squarefont.Leading * qv},
				{"CapHeight", fd.CapHeight, squarefont.CapHeight * qv},
				{"XHeight", fd.XHeight, squarefont.XHeight * qv},
			}
			for _, c := range cases {
				if math.Abs(c.got-c.want) > 1e-6 {
					t.Errorf("%s = %g, want %g", c.name, c.got, c.want)
				}
			}
		})
	}
}

// twoGlyphFont returns a Type 3 font with glyphs "A" and "B", and the
// content stream both glyphs use.
func twoGlyphFont(t *testing.T) (font.Layouter, content.Stream) {
	t.Helper()
	b := builder.New(content.Glyph, nil, pdf.V2_0)
	b.Type3ColoredGlyph(500, 0)
	b.Rectangle(0, 0, 500, 500)
	b.Fill()
	stream, err := b.Harvest()
	if err != nil {
		t.Fatal(err)
	}
	F, err := (&type3.Font{
		Glyphs: []*type3.Glyph{
			{},
			{Name: "A", Content: stream},
			{Name: "B", Content: stream},
		},
		FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
	}).New()
	if err != nil {
		t.Fatal(err)
	}
	return F, stream
}

// TestFontInfo checks that a new font instance describes all its glyph
// procedures and the codes assigned so far, so that a renderer can draw the
// font before it is written.
func TestFontInfo(t *testing.T) {
	F, stream := twoGlyphFont(t)
	gid := F.Layout(nil, 10, "A").Seq[0].GID
	code, ok := F.Encode(gid, "A")
	if !ok {
		t.Fatal("cannot encode A")
	}

	info, ok := F.FontInfo().(*dict.FontInfoType3)
	if !ok {
		t.Fatalf("FontInfo() is %T, want *dict.FontInfoType3", F.FontInfo())
	}
	want := matrix.Matrix{0.001, 0, 0, 0.001, 0, 0}
	if info.FontMatrix != want {
		t.Errorf("font matrix %v, want %v", info.FontMatrix, want)
	}
	if name, _ := info.Encoding(byte(code)); name != "A" {
		t.Errorf("code %d maps to %q, want \"A\"", code, name)
	}
	for _, name := range []pdf.Name{"A", "B"} {
		if cp := info.CharProcs[name]; cp == nil || cp.Content != stream {
			t.Errorf("the glyph procedure of %s is missing", name)
		}
	}
}

// TestEmbedUsedGlyphs checks that only glyphs which have been assigned
// character codes are written to the file.
func TestEmbedUsedGlyphs(t *testing.T) {
	F, _ := twoGlyphFont(t)

	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range F.Layout(nil, 10, "A").Seq {
		F.Encode(g.GID, g.Text)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	obj, err := extract.Dict(pdf.CursorAt(pdf.NewExtractor(w), nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	d := obj.(*dict.Type3)
	if _, ok := d.CharProcs["A"]; !ok {
		t.Error("glyph A is missing")
	}
	if _, ok := d.CharProcs["B"]; ok {
		t.Error("unused glyph B was written")
	}
}

// TestEmbedGlyphNames checks that glyphs keep their names in the file, so
// that every code the encoding maps leads to a glyph procedure.
func TestEmbedGlyphNames(t *testing.T) {
	b := builder.New(content.Glyph, nil, pdf.V2_0)
	b.Type3ColoredGlyph(500, 0)
	b.Rectangle(0, 0, 500, 500)
	b.Fill()
	stream, err := b.Harvest()
	if err != nil {
		t.Fatal(err)
	}
	glyphNames := []string{"a-b", "@", strings.Repeat("x", 40), "", ".notdef", "A"}
	glyphs := []*type3.Glyph{{}}
	for _, name := range glyphNames {
		glyphs = append(glyphs, &type3.Glyph{Name: name, Content: stream})
	}
	F, err := (&type3.Font{
		Glyphs:     glyphs,
		FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
	}).New()
	if err != nil {
		t.Fatal(err)
	}

	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	codes := make(map[string]byte)
	for i, name := range glyphNames {
		code, ok := F.Encode(glyph.ID(i+1), "x")
		if !ok {
			t.Fatalf("cannot encode %q", name)
		}
		codes[name] = byte(code)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	obj, err := extract.Dict(pdf.CursorAt(pdf.NewExtractor(w), nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	d := obj.(*dict.Type3)
	for _, name := range glyphNames {
		if got, ok := d.Encoding(codes[name]); got != name || !ok {
			t.Errorf("code %d maps to (%q, %t), want (%q, true)", codes[name], got, ok, name)
		}
		if d.Width[codes[name]] != 500 {
			t.Errorf("code %d has width %g, want 500", codes[name], d.Width[codes[name]])
		}
		if _, ok := d.CharProcs[pdf.Name(name)]; !ok {
			t.Errorf("glyph %q is missing", name)
		}
	}
}
