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
	"seehuhn.de/go/pdf/color"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cid"
	"seehuhn.de/go/pdf/font/type1"
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

	cmap, _ := tt.CMapTable.GetBest()

	rev := make(map[glyph.ID]rune)
	if cmap != nil {
		min, max := cmap.CodeRange()
		for r := min; r <= max; r++ {
			gid := cmap.Lookup(r)
			if gid == 0 {
				continue
			}
			r2 := rev[gid]
			if r2 == 0 || r < r2 {
				rev[gid] = r
			}
		}
	}

	paper := document.A4
	doc, err := document.CreateMultiPage("test.pdf", paper, nil)
	if err != nil {
		log.Fatal(err)
	}

	helvetica, err := type1.Helvetica.Embed(doc.Out, "R")
	if err != nil {
		log.Fatal(err)
	}
	italic, err := type1.TimesItalic.Embed(doc.Out, "I")
	if err != nil {
		log.Fatal(err)
	}
	courier, err := type1.Courier.Embed(doc.Out, "T")
	if err != nil {
		log.Fatal(err)
	}
	theFont, err := cid.Embed(doc.Out, tt, "X", language.Und)
	if err != nil {
		log.Fatal(err)
	}

	const margin = 36
	f := &fontTables{
		doc:        doc,
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

	numGlyphs := font.NumGlyphs(theFont)
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

	err = doc.Close()
	if err != nil {
		log.Fatal(err)
	}
}

type fontTables struct {
	doc *document.MultiPage

	textWidth  float64
	textHeight float64
	margin     float64

	bodyFont   font.Embedded
	italicFont font.Embedded
	monoFont   font.Embedded

	page   *document.Page
	pageNo int

	used float64 // vertical amount of page space currently used

	rev map[glyph.ID]rune
}

func (f *fontTables) ClosePage() error {
	if f.page == nil {
		return nil
	}

	f.pageNo++
	f.page.TextStart()
	f.page.TextSetFont(f.bodyFont, 10)
	f.page.TextFirstLine(f.margin+0.5*f.textWidth, f.margin-20)
	f.page.TextShowAligned(fmt.Sprintf("- %d -", f.pageNo), 0, 0.5)
	f.page.TextEnd()

	err := f.page.Close()
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

	f.page = f.doc.AddPage()
	f.used = 0
	return nil
}

func (f *fontTables) WriteHeader(title, fileName string) error {
	gBody := f.bodyFont.GetGeometry()
	gMono := f.monoFont.GetGeometry()
	v1 := gBody.ToPDF16(12, gBody.Ascent)
	v2 := gBody.ToPDF16(12, gBody.BaseLineDistance-gBody.Ascent) +
		gMono.ToPDF16(10, gMono.Ascent)
	v3 := gMono.ToPDF16(10, gMono.BaseLineDistance-gMono.Ascent) +
		12
	total := v1 + v2 + v3

	err := f.MakeSpace(total)
	if err != nil {
		return err
	}

	f.page.TextStart()
	f.page.TextSetFont(f.bodyFont, 12)
	f.page.TextFirstLine(f.margin, f.margin+f.textHeight-f.used-v1)
	f.page.TextShow(title)
	f.page.TextSetFont(f.monoFont, 10)
	f.page.TextSecondLine(0, -v2)
	f.page.TextShow(fileName)
	f.page.TextEnd()

	f.used += total
	return nil
}

func (f *fontTables) WriteGlyphRow(theFont font.Embedded, start int) error {
	const glyphSize = 24

	geom := theFont.GetGeometry()

	gid := make([]glyph.ID, 0, 10)
	for i := start; i < start+10; i++ {
		if i >= len(geom.Widths) {
			break
		}
		gid = append(gid, glyph.ID(i))
	}

	v1 := geom.ToPDF16(glyphSize, geom.Ascent)
	v2 := geom.ToPDF16(glyphSize, -geom.Descent)
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
	page.TextStart()
	page.TextFirstLine(left-24, yBase)
	page.TextSetFont(f.bodyFont, 10)
	var label string
	if start > 0 {
		label = fmt.Sprintf("%d", start/10)
	}
	page.TextShowAligned(label, 0, 1)
	page.TextSetFont(f.italicFont, 10)
	page.TextShow("x")
	page.TextEnd()

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
		w := geom.ToPDF16(glyphSize, geom.Widths[gid])
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
		ext := geom.GlyphExtents[gid]
		page.Rectangle(
			xPos[i]+geom.ToPDF16(glyphSize, ext.LLx),
			yBase+geom.ToPDF16(glyphSize, ext.LLy),
			geom.ToPDF16(glyphSize, ext.URx-ext.LLx),
			geom.ToPDF16(glyphSize, ext.URy-ext.LLy))
	}
	page.Fill()
	page.PopGraphicsState()

	// draw the glyphs and labels
	for i, gid := range gid {
		g := glyph.Info{
			Gid:     gid,
			Advance: geom.Widths[gid],
		}

		r := f.rev[gid]
		var label string
		if r > 0 {
			if r, ok := f.rev[gid]; ok {
				g.Text = []rune{r}

				// TODO(voss): fix this
				// Try to establish a mapping from glyph ID to rune in the embedded
				// font (called for side effects only).
				_ = theFont.Layout(string([]rune{r}), glyphSize)
			}
			if unicode.IsPrint(r) && r < 128 {
				label = fmt.Sprintf("%q", r)
			} else {
				label = fmt.Sprintf("U+%04X", r)
			}
		}

		page.TextStart()
		page.TextSetFont(theFont, glyphSize)
		page.TextFirstLine(xPos[i], yBase)
		page.TextShowGlyphs(glyph.Seq{g})
		page.TextSetFont(f.bodyFont, 8)
		page.TextFirstLine(left+float64(i)*dx-xPos[i], -v2-7.5)
		page.TextShowAligned(label, dx, 0.5)
		page.TextEnd()
	}

	f.used += total
	return nil
}
