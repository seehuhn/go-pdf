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

package squarefont_test

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/internal/squarefont"
)

func TestRendering(t *testing.T) {
	t.Skip("visial test only")

	doc, err := document.CreateSinglePage("test.pdf", document.A5r, pdf.V2_0, nil)
	if err != nil {
		t.Fatal(err)
	}

	label := standard.Courier.New()

	doc.TextBegin()
	for i, sample := range squarefont.All {
		switch i {
		case 0:
			doc.TextFirstLine(50, 350)
		case 1:
			doc.TextSecondLine(0, -25)
		default:
			doc.TextNextLine()
		}
		doc.TextSetFont(sample.MakeFont(), 20)
		doc.TextShow("A ")
		doc.TextSetFont(label, 20)
		doc.TextShow(sample.Label)
	}
	doc.TextEnd()

	err = doc.Close()
	if err != nil {
		t.Fatal(err)
	}
}
