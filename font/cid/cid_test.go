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
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/sfnt/glyph"
)

func TestCID(t *testing.T) {
	w, err := document.CreateSinglePage("test-otf-cid.pdf", 10+16*20, 5+32*20+5, nil)
	if err != nil {
		t.Fatal(err)
	}

	FF, err := LoadFont("../../../otf/SourceSerif4-Regular.otf", language.AmericanEnglish)
	if err != nil {
		t.Fatal(err)
	}
	F, err := FF.Embed(w.Out, "F")
	if err != nil {
		t.Fatal(err)
	}

	geom := F.GetGeometry()
	for i := 0; i < 512; i++ {
		row := i / 16
		col := i % 16
		gid := glyph.ID(i + 2)

		width := geom.Widths[gid]
		gg := []glyph.Info{
			{
				Gid:     gid,
				Advance: width,
			},
		}

		w.TextStart()
		w.TextSetFont(F, 16)
		w.TextFirstLine(float64(5+20*col+10), float64(32*20-10-20*row))
		w.TextShowGlyphsAligned(gg, 0, 0.5)
		w.TextEnd()
	}

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
}
