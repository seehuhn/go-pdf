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
	"bytes"
	"io"
	"math"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/reader"
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
				gg, err := out.TextLayout(s)
				if err != nil {
					t.Fatal(err)
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
			gg, err := out.TextLayout("fi")
			if err != nil {
				t.Fatal(err)
			}
			if len(gg.Seq) != 1 {
				t.Fatal("test is broken")
			}

			// Then make sure that ligatures are disabled when character
			// spacing is non-zero:
			out.TextSetCharacterSpacing(1)
			gg, err = out.TextLayout("fi")
			if err != nil {
				t.Fatal(err)
			}
			if len(gg.Seq) != 2 {
				t.Error("ligatures not disabled")
			}
		})
	}
}

func TestGlyphWidths(t *testing.T) {
	data := pdf.NewData(pdf.V1_7)
	F, err := type1.TimesRoman.Embed(data, nil)
	if err != nil {
		t.Fatal(err)
	}
	gg0 := F.Layout(50, "AB")
	if len(gg0.Seq) != 2 {
		t.Fatal("wrong number of glyphs")
	}

	buf := &bytes.Buffer{}
	out := graphics.NewWriter(buf, pdf.GetVersion(data))
	out.TextStart()
	out.TextSetHorizontalScaling(2)
	out.TextSetFont(F, 50)
	out.TextFirstLine(100, 100)
	gg := &font.GlyphSeq{
		Seq: []font.Glyph{
			{
				GID:     gg0.Seq[0].GID,
				Advance: 100,
				Text:    []rune("A"),
			},
			{
				GID:  gg0.Seq[1].GID,
				Text: []rune("B"),
			},
		},
	}
	out.TextShowGlyphs(gg)
	out.TextEnd()

	err = F.Close()
	if err != nil {
		t.Fatal(err)
	}

	in := reader.New(data, nil)
	var ggOut []font.Glyph
	var xxOut []float64
	in.DrawGlyph = func(g font.Glyph) error {
		ggOut = append(ggOut, g)
		x, _ := in.GetTextPositionDevice()
		xxOut = append(xxOut, x)
		return nil
	}
	in.NewPage()
	in.Resources = out.Resources
	err = in.ParseContentStream(buf)
	if err != nil {
		t.Fatal(err)
	}

	if len(xxOut) != 2 {
		t.Fatal("wrong number of glyphs")
	}
	if math.Abs(xxOut[0]-100) > 0.01 {
		t.Errorf("wrong glyph position: %f != 100", xxOut[0])
	}
	if math.Abs(xxOut[1]-200) > 0.01 {
		t.Errorf("wrong glyph position: %f != 200", xxOut[1])
	}
}
