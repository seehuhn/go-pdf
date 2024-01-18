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

package font_test

import (
	"math"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/postscript/funit"
)

func TestWidthsFull(t *testing.T) {
	data := pdf.NewData(pdf.V2_0)

	goRegular, err := gofont.OpenType(gofont.GoRegular)
	if err != nil {
		t.Fatal(err)
	}
	F, err := opentype.NewCFFComposite(goRegular, nil)
	if err != nil {
		t.Fatal(err)
	}
	E, err := F.Embed(data, "F")
	if err != nil {
		t.Fatal(err)
	}

	sampleText := "Hello World!"

	// Layout and encode a string to make sure the corresponding glyphs are
	// included in the embedded font.
	gg := E.Layout(sampleText)
	var s pdf.String
	for _, g := range gg {
		s = E.AppendEncoded(s, g.GID, g.Text)
	}
	err = E.Close()
	if err != nil {
		t.Fatal(err)
	}

	fontDicts, err := font.ExtractDicts(data, E.PDFObject())
	if err != nil {
		t.Fatal(err)
	}
	DW, err := pdf.GetNumber(data, fontDicts.CIDFontDict["DW"])
	if err != nil {
		t.Fatal(err)
	}
	W, err := font.DecodeWidthsComposite(data, fontDicts.CIDFontDict["W"], float64(DW))
	if err != nil {
		t.Fatal(err)
	}

	F1 := E.(font.NewFontComposite)
	i := 0
	F1.CS().AllCodes(s)(func(code pdf.String, valid bool) bool {
		cid := F1.CodeToCID(code)
		w, ok := W[cid]
		if !ok {
			w = float64(DW)
		}
		wFromPDF := funit.Int16(math.Round(w * float64(goRegular.UnitsPerEm) / 1000))
		wFromFont := goRegular.GlyphWidth(gg[i].GID)

		if wFromPDF != wFromFont {
			t.Errorf("widths differ for CID %d: %d vs %d", cid, wFromPDF, wFromFont)
		}

		i++
		return true
	})
}
