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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/color"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pages"
	"seehuhn.de/go/sfnt/glyph"
)

func main() {
	const documentTitle = "The 14 Built-in PDF Fonts"
	const margin = 50

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
		tree: tree,

		textWidth:  paper.URx - 2*margin,
		textHeight: paper.URy - 2*margin,
		margin:     margin,

		bodyFont:  labelFont,
		titleFont: titleFont,
	}

	err = f.AddTitle(documentTitle, 24, 36, 36)
	if err != nil {
		log.Fatal(err)
	}
	for _, fontName := range builtin.FontNames {
		err = f.AddTitle(fontName, 10, 36, 12)
		if err != nil {
			log.Fatal(err)
		}
		err = f.MakeColumns(fontName)
		if err != nil {
			log.Fatal(err)
		}
	}

	err = f.ClosePage()
	if err != nil {
		log.Fatal(err)
	}

	err = w.Close()
	if err != nil {
		log.Fatal(err)
	}
}

type fontTables struct {
	tree *pages.Tree

	textWidth  float64
	textHeight float64
	margin     float64

	used float64 // vertical amount of page space currently used

	bodyFont  *font.Font
	titleFont *font.Font

	page *graphics.Page

	pageNo int
	fontNo int
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
	if f.page != nil && f.used+vSpace < f.textWidth {
		// If we have enough space, just return ...
		return nil
	}

	// ... otherwise start a new page.
	err := f.ClosePage()
	if err != nil {
		return err
	}

	page, err := graphics.AppendPage(f.tree)
	if err != nil {
		return err
	}

	f.page = page
	f.used = 0
	return nil
}

func (f *fontTables) AddTitle(title string, fontSize, a, b float64) error {
	err := f.MakeSpace(a + b + 72)
	if err != nil {
		return err
	}

	f.used += a
	f.page.BeginText()
	f.page.SetFont(f.titleFont, fontSize)
	f.page.StartLine(f.margin+0.5*f.textWidth, f.margin+f.textHeight-f.used)
	f.page.ShowTextAligned(title, 0, 0.5)
	f.page.EndText()

	f.used += b

	return nil
}

func (f *fontTables) MakeColumns(fontName string) error {
	fontSize := 10.0

	afm, err := builtin.Afm(fontName)
	if err != nil {
		return err
	}

	nGlyph := len(afm.Code)

	baseLineSkip := 12.0
	colWidth := (f.textWidth + 32) / 4

	var font *font.Font

	curGlyph := 0
	for curGlyph < nGlyph {
		// we need space for at least one line
		err = f.MakeSpace(baseLineSkip)
		if err != nil {
			return nil
		}
		page := f.page

		rowsAvailable := int((f.textHeight - f.used) / baseLineSkip)
		rowsNeeded := (nGlyph - curGlyph + 3) / 4
		nRows := rowsAvailable
		if nRows > rowsNeeded {
			nRows = rowsNeeded
		}

		yTop := f.margin + f.textHeight - f.used - afm.Ascent.AsFloat(fontSize/1000)

		// draw the rectanges for the glyph extents in the background
		tmpGlyph := curGlyph
		page.PushGraphicsState()
		page.SetFillColor(color.RGB(.4, 1, .4))
		for col := 0; col < 4; col++ {
			x := f.margin + float64(col)*colWidth
			for i := 0; i < nRows; i++ {
				if tmpGlyph >= nGlyph {
					break
				}
				y := yTop - baseLineSkip*float64(i)

				ext := afm.GlyphExtents[tmpGlyph]
				if !ext.IsZero() {
					w := afm.Widths[tmpGlyph].AsFloat(fontSize / 1000)
					page.Rectangle(
						x+32-w/2+ext.LLx.AsFloat(fontSize/1000),
						y+ext.LLy.AsFloat(fontSize/1000),
						(ext.URx - ext.LLx).AsFloat(fontSize/1000),
						(ext.URy - ext.LLy).AsFloat(fontSize/1000))
				}

				tmpGlyph++
			}
		}
		page.Fill()
		page.PopGraphicsState()

		// draw the colunmns of text
		for col := 0; col < 4; col++ {
			page.BeginText()
			x := f.margin + float64(col)*colWidth
			for i := 0; i < nRows; i++ {
				if curGlyph >= nGlyph {
					break
				}

				if curGlyph%256 == 0 {
					instName := pdf.Name(fmt.Sprintf("X%d", f.fontNo))
					f.fontNo++
					font, err = builtin.EmbedAfm(f.tree.Out, afm, instName)
					if err != nil {
						return err
					}
				}

				y := yTop - baseLineSkip*float64(i)

				if i == 0 {
					page.StartLine(x, y)
				} else if i == 1 {
					page.StartNextLine(0, -baseLineSkip)
				} else {
					page.NewLine()
				}

				code := "—"
				if afm.Code[curGlyph] >= 0 {
					code = fmt.Sprintf("%d", afm.Code[curGlyph])
				}
				page.SetFont(f.bodyFont, fontSize)
				page.ShowTextAligned(code, 16, 1)

				page.SetFont(font, fontSize)
				g := glyph.Seq{
					{
						Gid:     glyph.ID(curGlyph),
						Advance: font.Widths[curGlyph],
					},
				}
				page.ShowGlyphsAligned(g, 32, 0.5)

				page.SetFont(f.bodyFont, fontSize)
				page.ShowText(afm.GlyphName[curGlyph])

				curGlyph++
			}
			page.EndText()
		}
		f.used += float64(nRows) * baseLineSkip
	}
	return nil
}
