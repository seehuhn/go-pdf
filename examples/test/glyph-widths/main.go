// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/internal/debug"
)

func main() {
	err := run("test.pdf")
	if err != nil {
		log.Fatal(err)
	}
}

func run(filename string) error {
	ff, err := debug.MakeFonts()
	if err != nil {
		return err
	}

	const (
		margin    = 36.0
		colWidth  = 2 * 72.0
		colHeight = 72.0

		testFontSize = 48.0
	)
	paper := &pdf.Rectangle{
		URx: 2*margin + colWidth,
		URy: 2*margin + float64(len(ff))*colHeight,
	}
	page, err := document.CreateSinglePage(filename, paper, nil)
	if err != nil {
		return err
	}

	labelFont, err := type1.Helvetica.Embed(page.Out, "H")
	if err != nil {
		return err
	}

	for i, font := range ff {
		xBase := margin
		yBase := margin + float64(len(ff)-i-1)*colHeight

		page.SetFillColor(color.Gray(0.5))
		page.TextStart()
		page.TextFirstLine(xBase, yBase+colHeight-10)
		page.TextSetFont(labelFont, 8)
		page.TextShow(font.Type.String())
		page.TextEnd()

		F, err := font.Font.Embed(page.Out, pdf.Name(fmt.Sprintf("F%d", i)))
		if err != nil {
			return err
		}

		geom := F.GetGeometry()
		gg := F.Layout("Nimm!")

		// Draw the glyphs.
		page.SetFillColor(color.Gray(0))
		page.TextStart()
		page.TextFirstLine(xBase, yBase+16)
		page.TextSetFont(F, testFontSize)
		totalWidth := page.TextShowGlyphsOld(gg)
		page.TextEnd()

		// Mark the glyph x-positions in blue.
		page.SetStrokeColor(color.RGB(0, 0, 0.8))
		page.SetLineWidth(1)
		xPos := xBase
		page.MoveTo(xPos, yBase+8)
		page.LineTo(xPos, yBase+0.5*colHeight)
		for _, g := range gg {
			xPos += geom.ToPDF16(testFontSize, g.Advance)
			page.MoveTo(xPos, yBase+8)
			page.LineTo(xPos, yBase+16)
		}
		xPos = xBase + totalWidth
		page.MoveTo(xPos, yBase+8)
		page.LineTo(xPos, yBase+0.5*colHeight)
		page.Stroke()

		// Mark the glyph widths in red.
		page.SetStrokeColor(color.RGB(0.8, 0, 0))
		page.SetLineWidth(1)
		xPos = xBase
		for i, g := range gg {
			w := geom.ToPDF16(testFontSize, geom.Widths[g.GID])
			y := yBase + 10
			if i%2 == 0 {
				y += 2
			}
			page.MoveTo(xPos, y)
			page.LineTo(xPos+w, y)
			xPos += geom.ToPDF16(testFontSize, g.Advance)
		}
		page.Stroke()

		// Mark the glyph bounding boxes in green.
		page.SetStrokeColor(color.RGB(0, 0.8, 0))
		page.SetLineWidth(1)
		xPos = xBase
		for _, g := range gg {
			x := xPos + geom.ToPDF16(testFontSize, g.XOffset)
			y := yBase + 16 + geom.ToPDF16(testFontSize, g.YOffset)
			bbox := geom.GlyphExtents[g.GID]
			page.Rectangle(x+geom.ToPDF16(testFontSize, bbox.LLx),
				y+geom.ToPDF16(testFontSize, bbox.LLy),
				geom.ToPDF16(testFontSize, bbox.URx-bbox.LLx),
				geom.ToPDF16(testFontSize, bbox.URy-bbox.LLy))
			xPos += geom.ToPDF16(testFontSize, g.Advance)
		}
		page.Stroke()
	}

	err = page.Close()
	if err != nil {
		return err
	}
	return nil
}
