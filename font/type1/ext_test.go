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
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/internal/many"
)

// TestEncoding checks that the encoding of a Type 1 font is the standard
// encoding, if the set of included characters is in the standard encoding.
func TestEncoding(t *testing.T) {
	goRegular := many.GoRegular
	t1, err := many.Type1(goRegular)
	if err != nil {
		t.Fatal(err)
	}
	metrics, err := many.AFM(goRegular)
	if err != nil {
		t.Fatal(err)
	}
	F, err := type1.New(t1, metrics)
	if err != nil {
		t.Fatal(err)
	}

	// Embed the font
	data := pdf.NewData(pdf.V1_7)
	E, err := F.Embed(data, &font.Options{ResName: "F"})
	if err != nil {
		t.Fatal(err)
	}
	gg := E.Layout(10, ".MiAbc")
	for _, g := range gg.Seq {
		E.CodeAndWidth(nil, g.GID, g.Text) // allocate codes
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
