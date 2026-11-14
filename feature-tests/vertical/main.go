// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"fmt"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/cmap"
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
	F, err := opentype.NewComposite(raw, &opentype.OptionsComposite{CMap: cmap.IdentityV})
	if err != nil {
		return err
	}

	// The synthetic font gives letters above the x-height a different
	// advance height and vertical origin than the others, so that the font
	// dictionary needs both DW2 and W2.
	const size = 64
	var s pdf.String
	codec := F.Codec()
	gg := F.Layout(nil, size, "Hello")
	for _, g := range gg.Seq {
		code, ok := F.Encode(g.GID, g.Text)
		if !ok {
			return fmt.Errorf("cannot encode glyph %d %q", g.GID, g.Text)
		}
		s = codec.AppendCode(s, code)
	}

	page.TextBegin()
	page.TextSetFont(F, size)
	page.TextFirstLine(72, 520)
	page.TextShowRaw(s)
	page.TextEnd()

	// A red dot at the vertical origin of each glyph.  The dots step by the
	// vertical advance the font reports for the codes, which is what the
	// text operators above use.
	page.SetFillColor(color.Red)
	y := 520.0
	for c := range F.Codes(s) {
		page.Circle(72, y, 3)
		y += c.VerticalAdvance * size
	}
	page.Fill()

	return page.Close()
}
