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
	"math"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestTextLayout1 tests that no text content is lost when a glyph sequence
// is laid out.
func TestTextLayout1(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		t.Run(v.String(), func(t *testing.T) {
			data, _ := memfile.NewPDFWriter(v, nil)
			rm := pdf.NewResourceManager(data)

			F, err := gofont.Regular.New(nil)
			if err != nil {
				t.Fatal(err)
			}
			out := graphics.NewWriter(io.Discard, rm)
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
				gg := out.TextLayout(nil, s)
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
			data, _ := memfile.NewPDFWriter(v, nil)
			rm := pdf.NewResourceManager(data)

			F, err := gofont.Regular.New(nil)
			if err != nil {
				t.Fatal(err)
			}
			out := graphics.NewWriter(io.Discard, rm)
			out.TextSetFont(F, 10)

			// First make sure the font uses ligatures:
			gg := out.TextLayout(nil, "fi")
			if gg == nil {
				t.Fatal("typesetting failed")
			}
			if len(gg.Seq) != 1 {
				t.Fatal("test is broken")
			}

			// Then make sure that ligatures are disabled when character
			// spacing is non-zero:
			out.TextSetCharacterSpacing(1)
			gg = out.TextLayout(nil, "fi")
			if gg == nil {
				t.Fatal("layout failed")
			}
			if len(gg.Seq) != 2 {
				t.Error("ligatures not disabled")
			}
		})
	}
}

// TestTextLayout3 tests that the width of a glyph sequence scales
// with the font size.
func TestTextLayout3(t *testing.T) {
	data, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(data)

	F, err := gofont.Regular.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	out := graphics.NewWriter(io.Discard, rm)

	out.TextSetFont(F, 10)
	L1 := out.TextLayout(nil, "hello world!").TotalWidth()
	out.TextSetFont(F, 20)
	L2 := out.TextLayout(nil, "hello world!").TotalWidth()

	if L1 <= 0 {
		t.Fatalf("invalid width: %f", L1)
	}
	if math.Abs(L2/L1-2) > 0.05 {
		t.Errorf("unexpected width ratio: %f/%f=%f", L2, L1, L2/L1)
	}
}
