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
	"math"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/reader"
)

func TestGlyphWidths(t *testing.T) {
	data := pdf.NewData(pdf.V1_7)
	F, err := type1.TimesRoman.Embed(data, "")
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
	out.TextSetHorizontalScaling(200)
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
