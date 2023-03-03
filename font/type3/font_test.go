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
	"seehuhn.de/go/pdf/graphics"
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

	g, err := F1Builder.AddGlyph(pdf.Name("A"), 1000, funit.Rect{0, 0, 750, 750}, true)
	if err != nil {
		t.Fatal(err)
	}
	g.Println("0 0 750 750 re")
	g.Println("f")
	err = g.Close()
	if err != nil {
		t.Fatal(err)
	}

	g, err = F1Builder.AddGlyph(pdf.Name("B"), 1000, funit.Rect{0, 0, 750, 750}, true)
	if err != nil {
		t.Fatal(err)
	}
	g.Println("0 0 m")
	g.Println("375 750 l")
	g.Println("750 0 l")
	g.Println("f")
	err = g.Close()
	if err != nil {
		t.Fatal(err)
	}

	F1 := F1Builder.MakeFont("F1")

	pageTree := pages.InstallTree(w, nil)

	page, err := graphics.NewPage(w)
	if err != nil {
		t.Fatal(err)
	}

	page.BeginText()
	page.SetNewFont(F1, 12)
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
	_, err = pageTree.AppendPage(dict)
	if err != nil {
		t.Fatal(err)
	}

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
}
