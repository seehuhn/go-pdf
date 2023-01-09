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
	"time"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/boxes"
	"seehuhn.de/go/pdf/color"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/font/names"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pages"
)

const documentTitle = "The 14 Built-in PDF Fonts"
const pageHeight = 62

type glyphBox struct {
	text *boxes.TextBox
}

func glyph(f *font.Font, ptSize float64, rr []rune) *glyphBox {
	text := boxes.Text(f, ptSize, string(rr))
	return &glyphBox{
		text: text,
	}
}

func (gl *glyphBox) Extent() *boxes.BoxExtent {
	return gl.text.Extent()
}

func (gl *glyphBox) Draw(page *graphics.Page, xPos, yPos float64) {
	font := gl.text.Font
	sz := gl.text.FontSize

	page.PushGraphicsState()
	x := xPos
	y := yPos
	page.SetFillColor(color.RGB(.4, 1, .4))
	for _, glyph := range gl.text.Glyphs {
		gid := glyph.Gid
		ext := font.GlyphExtents[gid]
		page.Rectangle(
			x+font.ToPDF16(sz, ext.LLx+glyph.XOffset),
			y+font.ToPDF16(sz, ext.LLy+glyph.YOffset),
			font.ToPDF16(sz, ext.URx-ext.LLx),
			font.ToPDF16(sz, ext.URy-ext.LLy))
		x += font.ToPDF16(sz, glyph.Advance)
	}
	page.Fill()
	page.PopGraphicsState()

	gl.text.Draw(page, xPos, yPos)
}

type fontTables struct {
	w           *pdf.Writer
	tree        *pages.Tree
	paperHeight float64
	paperWidth  float64
	textWidth   float64

	bodyFont  *font.Font
	titleFont *font.Font

	pageNo int
	fontNo int

	content   []boxes.Box
	available int
}

func (f *fontTables) GetGlyphRows(fontName string) ([]boxes.Box, error) {
	targetAfm, err := builtin.Afm(fontName)
	if err != nil {
		return nil, err
	}
	nGlyph := len(targetAfm.Code)

	nFont := (nGlyph + 255) / 256 // at most 256 glyphs per font
	tf := make([]*font.Font, nFont)
	for i := 0; i < nFont; i++ {
		name := fmt.Sprintf("T%d", f.fontNo)
		f.fontNo++
		targetFont, err := builtin.EmbedAfm(f.w, targetAfm, pdf.Name(name))
		if err != nil {
			return nil, err
		}
		tf[i] = targetFont
	}

	var res []boxes.Box
	for i := 0; i < nGlyph; i++ {
		iF := i / 256

		name := targetAfm.GlyphName[i]
		rr := names.ToUnicode(name, targetAfm.IsDingbats)
		line := boxes.HBoxTo(120,
			boxes.HBoxTo(16,
				boxes.Glue(0, 1, 1, 1, 1),
				boxes.Text(f.bodyFont, 10, fmt.Sprintf("%d", i))),
			boxes.HBoxTo(24,
				boxes.Glue(0, 1, 1, 1, 1),
				glyph(tf[iF], 10, rr),
				boxes.Glue(0, 1, 1, 1, 1)),
			boxes.Text(f.bodyFont, 10, name),
		)
		res = append(res, line)
	}
	return res, nil
}

func (f *fontTables) MakeColumns(fontName string) error {
	bb, err := f.GetGlyphRows(fontName)
	if err != nil {
		return err
	}

	p := boxes.Parameters{
		BaseLineSkip: 12,
	}

	for len(bb) > 0 {
		height := (len(bb) + 3) / 4
		if height > pageHeight {
			height = pageHeight
		}
		if height > f.available && f.available > 0 {
			height = f.available
		}
		err := f.TryFlush(height)
		if err != nil {
			return err
		}

		var cc []boxes.Box
		for i := 0; i < 4 && len(bb) > 0; i++ {
			var col []boxes.Box
			if height > len(bb) {
				height = len(bb)
			}
			col, bb = bb[:height], bb[height:]

			colBox := p.VTop(col...)
			if len(cc) > 0 {
				cc = append(cc, boxes.Kern(12))
			}
			cc = append(cc, colBox)
		}
		f.content = append(f.content, boxes.HBox(cc...))
	}

	return nil
}

func (f *fontTables) TryFlush(required int) error {
	if f.available < required {
		err := f.DoFlush()
		if err != nil {
			return err
		}
	}

	f.available -= required
	return nil
}

func (f *fontTables) DoFlush() error {
	p := boxes.Parameters{
		BaseLineSkip: 0,
	}

	f.pageNo++
	pageList := []boxes.Box{
		boxes.Kern(36),
	}
	pageList = append(pageList, f.content...)
	pageList = append(pageList,
		boxes.Glue(0, 1, 1, 1, 1),
		boxes.HBoxTo(f.textWidth,
			boxes.Glue(0, 1, 1, 1, 1),
			boxes.Text(f.bodyFont, 10, fmt.Sprintf("- %d -", f.pageNo)),
			boxes.Glue(0, 1, 1, 1, 1),
		),
		boxes.Kern(36),
	)
	pageBody := p.VBoxTo(f.paperHeight, pageList...)
	withMargins := boxes.HBox(boxes.Kern(50), pageBody)

	g, err := graphics.AppendPage(f.tree)
	if err != nil {
		return err
	}
	withMargins.Draw(g, 0, withMargins.Extent().Depth)
	_, err = g.Close()
	if err != nil {
		return err
	}

	f.content = nil
	f.available = pageHeight
	return nil
}

func (f *fontTables) AddTitle(title string) error {
	err := f.TryFlush(3 + 2 + 4)
	if err != nil {
		return err
	}

	f.content = append(f.content,
		boxes.Kern(36),
		boxes.HBoxTo(f.textWidth,
			boxes.Glue(0, 1, 1, 1, 1),
			boxes.Text(f.titleFont, 24, title),
			boxes.Glue(0, 1, 1, 1, 1),
		),
		boxes.Kern(48),
	)
	return nil
}

func (f *fontTables) AddSubTitle(title string) error {
	var cc []boxes.Box
	extra := 0
	if f.available < 10 {
		err := f.DoFlush()
		if err != nil {
			return err
		}
	} else {
		cc = append(cc, boxes.Kern(24))
		extra += 2
	}

	err := f.TryFlush(extra + 1 + 1)
	if err != nil {
		return err
	}

	f.content = append(f.content, cc...)
	f.content = append(f.content,
		boxes.HBoxTo(f.textWidth,
			boxes.Glue(0, 1, 1, 1, 1),
			boxes.Text(f.titleFont, 10, title),
			boxes.Glue(0, 1, 1, 1, 1),
		),
		boxes.Kern(12),
	)
	return nil
}

func main() {
	w, err := pdf.Create("builtin.pdf")
	if err != nil {
		log.Fatal(err)
	}

	paper := pages.A4
	tree := pages.InstallTree(w, &pages.InheritableAttributes{
		MediaBox: paper,
	})

	labelFont, err := builtin.Embed(w, "Times-Roman", "F")
	if err != nil {
		log.Fatal(err)
	}
	titleFont, err := builtin.Embed(w, "Times-Bold", "B")
	if err != nil {
		log.Fatal(err)
	}

	f := fontTables{
		w:           w,
		tree:        tree,
		paperHeight: paper.URy,
		paperWidth:  paper.URx,
		textWidth:   paper.URx - 100,

		bodyFont:  labelFont,
		titleFont: titleFont,

		available: pageHeight,
	}
	err = f.AddTitle(documentTitle)
	if err != nil {
		log.Fatal(err)
	}

	for _, fontName := range builtin.FontNames {
		err = f.AddSubTitle(fontName)
		if err != nil {
			log.Fatal(err)
		}
		err = f.MakeColumns(fontName)
		if err != nil {
			log.Fatal(err)
		}
	}
	f.DoFlush()

	w.SetInfo(&pdf.Info{
		Title:        documentTitle,
		Producer:     "seehuhn.de/go/pdf/demo/builtin-fonts",
		CreationDate: time.Now(),
	})

	err = w.Close()
	if err != nil {
		log.Fatal(err)
	}
}
