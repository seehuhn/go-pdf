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
		Color: color.Black,
	}

	blue := color.DeviceRGB(0, 0, 0.9)
	var testFont [4]text.F
	var testString [4]pdf.String

	for i, unitsPerEm := range []float64{1000, 2000} {
		F := makeTestFont(unitsPerEm, false)
		testFont[i] = text.F{
			Font:  F,
			Size:  12,
			Color: blue,
		}

		gg := F.Layout(nil, 1, "AAA")
		for _, g := range gg.Seq {
			code, ok := F.Encode(g.GID, g.Text)
			if !ok {
				return fmt.Errorf("cannot encode glyph %v", g)
			}
			codec := F.Codec()
			testString[i] = codec.AppendCode(testString[i], code)
		}
	}
	for i, unitsPerEm := range []float64{1000, 2000} {
		F := makeTestFont(unitsPerEm, true)
		testFont[i+2] = text.F{
			Font:  F,
			Size:  12,
			Color: blue,
		}

		gg := F.Layout(nil, 1, "AAA")
		for _, g := range gg.Seq {
			code, ok := F.Encode(g.GID, g.Text)
			if !ok {
				return fmt.Errorf("cannot encode glyph %v", g)
			}
			codec := F.Codec()
			testString[i+2] = codec.AppendCode(testString[i+2], code)
		}
	}

	// Write text content
	w.TextBegin()
	w.TextSetFont(note.Font, note.Size)
	w.SetFillColor(note.Color)
	w.TextFirstLine(36, 550)

	w.TextShow("PDF Type 3 fonts handle glyph space units differently from other ")
	w.TextSecondLine(0, -note.Size*1.2)
	w.TextShow("font types. While most fonts define 1000 glyph space units as ")
	w.TextNextLine()
	w.TextShow("1 text space unit, Type 3 fonts use their font matrix to convert ")
	w.TextNextLine()
	w.TextShow("between glyph and text space. This test verifies that viewers ")
	w.TextNextLine()
	w.TextShow("implement this conversion correctly.")
	w.TextNextLine()
	w.TextNextLine()
	w.TextShow("The following two lines should should show three squares each, ")
	w.TextNextLine()
	w.TextShow("followed by an X, and should look the same:")
	w.TextSecondLine(0, -10)
	w.TextSetFont(testFont[0].Font, testFont[0].Size)
	w.SetFillColor(testFont[0].Color)
	w.TextShowRaw(testString[0])
	w.TextSetFont(note.Font, note.Size)
	w.SetFillColor(note.Color)
	w.TextShow("X")
	w.TextSecondLine(0, -10)
	w.TextSetFont(testFont[1].Font, testFont[1].Size)
	w.SetFillColor(testFont[1].Color)
	w.TextShowRaw(testString[1])
	w.TextSetFont(note.Font, note.Size)
	w.SetFillColor(note.Color)
	w.TextShow("X")
	w.TextSecondLine(0, -10)
	w.TextShow("The following two lines should should show three rotated squares, ")
	w.TextNextLine()
	w.TextShow("followed by an X, and should look the same:")
	w.TextSecondLine(0, -10)
	w.TextSetFont(testFont[2].Font, testFont[2].Size)
	w.SetFillColor(testFont[2].Color)
	w.TextShowRaw(testString[2])
	w.TextSetFont(note.Font, note.Size)
	w.SetFillColor(note.Color)
	w.TextShow("X")
	w.TextSecondLine(0, -10)
	w.TextSetFont(testFont[3].Font, testFont[3].Size)
	w.SetFillColor(testFont[3].Color)
	w.TextShowRaw(testString[3])
	w.TextSetFont(note.Font, note.Size)
	w.SetFillColor(note.Color)
	w.TextShow("X")
	w.TextEnd()

	err = w.Close()
	if err != nil {
		return err
	}

	return nil
}

func makeTestFont(unitsPerEm float64, rotate bool) font.Layouter {
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
	builder := graphics.NewContentStreamBuilder()
	a := 0.05 * unitsPerEm
	b := 0.90 * unitsPerEm
	builder.Rectangle(a, a, b, b)
	builder.Fill()

	F.Glyphs = append(F.Glyphs, &type3.Glyph{
		Name:    "A",
		Width:   unitsPerEm,
		BBox:    rect.Rect{URx: unitsPerEm, URy: unitsPerEm},
		Content: builder.Build(),
	})
	res, err := F.New()
	if err != nil {
		panic(err)
	}
	return res
}
