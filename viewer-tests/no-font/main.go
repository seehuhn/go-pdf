// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/text"
)

const (
	margin      = 72.0
	titleY      = 800.0
	cellTopY    = 620.0
	cellBottomY = 500.0
	cellWidth   = 220.0
	cellGap     = 11.0
	cellPadX    = 18.0
	cellPadY    = 38.0 // text origin sits this far above cellBottomY
	crosshair   = 4.0
	tickLength  = 60.0
	footerY     = 460.0
	wrapWidth   = 451.0 // page width − 2*margin

	controlText = "Hello!"
)

func main() {
	if err := createDocument("test.pdf"); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func createDocument(filename string) error {
	page, err := document.CreateSinglePage(filename, document.A4, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	// Inject the malformed test stream first.  Tf is part of the
	// graphics state and persists across BT/ET, so once any text.Show
	// or page.TextSetFont call has run, every subsequent BT/Tj
	// inherits a non-empty font — including the test cell's Tj.  The
	// only way to keep the test meaningful is to emit it before the
	// first Tf appears anywhere on the page.
	injectTestText(page)

	titleFont := font.Must(standard.TimesBold.New())
	bodyFont := font.Must(standard.TimesRoman.New())

	title := text.F{Font: titleFont, Size: 14, Color: color.DeviceGray(0)}
	body := text.F{Font: bodyFont, Size: 10, Color: color.DeviceGray(0.1)}

	text.Show(page.Builder,
		text.M{X: margin, Y: titleY},
		title, "Text Without a Font", text.NL,
		text.NL,
		body,
		text.Wrap(wrapWidth,
			"PDF 32000-2:2020 §9.3.1 requires the font and font size to be set",
			"with a Tf operator before any text-showing operator (Tj, TJ, ', \").",
			"The PDF specification does not say what a conforming reader must do",
			"if Tf is absent.  This document tests how different viewers handle",
			"that case.  The test cell on the left contains a Tj with no Tf",
			"anywhere in the content stream.  The control cell on the right",
			"shows the same string with Tf set to Times-Roman 24pt.",
		),
	)

	cellLabel := text.F{Font: bodyFont, Size: 9, Color: color.DeviceGray(0.4)}
	drawCell(page, cellLabel, margin, "TEST: no Tf")
	drawCell(page, cellLabel, margin+cellWidth+cellGap, "CONTROL: Times-Roman 24")

	controlFont := font.Must(standard.TimesRoman.New())
	drawControlText(page, controlFont)

	return page.Close()
}

// injectTestText appends a BT / Tm / Tj (Hello!) / ET sequence to the
// content stream as a single OpRawContent pseudo-operator.  Wrapping
// the malformed sequence in OpRawContent lets it pass the page-level
// content-stream validator (which would otherwise reject Tj-without-Tf
// when validating typed Operators), while still emitting the raw bytes
// verbatim into the file.  No Tf is emitted, so the Tj runs with an
// unset font in the graphics state — this is the malformed input under
// test.  Call this before any code path that emits a Tf, since Tf
// persists across BT/ET.
func injectTestText(page *document.Page) {
	originX := margin + cellPadX
	originY := cellBottomY + cellPadY

	raw := fmt.Sprintf("BT\n1 0 0 1 %g %g Tm\n(%s) Tj\nET",
		originX, originY, controlText)

	page.Builder.Stream = append(page.Builder.Stream,
		content.Operator{
			Name: content.OpRawContent,
			Args: []pdf.Object{pdf.String(raw)},
		},
	)
}

// drawControlText renders the control string at the control cell's
// text origin, using a valid Tf for Times-Roman 24pt.
func drawControlText(page *document.Page, F font.Instance) {
	originX := margin + cellWidth + cellGap + cellPadX
	originY := cellBottomY + cellPadY

	page.SetFillColor(color.DeviceGray(0))
	page.TextBegin()
	page.TextSetFont(F, 24)
	page.TextSetMatrix(matrix.Translate(originX, originY))
	page.TextShow(controlText)
	page.TextEnd()
}

// drawCell draws a labelled cell rectangle with a crosshair at the text
// origin and a horizontal baseline tick.  The text origin is at
// (cellX+cellPadX, cellBottomY+cellPadY); the caption is drawn just
// above the top edge using the labelStyle text style.
func drawCell(page *document.Page, labelStyle text.F, cellX float64, caption string) {
	originX := cellX + cellPadX
	originY := cellBottomY + cellPadY

	// labelled rectangle
	page.SetLineWidth(0.5)
	page.SetStrokeColor(color.DeviceGray(0.7))
	page.Rectangle(cellX, cellBottomY, cellWidth, cellTopY-cellBottomY)
	page.Stroke()

	// crosshair at text origin
	page.SetLineWidth(0.4)
	page.SetStrokeColor(color.DeviceGray(0.3))
	page.MoveTo(originX-crosshair, originY)
	page.LineTo(originX+crosshair, originY)
	page.MoveTo(originX, originY-crosshair)
	page.LineTo(originX, originY+crosshair)
	page.Stroke()

	// baseline tick going right from origin
	page.SetLineWidth(0.3)
	page.SetStrokeColor(color.DeviceGray(0.5))
	page.MoveTo(originX, originY)
	page.LineTo(originX+tickLength, originY)
	page.Stroke()

	// caption above the cell
	text.Show(page.Builder,
		text.M{X: cellX, Y: cellTopY + 4},
		labelStyle, caption,
	)
}
