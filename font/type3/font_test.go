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
)

func TestType3(t *testing.T) {
	w, err := pdf.Create("test-type3.pdf")
	if err != nil {
		t.Fatal(err)
	}

	F1Builder, err := New(w, 1000)
	if err != nil {
		t.Fatal(err)
	}

	g, err := F1Builder.AddGlyph('A', 1000)
	if err != nil {
		t.Fatal(err)
	}
	g.Println("1000 0 0 0 750 750 d1")
	g.Println("0 0 750 750 re")
	g.Println("f")
	err = g.Close()
	if err != nil {
		t.Fatal(err)
	}

	g, err = F1Builder.AddGlyph('B', 1000)
	if err != nil {
		t.Fatal(err)
	}
	g.Println("1000 0 0 0 750 750 d1")
	g.Println("0 0 m")
	g.Println("375 750 l")
	g.Println("750 0 l")
	g.Println("f")
	err = g.Close()
	if err != nil {
		t.Fatal(err)
	}

	F1, err := F1Builder.Embed("F1")
	if err != nil {
		t.Fatal(err)
	}

	pageTree := pages.NewPageTree(w, nil)
	page, err := pageTree.NewPage(&pages.Attributes{
		Resources: &pdf.Resources{
			Font: pdf.Dict{
				F1.InstName: F1.Ref,
			},
		},
		MediaBox: pages.A5,
	})
	if err != nil {
		t.Fatal(err)
	}
	page.Println("BT")
	page.Println("/F1 12 Tf")
	page.Println("72 340 Td")
	page.Println("(ABABAB) Tj")
	page.Println("ET")
	page.Close()

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
}
