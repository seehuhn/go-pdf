// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package simple

import (
	"os"
	"testing"

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/pages"
	"seehuhn.de/go/pdf/sfnt"
	"seehuhn.de/go/pdf/sfnt/glyph"
)

func TestSimple(t *testing.T) {
	fd, err := os.Open("../../sfnt/otf/SourceSerif4-Regular.otf")
	// fd, err := os.Open("../../sfnt/ttf/SourceSerif4-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}

	fontInfo, err := sfnt.Read(fd)
	if err != nil {
		fd.Close()
		t.Fatal(err)
	}

	err = fd.Close()
	if err != nil {
		t.Error(err)
	}

	w, err := pdf.Create("test-otf-simple.pdf")
	if err != nil {
		t.Fatal(err)
	}

	F, err := Embed(w, fontInfo, "F", language.BritishEnglish)
	if err != nil {
		t.Fatal(err)
	}

	pageTree := pages.NewPageTree(w, nil)
	page, err := pageTree.NewPage(&pages.Attributes{
		Resources: &pages.Resources{
			Font: pdf.Dict{
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

	for i := 0; i < 256; i++ {
		row := i / 16
		col := i % 16
		gid := glyph.ID(i + 2)

		w := fontInfo.GlyphWidth(gid)
		layout := &font.Layout{
			Font:     F,
			FontSize: 16,
			Glyphs: []glyph.Info{{
				Gid:     gid,
				XOffset: 0,
				YOffset: 0,
				Advance: w,
			}},
		}
		dx := (20 - 16*float64(w)/float64(fontInfo.UnitsPerEm)) / 2
		layout.Draw(page, float64(5+20*col)+dx, float64(16*20-10-20*row))
	}
	page.Close()

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
}
