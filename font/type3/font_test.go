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

package type3

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/postscript/funit"
)

func TestType3(t *testing.T) {
	paper := document.A5
	doc, err := document.CreateSinglePage("test-type3.pdf", paper, pdf.V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}

	F1 := New(1000)

	bbox := funit.Rect16{LLx: 0, LLy: 0, URx: 750, URy: 750}
	g, err := F1.AddGlyph("A", 1000, bbox, true)
	if err != nil {
		t.Fatal(err)
	}
	g.Rectangle(0, 0, 750, 750)
	g.Fill()
	err = g.Close()
	if err != nil {
		t.Fatal(err)
	}

	g, err = F1.AddGlyph("B", 1000, bbox, true)
	if err != nil {
		t.Fatal(err)
	}
	g.MoveTo(0, 0)
	g.LineTo(750, 750)
	g.LineTo(750, 0)
	g.Fill()
	err = g.Close()
	if err != nil {
		t.Fatal(err)
	}

	F1Dict, err := F1.Embed(doc.Out, nil)
	if err != nil {
		t.Fatal(err)
	}

	doc.TextBegin()
	doc.TextSetFont(F1Dict, 12)
	doc.TextFirstLine(72, 340)
	doc.TextShow("ABABAB")
	doc.TextEnd()

	err = doc.Close()
	if err != nil {
		t.Fatal(err)
	}
}

var _ font.Embedded = (*embedded)(nil)
