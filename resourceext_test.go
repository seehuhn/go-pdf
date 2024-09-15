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

package pdf_test

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/internal/debug/tempfile"
	"seehuhn.de/go/pdf/pagetree"
)

// TestResourceManager creates a document which uses a font on two different
// pages, and checks that the font is only embedded once.
func TestResourceManager(t *testing.T) {
	buf := tempfile.New()

	// part 1: create a document with a font

	doc, err := document.WriteMultiPage(buf, document.A4, pdf.V2_0, nil)
	if err != nil {
		t.Fatal(err)
	}

	F, err := gofont.Regular.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	page1 := doc.AddPage()
	page1.SetFontNameInternal(F, "X")
	page1.TextBegin()
	page1.TextSetFont(F, 12)
	page1.TextFirstLine(100, 600)
	page1.TextShow("Hello, world!")
	page1.TextEnd()
	err = page1.Close()
	if err != nil {
		t.Fatal(err)
	}

	page2 := doc.AddPage()
	page2.SetFontNameInternal(F, "X")
	page2.TextBegin()
	page2.TextSetFont(F, 12)
	page2.TextFirstLine(100, 600)
	page2.TextShow("second page")
	page2.TextEnd()
	err = page2.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = doc.Close()
	if err != nil {
		t.Fatal(err)
	}

	// part 2: read the document and check that the font is only embedded once

	r, err := pdf.NewReader(buf, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, dict1, err := pagetree.GetPage(r, 0)
	if err != nil {
		t.Fatal(err)
	}
	resources1, err := pdf.GetDict(r, dict1["Resources"])
	if err != nil {
		t.Fatal(err)
	}
	font1, err := pdf.GetDict(r, resources1["Font"])
	if err != nil {
		t.Fatal(err)
	}

	_, dict2, err := pagetree.GetPage(r, 1)
	if err != nil {
		t.Fatal(err)
	}
	resources2, err := pdf.GetDict(r, dict2["Resources"])
	if err != nil {
		t.Fatal(err)
	}
	font2, err := pdf.GetDict(r, resources2["Font"])
	if err != nil {
		t.Fatal(err)
	}

	if font1["X"] != font2["X"] {
		t.Errorf("%s != %s", pdf.AsString(font1), pdf.AsString(font2))
	}
}
