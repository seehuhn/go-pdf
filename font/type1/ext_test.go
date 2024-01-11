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

package type1_test

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/type1"
)

// TestEncoding checks that the encoding of a Type 1 font is the standard
// encoding, if the set of included characters allows for this.
func TestEncoding(t *testing.T) {
	t1, err := gofont.Type1(gofont.GoRegular)
	if err != nil {
		t.Fatal(err)
	}
	F, err := type1.New(t1)
	if err != nil {
		t.Fatal(err)
	}

	// Embed the font
	// and make sure codes are allocated for a few characters.
	data := pdf.NewData(pdf.V1_7)
	E, err := F.Embed(data, "F")
	if err != nil {
		t.Fatal(err)
	}
	gg := E.Layout(".MiAbc", 100)
	var s pdf.String
	for _, g := range gg {
		s = E.AppendEncoded(s[:0], g.Gid, g.Text)
	}
	err = E.Close()
	if err != nil {
		t.Fatal(err)
	}

	dicts, err := font.ExtractDicts(data, E.PDFObject())
	if err != nil {
		t.Fatal(err)
	}
	info, err := type1.Extract(data, dicts)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 256; i++ {
		if info.Encoding[i] != ".notdef" && info.Encoding[i] != pdfenc.StandardEncoding[i] {
			t.Error(i, info.Encoding[i])
		}
	}
}
