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

package main

import (
	"fmt"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/text"
)

func main() {
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func createDocument(filename string) error {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	w, err := document.CreateSinglePage(filename, document.A5, pdf.V2_0, opt)
	if err != nil {
		return err
	}

	F := standard.TimesRoman.New()
	note := text.F{
		Font:  F,
		Size:  10,
		Color: color.DeviceGray(0),
	}

	blue := color.DeviceRGB(0, 0, 0.9)
	var testFont [4]text.F
	var testString [4]pdf.String

	for i, unitsPerEm := range []float64{1000, 2000} {
		T := makeTestFont(unitsPerEm, false)
		_, E, err := pdf.ResourceManagerEmbed(w.RM, T)
		if err != nil {
			return err
		}
		testFont[i] = text.F{
			Font:  T,
			Size:  12,
			Color: blue,
		}
		testString[i], _ = E.(font.EmbeddedLayouter).AppendEncoded(testString[i], 1, "A")
		testString[i], _ = E.(font.EmbeddedLayouter).AppendEncoded(testString[i], 1, "A")
		testString[i], _ = E.(font.EmbeddedLayouter).AppendEncoded(testString[i], 1, "A")
	}
	for i, unitsPerEm := range []float64{1000, 2000} {
		T := makeTestFont(unitsPerEm, true)
		_, E, err := pdf.ResourceManagerEmbed(w.RM, T)
		if err != nil {
			return err
		}
		testFont[i+2] = text.F{
			Font:  T,
			Size:  12,
			Color: blue,
		}
		testString[i+2], _ = E.(font.EmbeddedLayouter).AppendEncoded(testString[i+2], 1, "A")
		testString[i+2], _ = E.(font.EmbeddedLayouter).AppendEncoded(testString[i+2], 1, "A")
		testString[i+2], _ = E.(font.EmbeddedLayouter).AppendEncoded(testString[i+2], 1, "A")
	}

	text.Show(w.Writer,
		text.M{X: 36, Y: 550},
		note,
		text.Wrap(340,
			`PDF Type 3 fonts handle glyph space units differently from other
			font types. While most fonts define 1000 glyph space units as
			1 text space unit, Type 3 fonts use their font matrix to convert
			between glyph and text space. This test verifies that viewers
			implement this conversion correctly.`),
		text.NL,
		text.Wrap(340,
			"The following two lines should should show three squares each,",
			"followed by an X, and should look the same:"),
		text.M{X: 0, Y: -10},
		testFont[0], testString[0], note, "X", text.NL,
		text.M{X: 0, Y: -10},
		testFont[1], testString[1], note, "X", text.NL,
		text.Wrap(340,
			"The following two lines should should show three rotated squares,",
			"followed by an X, and should look the same:"),
		text.M{X: 0, Y: -10},
		testFont[2], testString[2], note, "X", text.NL,
		text.M{X: 0, Y: -10},
		testFont[3], testString[3], note, "X", text.NL,
	)

	err = w.Close()
	if err != nil {
		return err
	}

	return nil
}

func makeTestFont(unitsPerEm float64, rotate bool) font.Font {
	q := 1 / unitsPerEm

	fontMatrix := matrix.Matrix{q, 0, 0, q, 0, 0}
	if rotate {
		fontMatrix = fontMatrix.Mul(matrix.RotateDeg(30))
	}

	F := &type3.Font{
		Glyphs: []*type3.Glyph{
			{},
		},
		FontMatrix: fontMatrix,
		Ascent:     unitsPerEm,
		Leading:    unitsPerEm * 1.2,
	}
	F.Glyphs = append(F.Glyphs, &type3.Glyph{
		Name:  "A",
		Width: unitsPerEm,
		BBox:  rect.Rect{URx: unitsPerEm, URy: unitsPerEm},
		Draw: func(w *graphics.Writer) error {
			a := 0.05 * unitsPerEm
			b := 0.95 * unitsPerEm
			w.Rectangle(a, a, b, b)
			w.Fill()
			return nil
		},
	})
	res, err := type3.New(F)
	if err != nil {
		panic(err)
	}
	return res
}
