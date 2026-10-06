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

// Rulers draws horizontal and vertical rulers in centimetres and inches,
// to check a viewer's "actual size" zoom against a physical ruler.
package main

import (
	"fmt"
	"os"
	"strconv"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/text"
)

const (
	inch = 72.0
	cm   = inch / 2.54
	mm   = cm / 10

	labelSize = 7.0
	margin    = 2.5 * cm
)

// tick is one division of a ruler: its position along the ruler, its
// length across it, and its label, if any.
type tick struct {
	pos, length float64
	label       string
}

// cmTicks returns the ticks of a centimetre ruler n cm long, with
// millimetre subdivisions.
func cmTicks(n int) []tick {
	var ticks []tick
	for i := 0; i <= 10*n; i++ {
		t := tick{pos: float64(i) * mm, length: 2 * mm}
		switch {
		case i%10 == 0:
			t.length = 5 * mm
			t.label = strconv.Itoa(i / 10)
		case i%5 == 0:
			t.length = 3.5 * mm
		}
		ticks = append(ticks, t)
	}
	return ticks
}

// inchTicks returns the ticks of an inch ruler n inches long, with
// subdivisions down to 1/16 inch.
func inchTicks(n int) []tick {
	var ticks []tick
	for i := 0; i <= 16*n; i++ {
		t := tick{pos: float64(i) * inch / 16, length: 2 * mm}
		switch {
		case i%16 == 0:
			t.length = 6.5 * mm
			t.label = strconv.Itoa(i / 16)
		case i%8 == 0:
			t.length = 5 * mm
		case i%4 == 0:
			t.length = 4 * mm
		case i%2 == 0:
			t.length = 3 * mm
		}
		ticks = append(ticks, t)
	}
	return ticks
}

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

	pageWidth := 21 * cm
	pageHeight := 29.7 * cm

	title := text.F{Font: titleFont, Size: 14, Color: color.DeviceGray(0)}
	body := text.F{Font: bodyFont, Size: 10, Color: color.DeviceGray(0.1)}
	text.Show(page.Builder,
		text.M{X: margin, Y: pageHeight - 2*cm},
		title, "Rulers for checking “actual size”",
	)
	text.Show(page.Builder,
		text.M{X: margin, Y: pageHeight - 2*cm - 20},
		body,
		text.Wrap(pageWidth-2*margin,
			"Set the viewer to actual size (100%) and hold a physical ruler",
			"against the rulers below, measuring from the 0 tick.",
			"Centimetre rulers have millimetre ticks; inch rulers have ticks",
			"every 1/16 inch.  Then move the window to a monitor with a",
			"different resolution: once the page is redrawn there, the rulers",
			"must still match.",
		),
	)

	label := text.F{Font: bodyFont, Size: labelSize, Color: color.DeviceGray(0)}

	page.SetLineWidth(0.25)
	page.SetStrokeColor(color.DeviceGray(0))

	// horizontal rulers, ticks pointing up, labels above
	drawHorizontal(page, label, margin, pageHeight-8*cm, 15, cmTicks(15), "cm")
	drawHorizontal(page, label, margin, pageHeight-10.5*cm, 6*2.54, inchTicks(6), "in")

	// vertical rulers, cm on the left and inches on the right
	bottom := 2 * cm
	drawVertical(page, label, margin, bottom, cmTicks(15), +1, "cm")
	drawVertical(page, label, pageWidth-margin, bottom, inchTicks(6), -1, "in")

	return page.Close()
}

// drawHorizontal draws a ruler starting at (x, y), of the given length
// in cm, with ticks pointing up.
func drawHorizontal(page *document.Page, f text.F, x, y, lengthCM float64, ticks []tick, unit string) {
	page.MoveTo(x, y)
	page.LineTo(pdf.Round(x+lengthCM*cm, 3), y)
	for _, t := range ticks {
		tx := pdf.Round(x+t.pos, 3)
		page.MoveTo(tx, y)
		page.LineTo(tx, pdf.Round(y+t.length, 3))
	}
	page.Stroke()

	for _, t := range ticks {
		if t.label == "" {
			continue
		}
		w := labelWidth(t.label)
		text.Show(page.Builder,
			text.M{X: pdf.Round(x+t.pos-w/2, 3), Y: pdf.Round(y+t.length+2, 3)},
			f, t.label,
		)
	}
	text.Show(page.Builder,
		text.M{X: pdf.Round(x+lengthCM*cm+6, 3), Y: y},
		f, unit,
	)
}

// drawVertical draws a ruler starting at (x, y) and going up.  The ticks
// point right for dir = +1 and left for dir = -1, and the labels sit
// beyond them.
func drawVertical(page *document.Page, f text.F, x, y float64, ticks []tick, dir float64, unit string) {
	last := ticks[len(ticks)-1].pos
	page.MoveTo(x, y)
	page.LineTo(x, pdf.Round(y+last, 3))
	for _, t := range ticks {
		ty := pdf.Round(y+t.pos, 3)
		page.MoveTo(x, ty)
		page.LineTo(pdf.Round(x+dir*t.length, 3), ty)
	}
	page.Stroke()

	for _, t := range ticks {
		if t.label == "" {
			continue
		}
		lx := x + dir*(t.length+2)
		if dir < 0 {
			lx -= labelWidth(t.label)
		}
		text.Show(page.Builder,
			text.M{X: pdf.Round(lx, 3), Y: pdf.Round(y+t.pos-0.35*labelSize, 3)},
			f, t.label,
		)
	}
	ux := x - labelWidth(unit)/2
	text.Show(page.Builder,
		text.M{X: pdf.Round(ux, 3), Y: pdf.Round(y+last+8, 3)},
		f, unit,
	)
}

// labelWidth approximates the width of a label in Helvetica, whose
// digits are all 0.556 em wide.
func labelWidth(s string) float64 {
	return float64(len(s)) * 0.556 * labelSize
}
