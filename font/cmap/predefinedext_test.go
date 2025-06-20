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
	"testing"

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding/cidenc"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/font/truetype"
)

func TestPredefined(t *testing.T) {
	doc, err := document.CreateSinglePage("test.pdf", document.A5r, pdf.V2_0, nil)
	if err != nil {
		t.Fatal(err)
	}

	fontOpt := &truetype.Options{
		Language:     language.English,
		Composite:    true,
		WritingMode:  font.Horizontal,
		MakeGIDToCID: cmap.NewGIDToCIDSequential,
		MakeEncoder: func(cid0Width float64, wMode font.WritingMode) cidenc.CIDEncoder {
			var cmapInfo *cmap.File
			if wMode == font.Horizontal {
				cmapInfo, err = cmap.Predefined("Adobe-Japan1-7")
			} else {
				panic("vertical writing mode not supported in this test")
			}
			enc, err := cidenc.NewFromCMap(cmapInfo, cid0Width)
			if err != nil {
				panic(err)
			}
			return enc
		},
	}
	F, err := gofont.Regular.New(fontOpt)
	if err != nil {
		t.Fatal(err)
	}

	doc.TextSetFont(F, 12.0)
	doc.TextBegin()
	doc.TextFirstLine(50, 200)
	doc.TextShow("Hello")
	doc.TextEnd()

	err = doc.Close()
	if err != nil {
		t.Fatal(err)
	}
}
