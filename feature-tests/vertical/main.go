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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/makefont"
)

func main() {
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func createDocument(fname string) error {
	page, err := document.CreateSinglePage(fname, document.A5, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	raw := makefont.OpenType()
	opt := &opentype.OptionsComposite{
		WritingMode: font.Vertical,
	}
	F, err := opentype.NewComposite(raw, opt)
	if err != nil {
		return err
	}

	var s pdf.String
	codec := F.Codec()
	gg := F.Layout(nil, 10, "HELLO")
	for _, g := range gg.Seq {
		code, ok := F.Encode(g.GID, g.Text)
		if !ok {
			return fmt.Errorf("cannot encode glyph %d %q", g.GID, g.Text)
		}
		s = codec.AppendCode(s, code)
	}

	page.TextBegin()
	page.TextSetFont(F, 64)
	page.TextFirstLine(72, 520)
	page.TextShowRaw(s)
	page.TextEnd()

	page.SetFillColor(color.Red)
	page.Circle(72, 520, 5)
	page.Fill()

	return page.Close()
}
