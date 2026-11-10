// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/gofont"
)

// page geometry, in PDF points (US Letter)
const (
	pageWidth  = 612
	pageHeight = 792
	margin     = 72
)

// The text on the source page was written for this test data.
const (
	pageTitle = "Notes on Bi-Level Image Coding"

	pageIntro = "A bi-level image stores one bit for every pixel: a set bit " +
		"is ink and a clear bit is paper. Pages of printed text are the most " +
		"common images of this kind, and they compress well because most of " +
		"the page is empty, most rows look much like the row above, and the " +
		"same letter shapes come back again and again."
)

var pageSections = []struct {
	heading string
	body    []string
}{
	{
		"Context modelling",
		[]string{
			"A context-based coder predicts each pixel from a small template " +
				"of neighbours which have already been coded. The neighbours " +
				"form a number, and the number selects one of many adaptive " +
				"probability estimates. Where the estimate is good, the pixel " +
				"costs a small fraction of a bit; where it is poor, the " +
				"estimate is corrected and the next guess improves.",
			"Long stretches of white margin cost almost nothing, while the " +
				"edges of strokes carry most of the information. A row which " +
				"repeats the row above can be flagged as a copy, so that the " +
				"blank space between lines of text is coded with one decision " +
				"per row instead of one decision per pixel.",
		},
	},
	{
		"Run lengths and two-dimensional codes",
		[]string{
			"An older family of coders describes each row as alternating " +
				"runs of white and black pixels. The two-dimensional variant " +
				"codes the position of each colour change relative to a nearby " +
				"change in the reference row above, and falls back to explicit " +
				"run lengths only where the rows disagree. Fixed code tables " +
				"keep the decoder simple and fast.",
		},
	},
	{
		"Symbols and text regions",
		[]string{
			"When a page holds many copies of the same shapes, an encoder can " +
				"collect the distinct shapes into a dictionary and then describe " +
				"the page as a list of placements: which shape goes where. Each " +
				"placement needs only a shape index and a small offset from the " +
				"previous one. Shapes which are almost the same can be stored " +
				"once and refined on use.",
			"Choosing which shapes count as the same is the hard part. A strict " +
				"rule gives a large dictionary and a faithful page; a loose rule " +
				"gives a small file in which a few letters may have changed.",
		},
	},
}

var pageTable = [][3]string{
	{"Method", "Unit of coding", "Typical use"},
	{"Generic region", "single pixels", "line art, scans"},
	{"Run-length codes", "runs of pixels", "fax, legacy data"},
	{"Text region", "letter shapes", "printed pages"},
	{"Halftone region", "dot patterns", "photographs"},
}

var pageCode = []string{
	"for y := 0; y < height; y++ {",
	"    for x := 0; x < width; x++ {",
	"        ctx := context(img, x, y)",
	"        bit := decode(ctx)",
	"        img.Set(x, y, bit)",
	"    }",
	"}",
}

const pageClosing = "The figures above are not measurements; they describe " +
	"which kind of content each method is designed for. Real files often " +
	"combine several methods on one page, using each where it fits best, " +
	"and the decoder assembles the regions into the final page bitmap."

// writeSourcePDF writes a one-page PDF file containing the text above,
// set in the Go fonts.
func writeSourcePDF(fileName string) error {
	paper := &pdf.Rectangle{URx: pageWidth, URy: pageHeight}
	doc, err := document.CreateSinglePage(fileName, paper, pdf.V1_7, nil)
	if err != nil {
		return err
	}

	regular, err := gofont.Regular.NewSimple(nil)
	if err != nil {
		return err
	}
	bold, err := gofont.Bold.NewSimple(nil)
	if err != nil {
		return err
	}
	italic, err := gofont.Italic.NewSimple(nil)
	if err != nil {
		return err
	}
	mono, err := gofont.Mono.NewSimple(nil)
	if err != nil {
		return err
	}

	const (
		bodySize    = 10.0
		bodyLeading = 13.0
		textWidth   = pageWidth - 2*margin
	)
	y := float64(pageHeight - margin)

	line := func(F font.Layouter, size float64, x float64, s string) {
		doc.TextBegin()
		doc.TextSetFont(F, size)
		doc.TextFirstLine(x, y)
		doc.TextShow(s)
		doc.TextEnd()
	}
	paragraph := func(F font.Layouter, s string) {
		for _, l := range wrap(F, bodySize, s, textWidth) {
			line(F, bodySize, margin, l)
			y -= bodyLeading
		}
		y -= 0.5 * bodyLeading
	}
	rule := func(x0, x1, yy, w float64) {
		doc.SetLineWidth(w)
		doc.MoveTo(x0, yy)
		doc.LineTo(x1, yy)
		doc.Stroke()
	}

	y -= 18
	line(bold, 20, margin, pageTitle)
	y -= 10
	rule(margin, pageWidth-margin, y, 1)
	y -= 24

	paragraph(italic, pageIntro)
	for _, sec := range pageSections {
		y -= 4
		line(bold, 13, margin, sec.heading)
		y -= 18
		for _, p := range sec.body {
			paragraph(regular, p)
		}
	}

	// a small ruled table
	y -= 4
	colX := []float64{margin + 6, margin + 150, margin + 300}
	top := y + bodyLeading - 3
	rule(margin, pageWidth-margin, top, 0.8)
	for i, row := range pageTable {
		F := regular
		if i == 0 {
			F = bold
		}
		for j, cell := range row {
			line(F, bodySize, colX[j], cell)
		}
		y -= bodyLeading + 2
		if i == 0 {
			rule(margin, pageWidth-margin, y+bodyLeading-2, 0.5)
		}
	}
	rule(margin, pageWidth-margin, y+bodyLeading-2, 0.8)
	y -= bodyLeading

	// a code listing
	for _, l := range pageCode {
		line(mono, 10, margin+18, l)
		y -= 13
	}
	y -= bodyLeading

	paragraph(regular, pageClosing)

	// page number
	y = margin / 2
	line(regular, 10, pageWidth/2-3, "1")

	return doc.Close()
}

// wrap breaks s into lines no wider than width.
func wrap(F font.Layouter, size float64, s string, width float64) []string {
	var lines []string
	var cur string
	for w := range strings.FieldsSeq(s) {
		next := w
		if cur != "" {
			next = cur + " " + w
		}
		if cur != "" && F.Layout(nil, size, next).TotalWidth() > width {
			lines = append(lines, cur)
			next = w
		}
		cur = next
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}
