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
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/text"
)

// page geometry
const (
	margin    = 72.0
	pageWidth = 595.0 // A4
	wrapWidth = pageWidth - 2*margin
)

// grid geometry
//
// dx is both the horizontal distance between the No and Yes column
// boxes and the translation in the cm operator under test.  Drawing
// a tick at the "No" coordinates with the cm applied therefore lands
// inside the Yes box.
const (
	boxSize = 24.0
	dx      = 60.0

	xLabel  = margin
	xNoCol  = 290.0
	xYesCol = xNoCol + dx
	yQ1     = 620.0
	yQ2     = 560.0

	// glyph offset inside the Q1 box (Helvetica-Bold X at 18pt is
	// roughly 12pt wide and 13pt tall, so an origin at +6,+6 sits the
	// glyph near the box centre)
	q1GlyphSize = 18.0
	q1GlyphDX   = 6.0
	q1GlyphDY   = 6.0

	// padding for the Q2 stroked-X within its box
	q2Pad = 4.0
)

// vertical layout
const (
	titleY = 790.0
	introY = 745.0
	hdrY   = yQ1 + boxSize + 6
)

func main() {
	if err := createDocument("test.pdf"); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func createDocument(filename string) error {
	page, err := document.CreateSinglePage(filename, document.A4, pdf.V1_7, nil)
	if err != nil {
		return err
	}

	titleFont := font.Must(standard.HelveticaBold.New())
	bodyFont := font.Must(standard.Helvetica.New())
	tickFont := bodyFont

	drawTitleAndIntro(page, titleFont, bodyFont)
	drawGrid(page)
	drawLabels(page, bodyFont)

	// Pre-register the tick font with the builder so the raw content
	// stream below can reference it by its assigned resource name.
	tickFontName := page.Builder.FontName(tickFont)
	injectProbe(page, tickFontName)

	return page.Close()
}

func drawTitleAndIntro(page *document.Page, titleFont, bodyFont font.Instance) {
	title := text.F{Font: titleFont, Size: 14, Color: color.DeviceGray(0)}
	body := text.F{Font: bodyFont, Size: 10, Color: color.DeviceGray(0.1)}

	text.Show(page.Builder,
		text.M{X: margin, Y: titleY},
		title, "cm inside BT / ET — viewer behaviour probe",
	)
	text.Show(page.Builder,
		text.M{X: margin, Y: introY},
		body,
		text.Wrap(wrapWidth,
			"This page tests how a PDF viewer handles a cm operator placed",
			"inside a BT ... ET text object.  Such streams are malformed",
			"(ISO 32000-2:2020 §9.4.1 admits only general-graphics-state,",
			"colour, marked-content, and text operators inside a text object,",
			"and cm is in the special-graphics-state category), but §8.2",
			"NOTE 2 permits readers to render them anyway.",
		),
	)
}

func drawGrid(page *document.Page) {
	page.SetLineWidth(0.5)
	page.SetStrokeColor(color.DeviceGray(0.4))
	page.Rectangle(xNoCol, yQ1, boxSize, boxSize)
	page.Rectangle(xYesCol, yQ1, boxSize, boxSize)
	page.Rectangle(xNoCol, yQ2, boxSize, boxSize)
	page.Rectangle(xYesCol, yQ2, boxSize, boxSize)
	page.Stroke()
}

func drawLabels(page *document.Page, bodyFont font.Instance) {
	rowLabel := text.F{Font: bodyFont, Size: 10, Color: color.DeviceGray(0.1)}
	colLabel := text.F{Font: bodyFont, Size: 9, Color: color.DeviceGray(0.4)}

	// column headers (centred over each box; offsets tuned by eye for
	// 9pt Helvetica)
	text.Show(page.Builder,
		text.M{X: xNoCol + boxSize/2 - 4.5, Y: hdrY},
		colLabel, "No",
	)
	text.Show(page.Builder,
		text.M{X: xYesCol + boxSize/2 - 6, Y: hdrY},
		colLabel, "Yes",
	)

	// row labels, left-aligned, vertically centred against the boxes
	text.Show(page.Builder,
		text.M{X: xLabel, Y: yQ1 + boxSize/2 - 3},
		rowLabel, "was the cm applied inside the BT / ET?",
	)
	text.Show(page.Builder,
		text.M{X: xLabel, Y: yQ2 + boxSize/2 - 3},
		rowLabel, "did the cm effect persist past the ET?",
	)
}

// injectProbe appends the malformed drive sequence as raw content
// stream bytes.  A single BT / ET emits both ticks:
//
//   - Q1 tick: glyph X drawn by Tj after the cm.  Its device position
//     is Tm × CTM, and the cm shifts CTM by +dx.  If the viewer applies
//     the cm, the glyph lands in the Q1-Yes box; otherwise in Q1-No.
//
//   - Q2 tick: two stroked diagonals forming an X, drawn at constant
//     user-space coordinates pointing into the Q2-No box, immediately
//     after the ET.  If the cm leaks past the ET, those coordinates
//     are rendered at device coordinates shifted by +dx, landing in
//     Q2-Yes; otherwise in Q2-No.
//
// The whole sequence is wrapped in q ... Q so any leaked CTM change
// is rolled back before the legend is drawn.
func injectProbe(page *document.Page, fontName pdf.Name) {
	q1OriginX := xNoCol + q1GlyphDX
	q1OriginY := yQ1 + q1GlyphDY

	q2L := xNoCol + q2Pad
	q2R := xNoCol + boxSize - q2Pad
	q2B := yQ2 + q2Pad
	q2T := yQ2 + boxSize - q2Pad

	var b strings.Builder
	fmt.Fprintln(&b, "q")
	fmt.Fprintln(&b, "0.8 w")
	fmt.Fprintln(&b, "0 G")
	fmt.Fprintln(&b, "BT")
	fmt.Fprintf(&b, "/%s %g Tf\n", fontName, q1GlyphSize)
	fmt.Fprintf(&b, "1 0 0 1 %g %g Tm\n", q1OriginX, q1OriginY)
	fmt.Fprintf(&b, "1 0 0 1 %g 0 cm\n", dx)
	fmt.Fprintln(&b, "(X) Tj")
	fmt.Fprintln(&b, "ET")
	fmt.Fprintln(&b, "2 w")
	fmt.Fprintf(&b, "%g %g m\n", q2L, q2B)
	fmt.Fprintf(&b, "%g %g l\n", q2R, q2T)
	fmt.Fprintf(&b, "%g %g m\n", q2L, q2T)
	fmt.Fprintf(&b, "%g %g l\n", q2R, q2B)
	fmt.Fprintln(&b, "S")
	fmt.Fprintln(&b, "Q")

	page.Builder.Stream = append(page.Builder.Stream, content.Operator{
		Name: content.OpRawContent,
		Args: []pdf.Object{pdf.String(b.String())},
	})
}
