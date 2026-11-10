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
