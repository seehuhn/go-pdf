// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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
	"log"
	"os"
	"unicode"

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/color"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/font/cid"
	"seehuhn.de/go/pdf/pages"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyph"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: sfnt-glyph-table (font.ttf|font.otf)")
		os.Exit(1)
	}
	fontFileName := os.Args[1]
	tt, err := sfnt.ReadFile(fontFileName)
	if err != nil {
		log.Fatal(err)
	}

	rev := make(map[glyph.ID]rune)
	min, max := tt.CMap.CodeRange()
	for r := min; r <= max; r++ {
		gid := tt.CMap.Lookup(r)
		if gid == 0 {
			continue
		}
		r2 := rev[gid]
		if r2 == 0 || r < r2 {
			rev[gid] = r
		}
	}

	out, err := pdf.Create("test.pdf")
	if err != nil {
		log.Fatal(err)
	}

	helvetica, err := builtin.Embed(out, "Helvetica", "R")
	if err != nil {
		log.Fatal(err)
	}
	italic, err := builtin.Embed(out, "Times-Italic", "I")
	if err != nil {
		log.Fatal(err)
	}
	courier, err := builtin.Embed(out, "Courier", "T")
	if err != nil {
		log.Fatal(err)
	}
	theFont, err := cid.Embed(out, tt, "X", language.AmericanEnglish)
	if err != nil {
		log.Fatal(err)
	}

	paper := pages.A4
	pageTree := pages.InstallTree(out, &pages.InheritableAttributes{
		MediaBox: paper,
	})

	const margin = 36
	f := &fontTables{
		tree:       pageTree,
		textWidth:  paper.URx - 2*margin,
		textHeight: paper.URy - 2*margin,
		margin:     margin,
		bodyFont:   helvetica,
		italicFont: italic,
		monoFont:   courier,
		rev:        rev,
	}

	err = f.WriteHeader(tt.FullName(), fontFileName)
	if err != nil {
		log.Fatal(err)
	}

	numGlyphs := theFont.NumGlyphs()
	for i := 0; i < numGlyphs; i += 10 {
		err = f.WriteGlyphRow(theFont, i)
		if err != nil {
			log.Fatal(err)
		}
	}

	err = f.ClosePage()
	if err != nil {
		log.Fatal(err)
	}

	err = out.Close()
	if err != nil {
		log.Fatal(err)
	}
}

type fontTables struct {
	tree *pages.Tree

	textWidth  float64
	textHeight float64
	margin     float64

	bodyFont   *font.Font
	italicFont *font.Font
	monoFont   *font.Font

	page   *pages.Page
	pageNo int

	used float64 // vertical amount of page space currently used

	rev map[glyph.ID]rune
}

func (f *fontTables) ClosePage() error {
	if f.page == nil {
		return nil
	}

	f.pageNo++
	f.page.BeginText()
	f.page.SetFont(f.bodyFont, 10)
	f.page.StartLine(f.margin+0.5*f.textWidth, f.margin-20)
	f.page.ShowTextAligned(fmt.Sprintf("- %d -", f.pageNo), 0, 0.5)
	f.page.EndText()

	_, err := f.page.Close()
	f.page = nil

	return err
}

func (f *fontTables) MakeSpace(vSpace float64) error {
	if f.page != nil && f.used+vSpace < f.textHeight {
		// If we have enough space, just return ...
		return nil
	}

	// ... otherwise start a new page.
	err := f.ClosePage()
	if err != nil {
		return err
	}

	page, err := pages.AppendPage(f.tree)
	if err != nil {
		return err
	}

	f.page = page
	f.used = 0
	return nil
}

func (f *fontTables) WriteHeader(title, fileName string) error {
	v1 := f.bodyFont.ToPDF16(12, f.bodyFont.Ascent)
	v2 := f.bodyFont.ToPDF16(12, f.bodyFont.BaseLineSkip-f.bodyFont.Ascent) +
		f.monoFont.ToPDF16(10, f.monoFont.Ascent)
	v3 := f.monoFont.ToPDF16(10, f.monoFont.BaseLineSkip-f.monoFont.Ascent) +
		12
	total := v1 + v2 + v3

	err := f.MakeSpace(total)
	if err != nil {
		return err
	}

	f.page.BeginText()
	f.page.SetFont(f.bodyFont, 12)
	f.page.StartLine(f.margin, f.margin+f.textHeight-f.used-v1)
	f.page.ShowText(title)
	f.page.SetFont(f.monoFont, 10)
	f.page.StartNextLine(0, -v2)
	f.page.ShowText(fileName)
	f.page.EndText()

	f.used += total
	return nil
}

func (f *fontTables) WriteGlyphRow(theFont *font.Font, start int) error {
	const glyphSize = 24

	gid := make([]glyph.ID, 0, 10)
	for i := start; i < start+10; i++ {
		if i >= theFont.NumGlyphs() {
			break
		}
		gid = append(gid, glyph.ID(i))
	}

	v1 := theFont.ToPDF16(glyphSize, theFont.Ascent)
	v2 := theFont.ToPDF16(glyphSize, -theFont.Descent)
	v3 := 12.0
	total := v1 + v2 + v3

	err := f.MakeSpace(total)
	if err != nil {
		return err
	}

	page := f.page

	yBase := f.margin + f.textHeight - f.used - v1
	left := f.margin + 72
	right := f.margin + f.textWidth
	dx := (right - left) / 10

	// row label on the left
	page.BeginText()
	page.StartLine(left-24, yBase)
	page.SetFont(f.bodyFont, 10)
	var label string
	if start > 0 {
		label = fmt.Sprintf("%d", start/10)
	}
	page.ShowTextAligned(label, 0, 1)
	page.SetFont(f.italicFont, 10)
	page.ShowText("x")
	page.EndText()

	// grid of boxes
	page.PushGraphicsState()
	page.SetStrokeColor(color.RGB(.3, .3, 1))
	page.SetLineWidth(.5)
	page.MoveTo(left, yBase+v1)
	page.LineTo(right, yBase+v1)
	page.MoveTo(left, yBase)
	page.LineTo(right, yBase)
	page.MoveTo(left, yBase-v2)
	page.LineTo(right, yBase-v2)
	for i := 0; i <= 10; i++ {
		x := left + float64(i)*dx
		page.MoveTo(x, yBase+v1)
		page.LineTo(x, yBase-v2)
	}
	page.Stroke()
	xPos := make([]float64, len(gid))
	page.SetStrokeColor(color.RGB(1, 0, 0))
	for i, gid := range gid {
		w := theFont.ToPDF16(glyphSize, theFont.Widths[gid])
		xPos[i] = left + (float64(i)+0.5)*dx - 0.5*w
		page.MoveTo(xPos[i], yBase+v1)
		page.LineTo(xPos[i], yBase-v2)
		page.MoveTo(xPos[i]+w, yBase+v1)
		page.LineTo(xPos[i]+w, yBase-v2)
	}
	page.Stroke()
	page.PopGraphicsState()

	// boxes for glyph extent
	page.PushGraphicsState()
	page.SetFillColor(color.RGB(.4, 1, .4))
	for i, gid := range gid {
		ext := theFont.GlyphExtents[gid]
		page.Rectangle(
			xPos[i]+theFont.ToPDF16(glyphSize, ext.LLx),
			yBase+theFont.ToPDF16(glyphSize, ext.LLy),
			theFont.ToPDF16(glyphSize, ext.URx-ext.LLx),
			theFont.ToPDF16(glyphSize, ext.URy-ext.LLy))
	}
	page.Fill()
	page.PopGraphicsState()

	// draw the glyphs and labels
	for i, gid := range gid {
		g := glyph.Info{
			Gid:     gid,
			Advance: theFont.Widths[gid],
		}

		r := f.rev[gid]
		var label string
		if r > 0 {
			if r, ok := f.rev[gid]; ok {
				g.Text = []rune{r}

				// Try to establish a mapping from glyph ID to rune in the embedded
				// font (called for side effects only).
				_ = theFont.Layout([]rune{r})
			}
			if unicode.IsPrint(r) && r < 128 {
				label = fmt.Sprintf("%q", r)
			} else {
				label = fmt.Sprintf("U+%04X", r)
			}
		}

		page.BeginText()
		page.SetFont(theFont, glyphSize)
		page.StartLine(xPos[i], yBase)
		page.ShowGlyphs(glyph.Seq{g})
		page.SetFont(f.bodyFont, 8)
		page.StartLine(left+float64(i)*dx-xPos[i], -v2-7.5)
		page.ShowTextAligned(label, dx, 0.5)
		page.EndText()
	}

	f.used += total
	return nil
}
