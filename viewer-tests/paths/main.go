// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"log"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/graphics/color"
)

func main() {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	page, err := document.CreateSinglePage("test.pdf", document.A4, pdf.V1_7, opt)
	if err != nil {
		log.Fatal(err)
	}

	page.SetLineWidth(5)
	page.SetStrokeColor(color.DeviceGray(0.5))

	page.MoveTo(100, 100)
	page.LineTo(120, 200)
	page.LineTo(120, 300)
	page.LineTo(100, 400)
	page.ClosePath()
	page.LineTo(400, 250)
	page.Stroke()

	page.Rectangle(100, 500, 200, 200)
	page.LineTo(400, 600)
	page.Stroke()

	err = page.Close()
	if err != nil {
		log.Fatal(err)
	}
}
