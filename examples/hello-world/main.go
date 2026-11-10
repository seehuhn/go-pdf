// Copyright 2023 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"log"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/gofont"
)

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	doc, err := document.CreateSinglePage("hello.pdf", document.A4r, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	font, err := gofont.Regular.NewSimple(nil)
	if err != nil {
		return err
	}

	doc.TextSetFont(font, 50)
	doc.TextBegin()
	doc.TextFirstLine(50, 420)
	doc.TextShow("Hello, World!")
	doc.TextSecondLine(0, -60)
	doc.TextShow(string(gofont.Gopher))
	doc.TextEnd()

	return doc.Close()
}
