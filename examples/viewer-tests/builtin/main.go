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

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
)

func main() {
	const documentTitle = "The Standard 14 Fonts"
	const margin = 50

	paper := document.A4
	doc, err := document.CreateMultiPage("builtin.pdf", paper, pdf.V1_7, nil)
	if err != nil {
		log.Fatal(err)
	}

	titleFont, err := standard.TimesBold.New(nil)
	if err != nil {
		log.Fatal(err)
	}
	bodyFont, err := standard.TimesRoman.New(nil)
	if err != nil {
		log.Fatal(err)
	}

	f := fontTables{
		doc: doc,

		textWidth:  paper.URx - 2*margin,
		textHeight: paper.URy - 2*margin,
		margin:     margin,

		titleFont: titleFont,
		bodyFont:  bodyFont,
	}

	err = f.AddTitle(documentTitle, 24, 36, 36)
	if err != nil {
		log.Fatal(err)
	}

	for _, G := range standard.All {
		err = f.AddTitle(string(G), 10, 36, 12)
		if err != nil {
			log.Fatal(err)
		}
		err = f.MakeColumns(G)
		if err != nil {
			log.Fatal(err)
		}
	}

	err = f.FlushPage()
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

	used float64 // vertical amount of page space currently used

	titleFont font.Font
	bodyFont  font.Font

	page *document.Page

	pageNo int
}

func (f *fontTables) FlushPage() error {
	if f.page == nil {
		return nil
	}

	f.pageNo++
	f.page.TextBegin()
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
	err := f.FlushPage()
	if err != nil {
		return err
	}

	f.page = f.doc.AddPage()
	f.page.SetFontNameInternal(f.bodyFont, "R")
	f.page.SetFontNameInternal(f.titleFont, "B")
	f.used = 0
	return nil
}

// AddTitle adds a title to the current page, together with the given vertical
// space before and after the title.
func (f *fontTables) AddTitle(title string, fontSize, before, after float64) error {
	err := f.MakeSpace(before + after + 72)
	if err != nil {
		return err
	}

	f.used += before

	f.page.TextBegin()
	f.page.TextSetFont(f.titleFont, fontSize)
	f.page.TextFirstLine(f.margin+0.5*f.textWidth, f.margin+f.textHeight-f.used)
	f.page.TextShowAligned(title, 0, 0.5)
	f.page.TextEnd()

	f.used += after

	return nil
}

func (f *fontTables) MakeColumns(G standard.Font) error {
	fontSize := 10.0
	baseLineSkip := 12.0
	colWidth := (f.textWidth + 32) / 4

	fnt, err := G.New(nil)
	if err != nil {
		return err
	}

	afm := fnt.Metrics
	glyphNames := afm.GlyphList()
	nGlyph := len(glyphNames)

	glyphCode := make(map[string]int)
	for i, name := range afm.Encoding {
		if name != ".notdef" {
			glyphCode[name] = i
		}
	}

	var F font.Layouter
	var geom *font.Geometry

	curGlyph := 0
	for curGlyph < nGlyph {
		// we need space for at least one line
		err := f.MakeSpace(baseLineSkip)
		if err != nil {
			return nil
		}
		page := f.page

		nRows := int((f.textHeight - f.used) / baseLineSkip)
		if rowsNeeded := (nGlyph - curGlyph + 3) / 4; nRows > rowsNeeded {
			nRows = rowsNeeded
		}

		yTop := f.margin + f.textHeight - f.used - afm.Ascent*fontSize/1000

		// First draw the rectangles for the glyph extents onto the background.
		tmpGlyph := curGlyph
		page.PushGraphicsState()
		page.SetFillColor(color.DeviceRGB(.4, 1, .4))
		for col := 0; col < 4; col++ {
			x := f.margin + float64(col)*colWidth
			for i := 0; i < nRows; i++ {
				if tmpGlyph >= nGlyph {
					break
				}
				y := yTop - baseLineSkip*float64(i)

				gi := afm.Glyphs[glyphNames[tmpGlyph]]
				ext := gi.BBox
				if !ext.IsZero() {
					w := gi.WidthX * fontSize / 1000
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

		// Then draw the glyphs and their codes/names.
		for col := 0; col < 4; col++ {
			page.TextBegin()
			x := f.margin + float64(col)*colWidth
			for i := 0; i < nRows; i++ {
				if curGlyph >= nGlyph {
					break
				}

				if curGlyph%256 == 0 {
					// The builtin fonts are simple fonts, so we can only
					// use up to 256 glyphs for each embedded copy of the
					// font.
					F, err = G.New(nil)
					if err != nil {
						return err
					}
					geom = F.GetGeometry()
				}

				y := yTop - baseLineSkip*float64(i)

				if i == 0 {
					page.TextFirstLine(x, y)
				} else if i == 1 {
					page.TextSecondLine(0, -baseLineSkip)
				} else {
					page.TextNextLine()
				}

				name := glyphNames[curGlyph]
				code := "·"
				if x, ok := glyphCode[name]; ok {
					code = fmt.Sprintf("%d", x)
				}
				page.TextSetFont(f.bodyFont, fontSize)
				page.TextShowAligned(code, 16, 1)

				page.TextSetFont(F, fontSize)
				g := &font.GlyphSeq{
					Seq: []font.Glyph{
						{
							GID:     glyph.ID(curGlyph),
							Advance: fontSize * geom.Widths[curGlyph],
						},
					},
				}
				g.Align(32, 0.5)
				page.TextShowGlyphs(g)

				page.TextSetFont(f.bodyFont, fontSize)
				page.TextShow(name)

				curGlyph++
			}
			page.TextEnd()
		}
		f.used += float64(nRows) * baseLineSkip
	}
	return nil
}
