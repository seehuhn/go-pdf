// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package squarefont_test

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/internal/squarefont"
)

func TestRendering(t *testing.T) {
	t.Skip("visial test only")

	doc, err := document.CreateSinglePage("test.pdf", document.A5r, pdf.V2_0, nil)
	if err != nil {
		t.Fatal(err)
	}

	label := font.Must(standard.Courier.New())

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
