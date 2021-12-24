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

package opentype

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/pages"
)

func TestSimple(t *testing.T) {
	w, err := pdf.Create("test-otf-simple.pdf")
	if err != nil {
		t.Fatal(err)
	}

	tt, err := sfnt.Open("otf/SourceSerif4-Regular.otf", nil)
	if err != nil {
		t.Fatal(err)
	}
	F, err := EmbedFontSimple(w, tt, "F")
	if err != nil {
		t.Fatal(err)
	}

	page, err := pages.SinglePage(w, &pages.Attributes{
		Resources: &pages.Resources{
			Font: map[pdf.Name]pdf.Object{
				F.InstName: F.Ref,
			},
		},
		MediaBox: &pdf.Rectangle{
			URx: 10 + 16*20,
			URy: 5 + 16*20 + 5,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	text := map[font.GlyphID]rune{}
	for r, gid := range tt.CMap {
		rOld, ok := text[gid]
		if !ok || r < rOld {
			text[gid] = r
		}
	}

	for i := 0; i < 256; i++ {
		row := i / 16
		col := i % 16
		gid := font.GlyphID(i + 2)

		gg := F.Layout([]rune{text[gid]}) // try to establish glyph -> rune mapping
		if len(gg) != 1 || gg[0].Gid != gid {
			gg = []font.Glyph{
				{Gid: gid},
			}
		}

		layout := &font.Layout{
			Font:     F,
			FontSize: 16,
			Glyphs:   gg,
		}
		layout.Draw(page, float64(10+20*col), float64(16*20-10-20*row))
	}

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
}
