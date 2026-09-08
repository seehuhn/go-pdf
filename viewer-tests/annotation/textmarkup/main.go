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
	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/fallback"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content/builder"
)

const (
	leftColStart  = 36.0
	rightColStart = 304.0
	colWidth      = 244.0
	startY        = 780.0
)

func main() {
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func createDocument(filename string) error {
	paper := document.A4
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	page, err := document.CreateSinglePage(filename, paper, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	style, err := fallback.NewStyle().New(pdf.V1_7)
	if err != nil {
		return err
	}

	w := &writer{
		b:     page.Builder,
		font:  font.Must(standard.TimesRoman.New()),
		style: style,
		page:  page,
		yPos:  startY,
	}

	// title
	B := font.Must(standard.TimesBold.New())
	page.TextBegin()
	page.TextSetMatrix(matrix.Translate(leftColStart, w.yPos))
	page.TextSetFont(B, 12)
	glyphs := page.TextLayout(nil, "Your PDF viewer")
	glyphs.Align(colWidth, 0.5)
	page.TextShowGlyphs(glyphs)
	page.TextSetMatrix(matrix.Translate(rightColStart, w.yPos))
	glyphs = page.TextLayout(nil, "Quire appearance stream")
	glyphs.Align(colWidth, 0.5)
	page.TextShowGlyphs(glyphs)
	page.TextEnd()
	w.yPos -= 36.0

	// test cases
	rows := []struct {
		markupType annotation.TextMarkupType
		col        color.Color
		desc       string
		draw       func(x, y float64, text string) []vec.Vec2
	}{
		{annotation.TextMarkupTypeHighlight, color.DeviceRGB{1, 1, 0}, "yellow highlight", w.drawText},
		{annotation.TextMarkupTypeHighlight, color.DeviceRGB{0.6, 0.8, 1}, "blue highlight (rainbow text)", w.drawRainbowText},
		{annotation.TextMarkupTypeUnderline, color.Red, "red underline", w.drawText},
		{annotation.TextMarkupTypeStrikeOut, color.Blue, "blue strikeout", w.drawText},
		{annotation.TextMarkupTypeSquiggly, color.DeviceRGB{0, 0.6, 0}, "green squiggly", w.drawText},
		{annotation.TextMarkupTypeHighlight, nil, "nil color (invisible?)", w.drawText},
	}
	for _, row := range rows {
		err = w.addRow(row.markupType, row.col, row.desc, row.draw)
		if err != nil {
			return err
		}
	}
	err = w.addMultiQuadRow()
	if err != nil {
		return err
	}

	return page.Close()
}

type writer struct {
	b     *builder.Builder
	font  font.Layouter
	style *fallback.Generator
	page  *document.Page
	yPos  float64
}

func (w *writer) addAnnotation(a annotation.Annotation) {
	w.page.Page.AddAnnots(a)
}

// label draws the description of a test row above its left column.
func (w *writer) label(desc string) {
	b := w.b
	b.TextBegin()
	b.TextSetFont(w.font, 8)
	b.SetFillColor(color.DeviceGray(0.4))
	b.TextSetMatrix(matrix.Translate(leftColStart, w.yPos+14))
	b.TextShow(desc)
	b.TextEnd()
}

// newMarkup returns a text markup annotation covering the quads qq, with
// Rect the bounding box of the quads plus a small margin.
func newMarkup(markupType annotation.TextMarkupType, col color.Color, qq []vec.Vec2) *annotation.TextMarkup {
	a := &annotation.TextMarkup{
		Common: annotation.Common{
			Flags: annotation.FlagPrint,
			Color: col,
		},
		Type:       markupType,
		QuadPoints: qq,
	}
	for _, p := range qq {
		a.Rect.ExtendVec(p)
	}
	a.Rect.LLx -= 2
	a.Rect.LLy -= 2
	a.Rect.URx += 2
	a.Rect.URy += 2
	a.Rect.IRound(1)
	return a
}

// addPair adds the same markup to both columns: on the left without an
// appearance stream, for the viewer to draw, and on the right with the
// Quire fallback appearance.  The two quad slices come from the text drawn
// in each column.
func (w *writer) addPair(markupType annotation.TextMarkupType, col color.Color, qqLeft, qqRight []vec.Vec2) error {
	w.addAnnotation(newMarkup(markupType, col, qqLeft))

	right := newMarkup(markupType, col, qqRight)
	err := w.style.AddAppearance(right)
	if err != nil {
		return err
	}
	w.addAnnotation(right)

	w.yPos -= 36.0
	return nil
}

// addRow adds a test row with the same text in both columns, drawn by draw.
func (w *writer) addRow(markupType annotation.TextMarkupType, col color.Color, desc string, draw func(x, y float64, text string) []vec.Vec2) error {
	text := "The quick brown fox"
	w.label(desc)
	qqL := draw(leftColStart+20, w.yPos, text)
	qqR := draw(rightColStart+20, w.yPos, text)
	return w.addPair(markupType, col, qqL, qqR)
}

// addMultiQuadRow adds a test row with two separate quads (one per word).
func (w *writer) addMultiQuadRow() error {
	w.label("yellow highlight, two quads")
	qqL := w.drawText(leftColStart+20, w.yPos, "Hello")
	qqL = append(qqL, w.drawText(leftColStart+20+60, w.yPos, "World")...)
	qqR := w.drawText(rightColStart+20, w.yPos, "Hello")
	qqR = append(qqR, w.drawText(rightColStart+20+60, w.yPos, "World")...)
	return w.addPair(annotation.TextMarkupTypeHighlight, color.DeviceRGB{1, 1, 0}, qqL, qqR)
}

var rainbow = []color.Color{
	color.DeviceRGB{1, 0, 0},
	color.DeviceRGB{1, 0.5, 0},
	color.DeviceRGB{0.8, 0.8, 0},
	color.DeviceRGB{0, 0.7, 0},
	color.DeviceRGB{0, 0, 1},
	color.DeviceRGB{0.5, 0, 0.8},
	color.DeviceRGB{0.8, 0, 0.6},
}

// drawRainbowText draws text at (x, y) with each character in a different
// rainbow color, and returns the quad points.
func (w *writer) drawRainbowText(x, y float64, text string) []vec.Vec2 {
	b := w.b
	b.TextBegin()
	b.TextSetFont(w.font, 12)
	b.TextSetMatrix(matrix.Translate(x, y))

	// layout the whole string to get quad points
	glyphs := b.TextLayout(nil, text)
	qq := b.TextGetQuadPoints(glyphs, 0)

	// Draw the glyphs of that layout one at a time, with cycling colours.
	// Laying each glyph out on its own instead would drop the kerning
	// between them, and the text would no longer match the quad points.
	one := &font.GlyphSeq{Skip: glyphs.Skip}
	for i := range glyphs.Seq {
		b.SetFillColor(rainbow[i%len(rainbow)])
		one.Seq = glyphs.Seq[i : i+1]
		b.TextShowGlyphs(one)
		one.Skip = 0
	}
	b.TextEnd()
	return qq
}

// drawText draws text at (x, y) and returns the quad points.
func (w *writer) drawText(x, y float64, text string) []vec.Vec2 {
	b := w.b
	b.TextBegin()
	b.TextSetFont(w.font, 12)
	b.SetFillColor(color.DeviceGray(0))
	b.TextSetMatrix(matrix.Translate(x, y))
	glyphs := b.TextLayout(nil, text)
	qq := b.TextGetQuadPoints(glyphs, 0)
	b.TextShowGlyphs(glyphs)
	b.TextEnd()
	return qq
}
