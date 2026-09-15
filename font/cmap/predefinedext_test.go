// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package cmap_test

import (
	"bytes"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/page"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/reader"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/parser"
)

// TestPredefined writes text through a predefined CMap and reads it back.
// The identity CMap encodes every CID, so only a CMap with a real
// code-to-CID mapping checks that the glyphs are assigned CIDs the CMap
// has codes for.
func TestPredefined(t *testing.T) {
	cases := []struct {
		cmapName string
		text     string
	}{
		{"Adobe-Japan1-7", "Hello"},
		{"UniJIS-UCS2-H", "§ 5 ± 3 °C"},
	}
	for _, c := range cases {
		t.Run(c.cmapName, func(t *testing.T) {
			testPredefined(t, c.cmapName, c.text)
		})
	}
}

func testPredefined(t *testing.T, cmapName, testText string) {
	buf := &bytes.Buffer{}

	// step 1: write a complete PDF document

	doc, err := document.WriteSinglePage(buf, document.A5r, pdf.V2_0, nil)
	if err != nil {
		t.Fatal(err)
	}

	fontInfo, err := sfnt.Read(bytes.NewReader(goregular.TTF), parser.NewBudget(int64(len(goregular.TTF))))
	if err != nil {
		t.Fatal(err)
	}

	cmapInfo, err := cmap.Predefined(cmapName)
	if err != nil {
		t.Fatal(err)
	}

	fontOpt := &truetype.OptionsComposite{
		Language: language.English,
		CMap:     cmapInfo,
	}

	F, err := truetype.NewComposite(fontInfo, fontOpt)
	if err != nil {
		t.Fatal(err)
	}

	fontRefObj, err := doc.RM.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	fontRef := fontRefObj.(pdf.Reference)

	doc.TextSetFont(F, 12.0)
	doc.TextBegin()
	doc.TextFirstLine(50, 200)
	doc.TextShow(testText)
	doc.TextEnd()

	err = doc.Close()
	if err != nil {
		t.Fatal(err)
	}

	// step 2: read the PDF document, find the font dict, and check
	r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()), nil)
	if err != nil {
		t.Fatal(err)
	}

	fontDict, err := pdf.NewCursor(r).DictTyped(fontRef, "Font")
	if err != nil {
		t.Fatal(err)
	}

	encoding, err := pdf.NewCursor(r).Name(fontDict["Encoding"])
	if err != nil {
		t.Fatal(err)
	} else if encoding != pdf.Name(cmapName) {
		t.Errorf("expected encoding %q, got %q", cmapName, encoding)
	}

	if _, present := fontDict["ToUnicode"]; present {
		t.Error("unexpected ToUnicode entry in font dictionary")
	}

	// step 3: extract the text from the PDF document and check
	_, pageDict, err := pagetree.GetPage(r, 0)
	if err != nil {
		t.Fatal(err)
	}
	x := pdf.NewExtractor(r)
	pg, err := pdf.Decode(pdf.CursorAt(x, nil), pageDict, page.Decode)
	if err != nil {
		t.Fatal(err)
	}
	rd := reader.New(x)
	allText := ""
	rd.Character = func(c font.Code) error {
		allText += c.Text
		return nil
	}
	err = rd.ProcessPage(pg)
	if err != nil {
		t.Fatal(err)
	}

	if allText != testText {
		t.Errorf("expected text %q, got %q", testText, allText)
	}

	err = r.Close()
	if err != nil {
		t.Error(err)
	}
}
