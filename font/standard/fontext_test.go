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

package standard_test

import (
	"fmt"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestEmbedStandard tests that the 14 standard PDF fonts can be
// used and that the font file is not embedded.
func TestEmbedStandard(t *testing.T) {
	for _, standardFont := range standard.All {
		for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
			t.Run(fmt.Sprintf("%s@%s", standardFont, v), func(t *testing.T) {
				data, _ := memfile.NewPDFWriter(v, nil)
				rm := pdf.NewResourceManager(data)

				// Embed the font into a PDF file:

				F := standardFont.New()
				ref, err := rm.Embed(F)
				if err != nil {
					t.Fatal(err)
				}

				var testText string
				switch standardFont {
				case standard.Symbol:
					testText = "∀"
				case standard.ZapfDingbats:
					testText = "♠"
				default:
					testText = "Hello World"
				}
				gg := F.Layout(nil, 10, testText)
				for _, g := range gg.Seq { // allocate codes
					// Use the actual glyph width, not the kerned advance
					_, _ = F.Encode(g.GID, g.Text)
				}

				err = rm.Close()
				if err != nil {
					t.Fatal(err)
				}

				// Read back the font dictionary and check that everything is
				// as expected:

				x := pdf.NewExtractor(data)
				dictObj, err := extract.Dict(x, ref)
				if err != nil {
					t.Fatal(err)
				}

				t1Dict, ok := dictObj.(*dict.Type1)
				if !ok {
					t.Fatalf("unexpected font dictionary type %T", dictObj)
				}

				if t1Dict.PostScriptName != standardFont.PostScriptName() {
					t.Errorf("unexpected PostScriptName %q", t1Dict.PostScriptName)
				}
				if t1Dict.FontFile != nil {
					t.Errorf("unexpected embedded font file, expected external font")
				}
			})
		}
	}
}
