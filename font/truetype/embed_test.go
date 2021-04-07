// seehuhn.de/go/pdf - support for reading and writing PDF files
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

package truetype

import (
	"fmt"
	"log"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/pages"
)

func TestEmbedFont(t *testing.T) {
	w, err := pdf.Create("test.pdf")
	if err != nil {
		t.Fatal(err)
	}

	F1, err := builtin.Embed(w, "F1", "Times-Roman", font.AdobeStandardLatin)
	if err != nil {
		log.Fatal(err)
	}

	// F2, err := Embed(w, "ttf/FreeSerif.ttf", nil)
	F2, err := Embed(w, "F2", "ttf/SourceSerif4-Regular.ttf", nil)
	if err != nil {
		log.Fatal(err)
	}

	contents, err := pages.SinglePage(w, &pages.Attributes{
		Resources: map[pdf.Name]pdf.Object{
			"Font": pdf.Dict{
				F1.Name: F1.Ref,
				F2.Name: F2.Ref,
			},
		},
		MediaBox: pages.A4,
	})
	if err != nil {
		t.Fatal(err)
	}

	for col := 0; col < 6; col++ {
		contents.Println("BT")
		contents.Printf("%d %f Td\n", 50+90*col, contents.URy-50-10)
		for c := 50 * col; c < 50*(col+1); c++ {
			if c >= 256 {
				break
			}
			contents.Println("/F1 12 Tf")
			s := pdf.String(fmt.Sprintf("%3d: ", c))

			idx := F1.CMap[rune(c)]
			wd := F1.Width[idx]
			textWithKern := pdf.Array{
				pdf.String(s),
				pdf.Number(-0.5 * (1000 - float64(wd))),
				pdf.String(F1.Enc(idx)),
				pdf.Number(-0.5 * (1000 - float64(wd))),
				pdf.String(" "),
			}
			textWithKern.PDF(contents)
			contents.Print(" TJ ")

			contents.Println("/F2 12 Tf")
			out := pdf.String(F2.Enc(F2.CMap[rune(c)]))
			out.PDF(contents)
			contents.Println(" Tj")
			contents.Println("0 -15 TD")
		}
		contents.Println("ET")
	}

	err = contents.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
}
