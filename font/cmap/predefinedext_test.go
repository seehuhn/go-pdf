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
	"fmt"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/text/language"
	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding/cidenc"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/reader"
	"seehuhn.de/go/sfnt"
)

func TestPredefined(t *testing.T) {
	const testText = "Hello"

	buf := &bytes.Buffer{}

	// step 1: write a complete PDF document

	doc, err := document.WriteSinglePage(buf, document.A5r, pdf.V2_0, nil)
	if err != nil {
		t.Fatal(err)
	}

	fontInfo, err := sfnt.Read(bytes.NewReader(goregular.TTF))
	if err != nil {
		t.Fatal(err)
	}

	lookup, err := fontInfo.CMapTable.GetBest()
	if err != nil {
		t.Fatal(err)
	}

	cmapInfo, err := cmap.Predefined("Adobe-Japan1-7")
	if err != nil {
		t.Fatal(err)
	}

	fontOpt := &truetype.OptionsComposite{
		Language:    language.English,
		WritingMode: font.Horizontal,
		MakeGIDToCID: func() cmap.GIDToCID {
			return cmap.NewGIDToCIDFromROS(cmapInfo.ROS, lookup)
		},
		MakeEncoder: func(cid0Width float64, wMode font.WritingMode) cidenc.CIDEncoder {
			if cmapInfo.WMode != wMode {
				panic(fmt.Sprintf("cmap %s has wMode %d, but requested wMode is %d", cmapInfo.Name, cmapInfo.WMode, wMode))
			}
			enc, err := cidenc.NewFromCMap(cmapInfo, cid0Width)
			if err != nil {
				panic(err)
			}
			return enc
		},
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
	r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}

	fontDict, err := pdf.GetDictTyped(r, fontRef, "Font")
	if err != nil {
		t.Fatal(err)
	}

	encoding, err := pdf.GetName(r, fontDict["Encoding"])
	if err != nil {
		t.Fatal(err)
	} else if encoding != "Adobe-Japan1-7" {
		t.Errorf("expected encoding 'Adobe-Japan1-7', got %q", encoding)
	}

	if _, present := fontDict["ToUnicode"]; present {
		t.Error("unexpected ToUnicode entry in font dictionary")
	}

	// step 3: extract the text from the PDF document and check
	_, pageDict, err := pagetree.GetPage(r, 0)
	if err != nil {
		t.Fatal(err)
	}
	reader := reader.New(r, nil)
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
