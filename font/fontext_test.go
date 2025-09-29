// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package font_test

import (
	"io"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/fonttypes"
)

// TestSpaceIsBlank tests that space characters of common fonts are blank.
func TestSpaceIsBlank(t *testing.T) {
	for _, sample := range fonttypes.All {
		t.Run(sample.Label, func(t *testing.T) {
			F := sample.MakeFont()
			gg := F.Layout(nil, 10, " ")
			if len(gg.Seq) != 1 {
				t.Fatalf("expected 1 glyph, got %d", len(gg.Seq))
			}
			geom := F.GetGeometry()
			if !geom.GlyphExtents[gg.Seq[0].GID].IsZero() {
				t.Errorf("expected blank glyph, got %v",
					geom.GlyphExtents[gg.Seq[0].GID])
			}
		})
	}
}

func TestToUnicodeSimple1(t *testing.T) {
	for _, sample := range fonttypes.All {
		if sample.Composite {
			continue
		}
		t.Run(sample.Label, func(t *testing.T) {
			const fontSize = 10
			const fontName = "X"

			F := sample.MakeFont()
			seq := F.Layout(nil, fontSize, "ABC")
			if len(seq.Seq) != 3 {
				t.Fatalf("expected 3 glyphs, got %d", len(seq.Seq))
			}

			buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm := pdf.NewResourceManager(buf)

			page := graphics.NewWriter(io.Discard, rm)
			page.SetFontNameInternal(F, fontName)
			page.TextSetFont(F, fontSize)
			page.TextBegin()
			page.TextShowGlyphs(seq)
			page.TextEnd()

			if page.Err != nil {
				t.Fatal(page.Err)
			}
			err := rm.Close()
			if err != nil {
				t.Fatal(err)
			}

			ref := page.Resources.Font[fontName]
			x := pdf.NewExtractor(buf)
			d, err := dict.ExtractDict(x, ref)
			if err != nil {
				t.Fatal(err)
			}

			tu := getToUnicode(d)
			if tu != nil {
				t.Errorf("expected ToUnicode file for %q", sample.Label)
			}
		})
	}
}

func TestToUnicodeSimple2(t *testing.T) {
	for _, sample := range fonttypes.All {
		if sample.Composite {
			continue
		}
		t.Run(sample.Label, func(t *testing.T) {
			const fontSize = 10
			const fontName = "X"

			F := sample.MakeFont()
			seq := F.Layout(nil, fontSize, "ABC")
			if len(seq.Seq) != 3 {
				t.Fatalf("expected 3 glyphs, got %d", len(seq.Seq))
			}
			seq.Seq[1].Text = "D" // one glyph with non-standard text

			buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm := pdf.NewResourceManager(buf)

			page := graphics.NewWriter(io.Discard, rm)
			page.SetFontNameInternal(F, fontName)
			page.TextSetFont(F, fontSize)
			page.TextBegin()
			page.TextShowGlyphs(seq)
			page.TextEnd()

			if page.Err != nil {
				t.Fatal(page.Err)
			}
			err := rm.Close()
			if err != nil {
				t.Fatal(err)
			}

			ref := page.Resources.Font[fontName]
			x := pdf.NewExtractor(buf)
			d, err := dict.ExtractDict(x, ref)
			if err != nil {
				t.Fatal(err)
			}

			tu := getToUnicode(d)
			if tu == nil {
				t.Fatal("missing ToUnicode file")
			}
			if len(tu.Singles) != 1 {
				t.Fatalf("expected 1 single mapping, got %d", len(tu.Singles))
			}
			if tu.Singles[0].Value != "D" {
				t.Errorf("expected single mapping for 'D', got %q", tu.Singles[0].Value)
			}
		})
	}
}

func getToUnicode(d dict.Dict) *cmap.ToUnicodeFile {
	switch d := d.(type) {
	case *dict.Type1:
		return d.ToUnicode
	case *dict.TrueType:
		return d.ToUnicode
	case *dict.Type3:
		return d.ToUnicode
	case *dict.CIDFontType0:
		return d.ToUnicode
	case *dict.CIDFontType2:
		return d.ToUnicode
	default:
		panic("unknown font dictionary type")
	}
}
