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

package reader

import (
	"bytes"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/graphics/matrix"
	"seehuhn.de/go/pdf/internal/fonttypes"
	"seehuhn.de/go/pdf/pagetree"
)

func TestExtractText(t *testing.T) {
	t.Skip("reenable this, once ReadFont() is fully implemented")

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

			F := sample.MakeFont(doc.RM)

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
			r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), nil)
			if err != nil {
				t.Fatal(err)
			}

			var pieces []string
			contents := New(r, nil)
			contents.Text = func(text string) error {
				pieces = append(pieces, text)
				return nil
			}

			pageDict, err := pagetree.GetPage(r, 0)
			if err != nil {
				t.Fatal(err)
			}
			err = contents.ParsePage(pageDict, matrix.Identity)
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
