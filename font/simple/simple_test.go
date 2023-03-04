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
	"testing"

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pages"
	"seehuhn.de/go/sfnt/glyph"
)

func TestSimple(t *testing.T) {
	w, err := pdf.Create("test-otf-simple.pdf")
	if err != nil {
		t.Fatal(err)
	}

	F, err := EmbedFile(w, "../../../otf/SourceSerif4-Regular.otf", "F", language.AmericanEnglish)
	if err != nil {
		t.Fatal(err)
	}

	pageTree := pages.InstallTree(w, nil)

	g, err := pages.NewPage(w)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 256; i++ {
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
		g.StartLine(float64(5+20*col+10), float64(16*20-10-20*row))
		g.ShowGlyphsAligned(gg, 0, 0.5)
		g.EndText()
	}

	dict, err := g.Close()
	if err != nil {
		t.Fatal(err)
	}
	dict["MediaBox"] = &pdf.Rectangle{
		URx: 10 + 16*20,
		URy: 5 + 16*20 + 5,
	}
	_, err = pageTree.AppendPage(dict)
	if err != nil {
		t.Fatal(err)
	}

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestMakeSubset(t *testing.T) {
	origEncoding := make([]glyph.ID, 256)
	for i := 0; i < 10; i++ {
		origEncoding[32+10*i] = glyph.ID(2*i + 4)
	}

	subsetEncoding, subsetGlyphs := makeSubset(origEncoding)
	if len(subsetGlyphs) != 11 {
		t.Errorf("wrong number of glyphs: %d", len(subsetGlyphs))
	}
	if subsetGlyphs[0] != 0 {
		t.Errorf("wrong glyph ID for .notdef: %d", subsetGlyphs[0])
	}
	for i := 0; i < 10; i++ {
		if subsetGlyphs[i+1] != glyph.ID(2*i+4) {
			t.Errorf("wrong glyph ID for %d: %d", i, subsetGlyphs[i+1])
		}
	}

	for i, subsetGID := range subsetEncoding {
		origGid := subsetGlyphs[subsetGID]
		if origGid != origEncoding[i] {
			t.Errorf("wrong mapping for %d: %d", i, origGid)
		}
	}
}
