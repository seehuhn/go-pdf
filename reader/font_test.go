// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package reader

import (
	"bytes"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/internal/fonttypes"
	"seehuhn.de/go/pdf/page"
	"seehuhn.de/go/pdf/pagetree"
)

func TestExtractText(t *testing.T) {
	// TODO(voss): test both, fonts with and without ToUnicode maps

	line1 := "Hello World!"
	line2 := "— Jochen Voß"
	textEmbedded := line1 + line2

	for _, sample := range fonttypes.All {
		t.Run(sample.Label, func(t *testing.T) {
			// Create a document with two lines of text.
			buf := &bytes.Buffer{}
			doc, err := document.WriteSinglePage(buf, document.A5r, pdf.V1_7, nil)
			if err != nil {
				t.Fatal(err)
			}

			F := sample.MakeFont()

			doc.TextSetFont(F, 12)
			doc.TextBegin()
			doc.TextFirstLine(72, 100)
			doc.TextShow(line1)
			doc.TextSecondLine(0, -18)
			doc.TextShow(line2)
			doc.TextEnd()
			err = doc.Close()
			if err != nil {
				t.Fatal(err)
			}

			// os.WriteFile("test.pdf", buf.Bytes(), 0644)

			// Now try to read back the text.
			r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()), nil)
			if err != nil {
				t.Fatal(err)
			}

			var pieces []string
			contents := New(pdf.NewExtractor(r))
			contents.Character = func(c font.Code) error {
				pieces = append(pieces, c.Text)
				return nil
			}

			_, pageDict, err := pagetree.GetPage(r, 0)
			if err != nil {
				t.Fatal(err)
			}
			x := pdf.NewExtractor(r)
			pg, err := pdf.CursorAt(x, nil).Decode(pageDict, page.Decode)
			if err != nil {
				t.Fatal(err)
			}
			err = contents.ProcessPage(pg)
			if err != nil {
				t.Fatal(err)
			}

			textReceived := strings.Join(pieces, "")
			if textReceived != textEmbedded {
				t.Errorf("expected %q, got %q", textEmbedded, textReceived)
			}
		})
	}
}
