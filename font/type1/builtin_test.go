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
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/sfnt/glyph"
)

func TestSimple(t *testing.T) {
	paper := &pdf.Rectangle{URx: 10 + 16*20, URy: 5 + 16*20 + 5}
	doc, err := document.CreateSinglePage("test-builtin-simple.pdf", paper, nil)
	if err != nil {
		t.Fatal(err)
	}

	F, err := TimesRoman.Embed(doc.Out, "F")
	if err != nil {
		t.Fatal(err)
	}

	geom := F.GetGeometry()
	for i := 0; i < 256; i++ {
		row := i / 16
		col := i % 16
		gid := glyph.ID(i + 2)

		w := geom.Widths[gid]
		gg := []glyph.Info{
			{
				Gid:     gid,
				Advance: w,
			},
		}

		doc.TextStart()
		doc.TextSetFont(F, 16)
		doc.TextFirstLine(float64(5+20*col+10), float64(16*20-10-20*row))
		doc.TextShowGlyphsAligned(gg, 0, 0.5)
		doc.TextEnd()
	}

	err = doc.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSpace(t *testing.T) {
	for _, F := range All {
		gid, width := font.GetGID(F, ' ')
		if gid == 0 || width == 0 {
			t.Errorf("%s: space not found", string(F))
		}
	}
}
