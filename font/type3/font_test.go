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

package type3

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pages"
	"seehuhn.de/go/sfnt/funit"
)

func TestType3(t *testing.T) {
	w, err := pdf.Create("test-type3.pdf")
	if err != nil {
		t.Fatal(err)
	}

	F1Builder, err := New(1000)
	if err != nil {
		t.Fatal(err)
	}

	bbox := funit.Rect{LLx: 0, LLy: 0, URx: 750, URy: 750}
	g, err := F1Builder.AddGlyph(pdf.Name("A"), 1000, bbox, true)
	if err != nil {
		t.Fatal(err)
	}
	g.Rectangle(0, 0, 750, 750)
	g.Fill()
	err = g.Close()
	if err != nil {
		t.Fatal(err)
	}

	g, err = F1Builder.AddGlyph(pdf.Name("B"), 1000, bbox, true)
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

	F1, err := F1Builder.MakeFont()
	if err != nil {
		t.Fatal(err)
	}

	pageTree := pages.InstallTree(w, nil)

	page, err := pages.NewPage(w)
	if err != nil {
		t.Fatal(err)
	}

	F1Dict, err := F1.Embed(w, "F")
	if err != nil {
		t.Fatal(err)
	}

	page.BeginText()
	page.SetFont(F1Dict, 12)
	page.StartLine(72, 340)
	page.ShowText("ABABAB")
	page.EndText()

	dict, err := page.Close()
	if err != nil {
		t.Fatal(err)
	}
	dict["MediaBox"] = pages.A5
	if err != nil {
		t.Fatal(err)
	}
	_, err = pageTree.AppendPage(dict, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
}
