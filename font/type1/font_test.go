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
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata/type1glyphs"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// New must reject multiple master fonts: the embed layer is responsible for
// instantiating a single-master snapshot before calling New.
func TestNewRejectsMultipleMaster(t *testing.T) {
	mmFont := makefont.MMType1()
	_, err := type1.New(mmFont, nil)
	if err == nil {
		t.Fatal("expected an error for a multiple master font")
	}
}

// A font program taken out of a PDF file carries the tagged name of the subset
// it is.  The tag names the subset rather than the font, so it must not reach
// the choice of glyph list: ZapfDingbats names its glyphs "a1", "a2", ... and
// those mean something else in every other font.
func TestNewIgnoresSubsetTagWhenNamingGlyphs(t *testing.T) {
	const pointingHand = '☞' // the "a12" glyph of ZapfDingbats

	for _, name := range []string{"ZapfDingbats", "AAAAAA+ZapfDingbats"} {
		psFont := makefont.Type1()
		psFont.FontName = name
		psFont.Glyphs["a12"] = psFont.Glyphs["A"]

		F, err := type1.New(psFont, nil)
		if err != nil {
			t.Fatal(err)
		}

		seq := F.Layout(nil, 10, string(pointingHand))
		if len(seq.Seq) != 1 {
			t.Fatalf("%s: %q laid out as %d glyphs, want 1",
				name, pointingHand, len(seq.Seq))
		}
		if got := F.GlyphNames[seq.Seq[0].GID]; got != "a12" {
			t.Errorf("%s: %q drawn with the glyph %q, want %q",
				name, pointingHand, got, "a12")
		}
	}
}

// A font program whose name cannot be written as a PostScript name is embedded
// under a repaired one rather than refused.  The name reaches the /BaseFont
// entry, the descriptor and the embedded program alike, so all three must agree
// on the name settled here.
func TestEmbedRepairsUnwritableFontName(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"Gr\xfc\xdfe-Regular", "Gre-Regular"}, // Latin-1, so not valid UTF-8
		{"Go Regular", "GoRegular"},            // white space
		{"Go(Regular)", "GoRegular"},           // PostScript delimiters
		{strings.Repeat("x", 500), strings.Repeat("x", subset.MaxBaseNameLen)},
	} {
		psFont := makefont.Type1()
		psFont.FontName = tc.name

		F, err := type1.New(psFont, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := F.PostScriptName(); got != tc.want {
			t.Errorf("the font is named %q, want %q", got, tc.want)
		}

		d := embedType1(t, F)
		if d.PostScriptName != tc.want {
			t.Errorf("the dictionary names the font %q, want %q",
				d.PostScriptName, tc.want)
		}
		if got := subset.Join(d.SubsetTag, d.PostScriptName); d.Descriptor.FontName != got {
			t.Errorf("the descriptor names the font %q, want %q",
				d.Descriptor.FontName, got)
		}

		// the program carries the same name, so that the font can be embedded
		// again from what the file holds
		prog, err := type1glyphs.FromStream(d.FontFile)
		if err != nil {
			t.Fatal(err)
		}
		if want := subset.Join(d.SubsetTag, tc.want); prog.FontName != want {
			t.Errorf("the embedded program calls itself %q, want %q",
				prog.FontName, want)
		}
	}
}

// embedType1 writes the font into a document of its own and reads the font
// dictionary back out.
func embedType1(t *testing.T, F *type1.Instance) *dict.Type1 {
	t.Helper()

	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range F.Layout(nil, 10, "Hello").Seq {
		F.Encode(g.GID, g.Text)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	fontDict, err := extract.Dict(pdf.CursorAt(pdf.NewExtractor(w), nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := fontDict.(*dict.Type1)
	if !ok {
		t.Fatalf("unexpected font dictionary type %T", fontDict)
	}
	return d
}

// A name which is a PostScript name, non-ASCII included, is embedded as it
// stands.
func TestEmbedKeepsNonASCIIFontName(t *testing.T) {
	const name = "宋体-Regular"

	psFont := makefont.Type1()
	psFont.FontName = name

	F, err := type1.New(psFont, nil)
	if err != nil {
		t.Fatal(err)
	}

	if got := embedType1(t, F).PostScriptName; got != name {
		t.Errorf("the font is named %q, want %q", got, name)
	}
}
