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

// Command rectangle probes where a viewer puts the border of a square
// annotation.  The border can be drawn inside the annotation rectangle, or
// centred on it, and the /RD entry moves the square the border is drawn
// round.  Each annotation sits on a ruler of concentric squares, so that the
// answer can be read off the page.
//
// Only square annotations are used, with the same value in all four /RD
// entries, so that nothing depends on which entry belongs to which edge.
package main

import (
	"fmt"
	"math"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/fallback"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
)

const (
	// the two columns
	leftCenterX  = 174.0 // the figures line up with the readings, not the headings
	rightCenterX = 410.0

	// the ruler: squares concentric with the annotation
	ringMin  = 40.0
	ringMax  = 120.0
	ringStep = 5.0

	// the annotation under test
	annotSize = 100.0
	lineWidth = 10.0

	textStart = 100.0
	firstRowY = 736.0 // baseline of the first section's heading

	lineSkip = 11.0 // between lines of a section's text
	rowGap   = 26.0 // between one section and the next
)

// probe is one row of the page: a square annotation which differs from the
// others only in its /RD entry, together with the answers a viewer might
// give.  Each reading names the two ruler squares the border lies between.
type probe struct {
	rd       float64 // the value of each /RD entry, or 0 for no /RD at all
	dict     string
	readings []string
}

var probes = []probe{
	{
		rd:   0,
		dict: "no /RD",
		readings: []string{
			"80 to 100: the border is drawn inside Rect",
			"90 to 110: the border is centred on Rect",
		},
	},
	{
		rd:   10,
		dict: "/RD [10 10 10 10], half of which is the line width",
		readings: []string{
			"60 to 80: the border is drawn inside the square Rect minus RD",
			"70 to 90: the border is centred on the square Rect minus RD",
			"80 to 100: RD is ignored",
		},
	},
	{
		rd:   2.5,
		dict: "/RD [2.5 2.5 2.5 2.5], less than half the line width",
		readings: []string{
			"75 to 95: the border is drawn inside the square Rect minus RD",
			"85 to 105: the border is centred on it, and reaches outside Rect",
			"80 to 100: RD is ignored, or raised to half the line width",
		},
	},
}

func main() {
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func createDocument(filename string) error {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	page, err := document.CreateSinglePage(filename, document.A4, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	style, err := fallback.NewStyle().New(pdf.V1_7)
	if err != nil {
		return err
	}

	w := &writer{
		page:  page,
		style: style,
		bold:  font.Must(standard.HelveticaBold.New()),
		body:  font.Must(standard.Helvetica.New()),
	}

	w.text(textStart, 800, w.bold, 13,
		"Square annotations: where is the border drawn?")
	w.text(textStart, 784, w.body, 9,
		"Left: the annotation has no appearance stream, so your viewer draws it.")
	w.text(textStart, 773, w.body, 9,
		"Right: the same annotation, with Quire's appearance stream.")
	w.text(textStart, 762, w.body, 9,
		"Each annotation: Rect 100 by 100, /BS with /W 10, a blue border, no interior colour.")

	w.rowY = firstRowY
	for i, p := range probes {
		err = w.addRow(i+1, p)
		if err != nil {
			return err
		}
	}

	// what the readings mean, for whoever fills the answers in
	w.text(textStart, 162, w.bold, 9, "What the PDF spec says")
	w.text(textStart, 150, w.body, 9,
		"Clause 12.5.4: \"If present, the border shall be drawn completely inside the annotation rectangle.\"")
	w.text(textStart, 139, w.body, 9,
		"Clause 12.5.6.8: the square \"shall be inscribed within the annotation rectangle\".")
	w.text(textStart, 128, w.body, 9,
		"RD (PDF 1.5) gives the difference between Rect and \"the actual boundaries of the underlying square\".")

	return page.Close()
}

type writer struct {
	page  *document.Page
	style *fallback.Generator
	bold  font.Layouter
	body  font.Layouter
	rowY  float64
}

// addRow writes the section's heading and readings, then draws the ruler and
// the two annotations below them.
func (w *writer) addRow(num int, p probe) error {
	y := w.rowY
	w.text(textStart, y, w.bold, 9, fmt.Sprintf("%d.  %s", num, p.dict))
	for _, reading := range p.readings {
		y -= lineSkip
		w.text(textStart+14, y, w.body, 9, reading)
	}

	// the first section's figures carry the column headings
	y -= 12
	if num == 1 {
		y -= 14
		w.text(leftCenterX-ringMax/2, y+4, w.bold, 10, "your viewer")
		w.text(rightCenterX-ringMax/2, y+4, w.bold, 10, "Quire")
	}

	cy := y - ringMax/2
	w.drawRuler(leftCenterX, cy)
	w.drawRuler(rightCenterX, cy)

	left := newSquare(leftCenterX, cy, p)
	left.Contents += " (viewer)"

	right := newSquare(rightCenterX, cy, p)
	right.Contents += " (Quire)"
	err := w.style.AddAppearance(right)
	if err != nil {
		return err
	}

	w.page.Page.AddAnnots(left, right)

	w.rowY = cy - ringMax/2 - rowGap
	return nil
}

// drawRuler draws the concentric squares the border is measured against.
func (w *writer) drawRuler(cx, cy float64) {
	for s := ringMin; s <= ringMax+ringStep/2; s += ringStep {
		labelled := math.Mod(s, 20) == 0
		if labelled {
			w.page.SetStrokeColor(color.DeviceGray(0.45))
			w.page.SetLineWidth(0.5)
		} else {
			w.page.SetStrokeColor(color.DeviceGray(0.75))
			w.page.SetLineWidth(0.3)
		}
		w.page.Rectangle(cx-s/2, cy-s/2, s, s)
		w.page.Stroke()

		// the labels sit clear of the outermost square, each level with the
		// top edge of the square it names
		if labelled {
			w.text(cx+ringMax/2+5, cy+s/2-2.5, w.body, 6, fmt.Sprintf("%g", s))
		}
	}
}

func (w *writer) text(x, y float64, f font.Layouter, size float64, s string) {
	w.page.TextBegin()
	w.page.TextSetFont(f, size)
	w.page.TextSetMatrix(matrix.Translate(x, y))
	w.page.TextShow(s)
	w.page.TextEnd()
}

func newSquare(cx, cy float64, p probe) *annotation.Square {
	a := &annotation.Square{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: cx - annotSize/2,
				LLy: cy - annotSize/2,
				URx: cx + annotSize/2,
				URy: cy + annotSize/2,
			},
			Contents: p.dict,
			Flags:    annotation.FlagPrint,
			Color:    color.Blue,
		},
		BorderStyle: &annotation.BorderStyle{
			Width:     lineWidth,
			Style:     "S",
			SingleUse: true,
		},
	}
	if p.rd > 0 {
		a.Margin = []float64{p.rd, p.rd, p.rd, p.rd}
	}
	return a
}
