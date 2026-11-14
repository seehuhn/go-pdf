// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package notdefgrid draws the rows of circles and crosses which the
// viewer tests for missing glyphs in composite fonts use to show which
// glyph a viewer selects and which width it applies.
//
// Each row is drawn in a test font at size 30 and shows the string
// "cDDDDDA", where c is the code under test.  The test font must contain
// glyphs made with [CrossGlyph] for CID 0, the notdef CID and the tested
// glyph, at positions 250, 750 and 1250 and with widths [WidthCID0],
// [WidthNotdef] and [WidthGlyph].  The code 'D' must select a blank glyph
// of width 500, and the code 'A' must select CID 0.  The first cross then
// falls into one of the three circles on the left, depending on the glyph
// shown, and the last cross falls into one of the five circles on the
// right, depending on the width used for the code under test.
package notdefgrid

import (
	"math"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/sfnt/cff"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/text"
)

// FontSize is the size at which the test font must be shown.
const FontSize = 30

// The widths which mark the right-hand circles, in PDF glyph space units.
// A width of 0 marks the first circle.
const (
	WidthCID0    = 500
	WidthNotdef  = 1000
	WidthGlyph   = 1500
	WidthDefault = 2000
)

// The glyph choices for the gIdx argument of [Row].
const (
	ShowsCID0 = iota
	ShowsNotdef
	ShowsGlyph
)

// The width choices for the wIdx argument of [Row].
const (
	UsesCID0Width = iota
	UsesNotdefWidth
	UsesGlyphWidth
)

// Unspecified can be used for the gIdx and wIdx arguments of [Row] when
// there is no expected result.
const Unspecified = -1

// RowHeight is the vertical distance between two rows.
const RowHeight = 13

const q = FontSize / 1000.0

// columns is the number of circle columns in a row, and columnWidth is the
// width of a column in glyph space units.
const (
	columns     = 10
	columnWidth = 500
)

// RowWidth is the width of a row of circles in user space units.
const RowWidth = columns * columnWidth * q

// Headers draws the column labels for the rows which follow.  The left
// edge of the rows is at x, and y is moved down to the baseline of the
// first row.
func Headers(page *document.Page, f text.F, x float64, y *float64) {
	*y -= 32

	M := matrix.Translate(x+250*q+2, *y)
	M = matrix.RotateDeg(90).Mul(M)

	page.TextBegin()
	page.TextSetFont(f.Font, f.Size)
	page.SetFillColor(f.Color)
	page.TextSetMatrix(M)
	page.TextShow("CID 0")
	page.TextSecondLine(0, -500*q)
	page.TextShow("notdef")
	page.TextNextLine()
	page.TextShow("glyph")
	page.TextFirstLine(0, -1500*q)
	page.TextShow("width 0")
	page.TextNextLine()
	page.TextShow("CID 0 width")
	page.TextNextLine()
	page.TextShow("notdef width")
	page.TextNextLine()
	page.TextShow("glyph width")
	page.TextNextLine()
	page.TextShow("default width")
	page.TextEnd()

	*y -= 11
}

// Row draws one row of circles with its left edge at x and its baseline at
// y, and shows the test string for the given code on top.  The circles for
// the expected glyph gIdx and the expected width wIdx are filled in green.
// Afterwards, y is moved down to the baseline of the next row.
func Row(page *document.Page, test text.F, x float64, y *float64, code byte, gIdx, wIdx int) {
	for i := range 3 {
		if i == gIdx {
			good(page, x, *y, i)
		} else {
			choice(page, x, *y, i)
		}
	}
	choice(page, x, *y, 5)
	for j := range 3 {
		if j == wIdx {
			good(page, x, *y, j+6)
		} else {
			choice(page, x, *y, j+6)
		}
	}
	choice(page, x, *y, 9)
	text.Show(page.Builder,
		text.M{X: x, Y: *y},
		test,
		pdf.String{code, 'D', 'D', 'D', 'D', 'D', 'A'},
	)

	*y -= RowHeight
}

func choice(page *document.Page, x0, y0 float64, idx int) {
	page.SetLineWidth(0.8)
	page.SetStrokeColor(color.DeviceGray(0.6))
	circle(page, x0, y0, idx)
	page.Stroke()
}

func good(page *document.Page, x0, y0 float64, idx int) {
	page.SetFillColor(color.DeviceRGB{0.3, 1, 0.3})
	circle(page, x0, y0, idx)
	page.Fill()
}

func circle(page *document.Page, x0, y0 float64, idx int) {
	x := x0 + (columnWidth/2+columnWidth*float64(idx))*q
	y := y0 + 150*q
	page.Circle(x, y, 130*q)
}

// CrossGlyph returns a glyph of the given width which shows a cross centred
// at horizontal position x.  A negative x gives a blank glyph.
func CrossGlyph(x, width float64) *cff.Glyph {
	g := &cff.Glyph{Width: width}
	if x < 0 {
		return g
	}

	const (
		y  = 150
		r  = 140
		lw = 32
	)
	a := math.Round(r / math.Sqrt2)
	b := math.Round(0.5 * lw / math.Sqrt2)

	g.MoveTo(x-a-b, y-a+b)
	g.LineTo(x-a+b, y-a-b)
	g.LineTo(x, y-2*b)
	g.LineTo(x+a-b, y-a-b)
	g.LineTo(x+a+b, y-a+b)
	g.LineTo(x+2*b, y)
	g.LineTo(x+a+b, y+a-b)
	g.LineTo(x+a-b, y+a+b)
	g.LineTo(x, y+2*b)
	g.LineTo(x-a+b, y+a+b)
	g.LineTo(x-a-b, y+a-b)
	g.LineTo(x-2*b, y)
	return g
}
