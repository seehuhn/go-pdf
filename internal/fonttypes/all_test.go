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

			F := sample.MakeFont(page.RM)

			page.TextSetFont(F, 12)
			page.TextBegin()
			page.TextFirstLine(72, 72)
			page.TextShow(`“Hello World!”`)
			page.TextEnd()

			fontRef, _, err := pdf.ResourceManagerEmbed(page.RM, F)
			if err != nil {
				t.Fatal(err)
			}

			err = page.Close()
			if err != nil {
				t.Fatal(err)
			}

			r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), nil)
			if err != nil {
				t.Fatal(err)
			}
			dicts, err := font.ExtractDicts(r, fontRef)
			if err != nil {
				t.Fatal(err)
			}

			if dicts.Type != sample.Type {
				t.Errorf("got %q, want %q", dicts.Type, sample.Type)
			}
		})
	}
}

// TestPostScriptName ensures that the .PostScriptName method of all fonts
// works correctly.
func TestPostScriptName(t *testing.T) {
	data := pdf.NewData(pdf.V1_7)
	rm := pdf.NewResourceManager(data)

	for _, sample := range All {
		F := sample.MakeFont(rm)

		name := F.PostScriptName()

		var expected string
		switch sample.Label {
		case "BuiltIn":
			// There is no built-in Go-Regular font.
			expected = "Helvetica"
		case "Type3":
			// Type 3 fonts don't have a PostScript name.
			expected = ""
		default:
			expected = "Go-Regular"
		}

		if name != expected {
			t.Errorf("%s: got %q, want %q", sample.Label, name, expected)
		}
	}
}
