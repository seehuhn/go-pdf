// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/sfnt/funit"
)

func embedType3Font(out pdf.Putter) (font.Embedded, error) {
	b := type3.New(1000)
	b.Ascent = 800
	b.Descent = -200
	b.BaseLineSkip = 1000

	A, err := b.AddGlyph("A", 1000, funit.Rect16{LLx: 0, LLy: 0, URx: 800, URy: 800}, true)
	if err != nil {
		return nil, err
	}
	A.MoveTo(0, 0)
	A.LineTo(800, 0)
	A.LineTo(800, 800)
	A.LineTo(0, 800)
	A.Fill()
	err = A.Close()
	if err != nil {
		return nil, err
	}

	B, err := b.AddGlyph("B", 900, funit.Rect16{LLx: 0, LLy: 0, URx: 800, URy: 800}, true)
	if err != nil {
		return nil, err
	}
	B.Circle(400, 400, 400)
	B.Fill()
	err = B.Close()
	if err != nil {
		return nil, err
	}

	C, err := b.AddGlyph("C", 1000, funit.Rect16{LLx: 0, LLy: 0, URx: 800, URy: 800}, true)
	if err != nil {
		return nil, err
	}
	C.MoveTo(0, 0)
	C.LineTo(800, 0)
	C.LineTo(400, 800)
	C.Fill()
	err = C.Close()
	if err != nil {
		return nil, err
	}

	return b.EmbedFont(out, "X")
}
