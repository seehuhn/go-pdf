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

package dict_test

import (
	"bytes"
	"testing"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/type1glyphs"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/fonttypes"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/reader"
)

func TestType1Embedding(t *testing.T) {
	const (
		testText    = "Helllo"
		uniqueChars = 4 // 'H', 'e', 'l', 'o'
	)

	buf := &bytes.Buffer{}

	// step 1: write a complete PDF document

	doc, err := document.WriteSinglePage(buf, document.A5r, pdf.V2_0, nil)
	if err != nil {
		t.Fatal(err)
	}

	F := fonttypes.Type1WithMetrics()

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
	r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}

	fontDict, err := pdf.GetDictTyped(r, fontRef, "Font")
	if err != nil {
		t.Fatal(err)
	}

	_, err = pdf.GetName(r, fontDict["Encoding"])
	if err != nil {
		t.Fatal(err)
	}

	if _, present := fontDict["ToUnicode"]; present {
		t.Error("unexpected ToUnicode entry in font dictionary")
	}

	// step 3: extract the font file and check
	x := pdf.NewExtractor(r)
	parsedDict, err := extract.Dict(x, fontDict)
	if err != nil {
		t.Fatal(err)
	}
	fontInfo, ok := parsedDict.FontInfo().(*dict.FontInfoSimple)
	if !ok {
		t.Fatal("expected FontInfoSimple type")
	}

	if fontInfo.FontFile == nil {
		t.Fatal("expected embedded font file")
	}
	if fontInfo.FontFile.Type != glyphdata.Type1 {
		t.Fatal("expected Type1 font")
	}

	// Extract font data using FromStream helper
	t1Font, err := type1glyphs.FromStream(fontInfo.FontFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(t1Font.Glyphs) != uniqueChars+1 { // +1 for the .notdef glyph
		t.Errorf("expected %d unique glyphs, got %d", uniqueChars+1, len(t1Font.Glyphs))
	}

	// step 4: extract the text from the PDF document and check
	_, pageDict, err := pagetree.GetPage(r, 0)
	if err != nil {
		t.Fatal(err)
	}
	reader := reader.New(r)
	allText := ""
	reader.Text = func(text string) error {
		allText += text
		return nil
	}
	err = reader.ParsePage(pageDict, matrix.Identity)
	if err != nil {
		t.Fatal(err)
	}

	if allText != testText {
		t.Errorf("expected text 'Hello', got %q", allText)
	}

	err = r.Close()
	if err != nil {
		t.Error(err)
	}
}
