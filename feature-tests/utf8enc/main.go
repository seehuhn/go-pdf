// Copyright 2023 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"fmt"
	"os"

	"golang.org/x/text/language"

	"seehuhn.de/go/pdf"

	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/cmap"
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
	page, err := document.CreateSinglePage(fname, document.A4, pdf.V1_7, nil)
	if err != nil {
		return err
	}

	cffFont := makefont.OpenType()
	fontOpt := &cff.OptionsComposite{
		Language: language.German,
		CMap:     cmap.UTF8H,
	}
	F1, err := cff.NewComposite(cffFont, fontOpt)
	if err != nil {
		return err
	}
	F2, err := cff.NewSimple(cffFont, nil)
	if err != nil {
		return err
	}

	page.TextBegin()
	page.TextFirstLine(100, 700)
	page.TextSetFont(F1, 36)
	page.TextShow("„Größenwahn“")
	page.TextSecondLine(0, -40)
	page.TextSetFont(F2, 36)
	page.TextShow("„Größenwahn“")
	page.TextEnd()

	return page.Close()
}
