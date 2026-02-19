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

	w := &writer{
		b:     page.Builder,
		font:  standard.TimesRoman.New(),
		style: fallback.NewStyle(),
		page:  page,
		yPos:  startY,
	}

	// title
	B := standard.TimesBold.New()
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

	err = w.addRow(annotation.TextMarkupTypeHighlight,
		color.DeviceRGB{1, 1, 0}, "yellow highlight")
	if err != nil {
		return err
	}
	err = w.addRainbowRow(annotation.TextMarkupTypeHighlight,
		color.DeviceRGB{0.6, 0.8, 1}, "blue highlight (rainbow text)")
	if err != nil {
		return err
	}
	err = w.addRow(annotation.TextMarkupTypeUnderline,
		color.Red, "red underline")
	if err != nil {
		return err
	}
	err = w.addRowWithBorder(annotation.TextMarkupTypeUnderline,
		color.Red, 2, "2pt red underline")
	if err != nil {
		return err
	}
	err = w.addRow(annotation.TextMarkupTypeStrikeOut,
		color.Blue, "blue strikeout")
	if err != nil {
		return err
	}
	err = w.addRowWithBorder(annotation.TextMarkupTypeStrikeOut,
		color.Blue, 2, "2pt blue strikeout")
	if err != nil {
		return err
	}
	err = w.addRow(annotation.TextMarkupTypeSquiggly,
		color.DeviceRGB{0, 0.6, 0}, "green squiggly")
	if err != nil {
		return err
	}
	err = w.addRowWithBorder(annotation.TextMarkupTypeSquiggly,
		color.DeviceRGB{0, 0.6, 0}, 2, "2pt green squiggly")
	if err != nil {
		return err
	}
	err = w.addRow(annotation.TextMarkupTypeHighlight,
		nil, "nil color (invisible?)")
	if err != nil {
		return err
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
	style *fallback.Style
	page  *document.Page
	yPos  float64
}

func (w *writer) addAnnotation(a annotation.Annotation) {
	w.page.Page.Annots = append(w.page.Page.Annots, a)
}

// addRowWithBorder is like addRow but sets Border.Width on both annotations.
func (w *writer) addRowWithBorder(markupType annotation.TextMarkupType, col color.Color, borderWidth float64, desc string) error {
	b := w.b
	text := "The quick brown fox"

	b.TextBegin()
	b.TextSetFont(w.font, 8)
	b.SetFillColor(color.DeviceGray(0.4))
	b.TextSetMatrix(matrix.Translate(leftColStart, w.yPos+14))
	b.TextShow(desc)
	b.TextEnd()

	qq := w.drawText(leftColStart+20, w.yPos, text)
	left := &annotation.TextMarkup{
		Common: annotation.Common{
			Flags:  annotation.FlagPrint,
			Color:  col,
			Border: &annotation.Border{Width: borderWidth, SingleUse: true},
		},
		Type:       markupType,
		QuadPoints: qq,
	}
	for _, p := range qq {
		left.Common.Rect.ExtendVec(p)
	}
	left.Common.Rect.LLx -= borderWidth
	left.Common.Rect.LLy -= borderWidth
	left.Common.Rect.URx += borderWidth
	left.Common.Rect.URy += borderWidth
	left.Common.Rect.IRound(1)
	w.addAnnotation(left)

	qq = w.drawText(rightColStart+20, w.yPos, text)
	right := &annotation.TextMarkup{
		Common: annotation.Common{
			Flags:  annotation.FlagPrint,
			Color:  col,
			Border: &annotation.Border{Width: borderWidth, SingleUse: true},
		},
		Type:       markupType,
		QuadPoints: qq,
	}
	for _, p := range qq {
		right.Common.Rect.ExtendVec(p)
	}
	right.Common.Rect.LLx -= borderWidth
	right.Common.Rect.LLy -= borderWidth
	right.Common.Rect.URx += borderWidth
	right.Common.Rect.URy += borderWidth
	right.Common.Rect.IRound(1)
	err := w.style.AddAppearance(right)
	if err != nil {
		return err
	}
	w.addAnnotation(right)

	w.yPos -= 36.0
	return nil
}

// addRow adds a test row with left (viewer) and right (Quire) text markup.
func (w *writer) addRow(markupType annotation.TextMarkupType, col color.Color, desc string) error {
	b := w.b
	text := "The quick brown fox"

	// draw description label
	b.TextBegin()
	b.TextSetFont(w.font, 8)
	b.SetFillColor(color.DeviceGray(0.4))
	b.TextSetMatrix(matrix.Translate(leftColStart, w.yPos+14))
	b.TextShow(desc)
	b.TextEnd()

	// left column: text + annotation without appearance
	qq := w.drawText(leftColStart+20, w.yPos, text)
	left := &annotation.TextMarkup{
		Common: annotation.Common{
			Flags: annotation.FlagPrint,
			Color: col,
		},
		Type:       markupType,
		QuadPoints: qq,
	}
	for _, p := range qq {
		left.Common.Rect.ExtendVec(p)
	}
	left.Common.Rect.LLx -= 2
	left.Common.Rect.LLy -= 2
	left.Common.Rect.URx += 2
	left.Common.Rect.URy += 2
	left.Common.Rect.IRound(1)
	w.addAnnotation(left)

	// right column: text + annotation with Quire appearance
	qq = w.drawText(rightColStart+20, w.yPos, text)
	right := &annotation.TextMarkup{
		Common: annotation.Common{
			Flags: annotation.FlagPrint,
			Color: col,
		},
		Type:       markupType,
		QuadPoints: qq,
	}
	for _, p := range qq {
		right.Common.Rect.ExtendVec(p)
	}
	right.Common.Rect.LLx -= 2
	right.Common.Rect.LLy -= 2
	right.Common.Rect.URx += 2
	right.Common.Rect.URy += 2
	right.Common.Rect.IRound(1)
	err := w.style.AddAppearance(right)
	if err != nil {
		return err
	}
	w.addAnnotation(right)

	w.yPos -= 36.0
	return nil
}

// addMultiQuadRow adds a test row with two separate quads (one per word).
func (w *writer) addMultiQuadRow() error {
	b := w.b
	col := color.DeviceRGB{1, 1, 0}
	desc := "yellow highlight, two quads"

	b.TextBegin()
	b.TextSetFont(w.font, 8)
	b.SetFillColor(color.DeviceGray(0.4))
	b.TextSetMatrix(matrix.Translate(leftColStart, w.yPos+14))
	b.TextShow(desc)
	b.TextEnd()

	// left column
	qq1L := w.drawText(leftColStart+20, w.yPos, "Hello")
	qq2L := w.drawText(leftColStart+20+60, w.yPos, "World")
	qqL := append(qq1L, qq2L...)
	left := &annotation.TextMarkup{
		Common: annotation.Common{
			Flags: annotation.FlagPrint,
			Color: col,
		},
		Type:       annotation.TextMarkupTypeHighlight,
		QuadPoints: qqL,
	}
	for _, p := range qqL {
		left.Common.Rect.ExtendVec(p)
	}
	left.Common.Rect.LLx -= 2
	left.Common.Rect.LLy -= 2
	left.Common.Rect.URx += 2
	left.Common.Rect.URy += 2
	left.Common.Rect.IRound(1)
	w.addAnnotation(left)

	// right column
	qq1R := w.drawText(rightColStart+20, w.yPos, "Hello")
	qq2R := w.drawText(rightColStart+20+60, w.yPos, "World")
	qqR := append(qq1R, qq2R...)
	right := &annotation.TextMarkup{
		Common: annotation.Common{
			Flags: annotation.FlagPrint,
			Color: col,
		},
		Type:       annotation.TextMarkupTypeHighlight,
		QuadPoints: qqR,
	}
	for _, p := range qqR {
		right.Common.Rect.ExtendVec(p)
	}
	right.Common.Rect.LLx -= 2
	right.Common.Rect.LLy -= 2
	right.Common.Rect.URx += 2
	right.Common.Rect.URy += 2
	right.Common.Rect.IRound(1)
	err := w.style.AddAppearance(right)
	if err != nil {
		return err
	}
	w.addAnnotation(right)

	w.yPos -= 36.0
	return nil
}

// addRainbowRow is like addRow but draws the text in rainbow colors.
func (w *writer) addRainbowRow(markupType annotation.TextMarkupType, col color.Color, desc string) error {
	b := w.b

	b.TextBegin()
	b.TextSetFont(w.font, 8)
	b.SetFillColor(color.DeviceGray(0.4))
	b.TextSetMatrix(matrix.Translate(leftColStart, w.yPos+14))
	b.TextShow(desc)
	b.TextEnd()

	text := "The quick brown fox"

	qq := w.drawRainbowText(leftColStart+20, w.yPos, text)
	left := &annotation.TextMarkup{
		Common: annotation.Common{
			Flags: annotation.FlagPrint,
			Color: col,
		},
		Type:       markupType,
		QuadPoints: qq,
	}
	for _, p := range qq {
		left.Common.Rect.ExtendVec(p)
	}
	left.Common.Rect.LLx -= 2
	left.Common.Rect.LLy -= 2
	left.Common.Rect.URx += 2
	left.Common.Rect.URy += 2
	left.Common.Rect.IRound(1)
	w.addAnnotation(left)

	qq = w.drawRainbowText(rightColStart+20, w.yPos, text)
	right := &annotation.TextMarkup{
		Common: annotation.Common{
			Flags: annotation.FlagPrint,
			Color: col,
		},
		Type:       markupType,
		QuadPoints: qq,
	}
	for _, p := range qq {
		right.Common.Rect.ExtendVec(p)
	}
	right.Common.Rect.LLx -= 2
	right.Common.Rect.LLy -= 2
	right.Common.Rect.URx += 2
	right.Common.Rect.URy += 2
	right.Common.Rect.IRound(1)
	err := w.style.AddAppearance(right)
	if err != nil {
		return err
	}
	w.addAnnotation(right)

	w.yPos -= 36.0
	return nil
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

	// draw character by character with cycling colors
	for i, ch := range text {
		b.SetFillColor(rainbow[i%len(rainbow)])
		b.TextShow(string(ch))
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
