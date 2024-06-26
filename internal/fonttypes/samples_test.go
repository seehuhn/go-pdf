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

package fonttypes

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
)

func TestSamples(t *testing.T) {
	for _, sample := range All {
		t.Run(sample.Label, func(t *testing.T) {
			buf := &bytes.Buffer{}
			page, err := document.WriteSinglePage(buf, document.A4, pdf.V1_7, nil)
			if err != nil {
				t.Fatal(err)
			}
			X, err := sample.Embed(page.Out, nil)
			if err != nil {
				t.Fatal(err)
			}

			page.TextSetFont(X, 12)
			page.TextBegin()
			page.TextFirstLine(72, 72)
			page.TextShow(`“Hello World!”`)
			page.TextEnd()
			err = page.Close()
			if err != nil {
				t.Fatal(err)
			}

			r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), nil)
			if err != nil {
				t.Fatal(err)
			}
			dicts, err := font.ExtractDicts(r, X.PDFObject())
			if err != nil {
				t.Fatal(err)
			}

			if dicts.Type != sample.Type {
				t.Errorf("got %q, want %q", dicts.Type, sample.Type)
			}
		})
	}
}
