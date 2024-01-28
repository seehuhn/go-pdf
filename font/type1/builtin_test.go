// seehuhn.de/go/pdf - a library for reading and writing PDF files
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

package type1

import (
	"os"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
)

// TestEmbedBuiltin tests that the 14 standard PDF fonts can be
// embedded and that (for PDF-1.7) the font program is not included.
func TestEmbedBuiltin(t *testing.T) {
	for _, F := range All {
		t.Run(string(F), func(t *testing.T) {
			data := pdf.NewData(pdf.V1_7)

			E, err := F.Embed(data, "F")
			if err != nil {
				t.Fatal(err)
			}

			gg := E.Layout("Hello World")
			for _, g := range gg { // allocate codes
				E.CodeAndWidth(nil, g.GID, g.Text)
			}

			err = E.Close()
			if err != nil {
				t.Fatal(err)
			}

			dicts, err := font.ExtractDicts(data, E.PDFObject())
			if err != nil {
				t.Fatal(err)
			}
			if dicts.FontDict["BaseFont"] != pdf.Name(F) {
				t.Errorf("wrong BaseFont: %s != %s", dicts.FontDict["BaseFont"], F)
			}
			if dicts.FontProgram != nil {
				t.Errorf("font program wrongly included")
			}
		})
	}
}

// TestExtractBuiltin tests that one of the 14 standard PDF fonts,
// embedded using a information, can be extracted again.
func TestExtractBuiltin(t *testing.T) {
	data := pdf.NewData(pdf.V1_7)
	ref := data.Alloc()
	fontDict := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": pdf.Name("Times-Roman"),
	}
	err := data.Put(ref, fontDict)
	if err != nil {
		t.Fatal(err)
	}

	dicts, err := font.ExtractDicts(data, ref)
	if err != nil {
		t.Fatal(err)
	}

	info, err := Extract(data, dicts)
	if err != nil {
		t.Fatal(err)
	}

	if !info.IsStandard() {
		t.Errorf("built-in font not recognized")
	}
}

func TestUnknownBuiltin(t *testing.T) {
	F := Builtin("unknown font")
	w := pdf.NewData(pdf.V1_7)
	_, err := F.Embed(w, "F")
	if !os.IsNotExist(err) {
		t.Errorf("wrong error: %s", err)
	}
}

var _ font.Font = TimesRoman
