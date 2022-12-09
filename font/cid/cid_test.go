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

package cid

import (
	"testing"

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pages2"
	"seehuhn.de/go/pdf/sfnt/glyph"
)

func TestCID(t *testing.T) {
	w, err := pdf.Create("test-otf-cid.pdf")
	if err != nil {
		t.Fatal(err)
	}

	F, err := EmbedFile(w, "../../sfnt/otf/SourceSerif4-Regular.otf", "F", language.AmericanEnglish)
	if err != nil {
		t.Fatal(err)
	}

	pageTree := pages2.NewTree(w, nil)

	g, err := graphics.NewPage(w)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 512; i++ {
		row := i / 16
		col := i % 16
		gid := glyph.ID(i + 2)

		w := F.Widths[gid]
		gg := []glyph.Info{
			{
				Gid:     gid,
				Advance: w,
			},
		}

		g.BeginText()
		g.SetFont(F, 16)
		g.StartLine(float64(5+20*col+10), float64(32*20-10-20*row))
		g.ShowGlyphsAligned(gg, -0.5, 0)
		g.EndText()
	}

	dict, err := g.Close()
	if err != nil {
		t.Fatal(err)
	}
	dict["MediaBox"] = &pdf.Rectangle{
		URx: 10 + 16*20,
		URy: 5 + 32*20 + 5,
	}
	_, err = pageTree.AppendPage(dict)
	if err != nil {
		t.Fatal(err)
	}

	ref, err := pageTree.Close()
	if err != nil {
		t.Fatal(err)
	}
	w.Catalog.Pages = ref

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
}
