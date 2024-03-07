// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package type1

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
)

// TestEmbedBuiltin tests that the 14 standard PDF fonts can be
// embedded and that (for PDF-1.7) the font program is not included.
func TestEmbedBuiltin(t *testing.T) {
	for _, F := range Standard {
		t.Run(string(F), func(t *testing.T) {
			data := pdf.NewData(pdf.V1_7)

			E, err := F.Embed(data, &font.Options{ResName: "F"})
			if err != nil {
				t.Fatal(err)
			}

			gg := E.Layout(nil, 10, "Hello World")
			for _, g := range gg.Seq { // allocate codes
				E.CodeAndWidth(nil, g.GID, g.Text)
			}

			err = E.Close()
			if err != nil {
				t.Fatal(err)
			}

			dicts, err := font.ExtractDicts(data, E.PDFObject())
			if err != nil {
				t.Fatal(err)
			}
			if dicts.FontDict["BaseFont"] != pdf.Name(F) {
				t.Errorf("wrong BaseFont: %s != %s", dicts.FontDict["BaseFont"], F)
			}
			if dicts.FontProgram != nil {
				t.Errorf("font program wrongly included")
			}
		})
	}
}

// TestExtractBuiltin tests that one of the 14 standard PDF fonts,
// embedded using a information, can be extracted again.
func TestExtractBuiltin(t *testing.T) {
	data := pdf.NewData(pdf.V1_7)
	ref := data.Alloc()
	fontDict := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": pdf.Name("Times-Roman"),
	}
	err := data.Put(ref, fontDict)
	if err != nil {
		t.Fatal(err)
	}

	dicts, err := font.ExtractDicts(data, ref)
	if err != nil {
		t.Fatal(err)
	}

	info, err := Extract(data, dicts)
	if err != nil {
		t.Fatal(err)
	}

	if !info.IsStandard() {
		t.Errorf("built-in font not recognized")
	}
}

func TestUnknownBuiltin(t *testing.T) {
	F := Builtin("unknown font")
	w := pdf.NewData(pdf.V1_7)
	_, err := F.Embed(w, &font.Options{ResName: "F"})
	if !os.IsNotExist(err) {
		t.Errorf("wrong error: %s", err)
	}
}

// TestGlyphLists tests that the glyph lists of the 14 standard PDF
// fonts are consistent between the .pfb and the .afm files.
func TestGlyphLists(t *testing.T) {
	for _, F := range Standard {
		psFont, err := F.psFont()
		if err != nil {
			t.Fatal(err)
		}
		metrics, err := F.AFM()
		if err != nil {
			t.Fatal(err)
		}

		glyphNames1 := psFont.GlyphList()
		glyphNames2 := metrics.GlyphList()
		if d := cmp.Diff(glyphNames1, glyphNames2); d != "" {
			t.Errorf("%-22s differ: %s", F, d)
		}
	}
}

// TestGlyphWidths tests that the glyph widths of the 14 standard PDF
// fonts are consistent between the .pfb and the .afm files.
func TestGlyphWidths(t *testing.T) {
	for _, F := range Standard {
		psFont, err := F.psFont()
		if err != nil {
			t.Fatal(err)
		}
		metrics, err := F.AFM()
		if err != nil {
			t.Fatal(err)
		}

		for name, g := range psFont.Glyphs {
			w1 := g.WidthX
			w2 := metrics.Glyphs[name].WidthX
			if w1 != w2 {
				t.Errorf("%-22s %-8s width=%g, claimedWidth=%g",
					F, name, w1, w2)
			}
		}
	}
}

// TestBlankGlyphs checks that glyphs are marked as blank in the
// metrics file if and only if they are blank in the .pfb file.
func TestBlankGlyphs(t *testing.T) {
	for _, F := range Standard {
		psFont, err := F.psFont()
		if err != nil {
			t.Fatal(err)
		}
		metrics, err := F.AFM()
		if err != nil {
			t.Fatal(err)
		}
		for name, g := range psFont.Glyphs {
			isBlank := len(g.Cmds) == 0
			claimedBlank := metrics.Glyphs[name].BBox.IsZero()
			// TODO(voss): fix this for the .notdef glyphs by
			// life-patching the type 1 fonts after loading.
			if isBlank != claimedBlank && name != ".notdef" {
				t.Errorf("%-22s %-8s isBlank=%v, claimedBlank=%v",
					F, name, isBlank, claimedBlank)
			}
		}
	}
}

// TestLigatures checks that letters are correctly combined into ligatures.
func TestLigatures(t *testing.T) {
	ligatures := []string{"ﬀ=ff", "ﬁ=fi", "ﬂ=fl", "ﬃ=ffi", "ﬄ=ffl"}
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for i, F := range Standard {
			data := pdf.NewData(v)
			E, err := F.Embed(data, nil)
			if err != nil {
				t.Fatal(err)
			}

			geom := E.GetGeometry()

			for _, lig := range ligatures {
				gg := E.Layout(nil, 10, lig)

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
					if string(gg.Seq[0].Text) != string(rr[0]) {
						t.Errorf("font %d: ligature %q: unexpected glyph for %q[0]: %q",
							i, lig, lig, gg.Seq[0].Text)
					}

					if string(gg.Seq[1].Text) != "=" {
						// test is broken
						t.Fatalf("font %d: ligature %q: unexpected glyph for %q[1]: %q",
							i, lig, lig, gg.Seq[1].Text)
					}

					if string(gg.Seq[2].Text) != string(rr[2:]) {
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
}

var _ font.Font = Courier
var _ font.Font = CourierBold
var _ font.Font = CourierBoldOblique
var _ font.Font = CourierOblique
var _ font.Font = Helvetica
var _ font.Font = HelveticaBold
var _ font.Font = HelveticaBoldOblique
var _ font.Font = HelveticaOblique
var _ font.Font = TimesRoman
var _ font.Font = TimesBold
var _ font.Font = TimesBoldItalic
var _ font.Font = TimesItalic
var _ font.Font = Symbol
var _ font.Font = ZapfDingbats
