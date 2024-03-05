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

package graphics_test

import (
	"io"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/graphics"
)

// TestTextLayout1 tests that no text content is lost when a glyph sequence
// is laid out.
func TestTextLayout1(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		t.Run(v.String(), func(t *testing.T) {
			data := pdf.NewData(v)
			F, err := gofont.GoRegular.Embed(data, nil)
			if err != nil {
				t.Fatal(err)
			}
			out := graphics.NewWriter(io.Discard, v)
			out.TextSetFont(F, 10)

			var testCases = []string{
				"",
				" ",
				"ABC",
				"Hello World",
				"flower", // typeset as ligature
				"fish",   // typeset as ligature
				"ﬂower",  // ligature in source text
				"ﬁsh",    // ligature in source text
			}
			for _, s := range testCases {
				gg := out.TextLayout(s)
				if gg == nil {
					t.Fatal("typesetting failed")
				}
				if gg.Text() != s {
					t.Errorf("wrong text: %s != %s", gg.Text(), s)
				}
			}
		})
	}
}

// TestTextLayout2 tests that ligatures are disabled when character spacing is
// non-zero.
func TestTextLayout2(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		t.Run(v.String(), func(t *testing.T) {
			data := pdf.NewData(v)
			F, err := gofont.GoRegular.Embed(data, nil)
			if err != nil {
				t.Fatal(err)
			}
			out := graphics.NewWriter(io.Discard, v)
			out.TextSetFont(F, 10)

			// First make sure the font uses ligatures:
			gg := out.TextLayout("fi")
			if gg == nil {
				t.Fatal("typesetting failed")
			}
			if len(gg.Seq) != 1 {
				t.Fatal("test is broken")
			}

			// Then make sure that ligatures are disabled when character
			// spacing is non-zero:
			out.TextSetCharacterSpacing(1)
			gg = out.TextLayout("fi")
			if gg == nil {
				t.Fatal("layout failed")
			}
			if len(gg.Seq) != 2 {
				t.Error("ligatures not disabled")
			}
		})
	}
}
