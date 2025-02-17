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
	"log"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/fonttypes"
)

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	doc, err := document.CreateMultiPage("test.pdf", nil, pdf.V1_7, nil)
	if err != nil {
		return err
	}

	H, err := standard.Helvetica.New(nil)
	if err != nil {
		return err
	}

	for _, sample := range fonttypes.All {
		page := doc.AddPage()

		F := sample.MakeFont()

		if sample.Type.IsComposite() {
			drawPage(H, 32, page, F, sample.Description)
		} else {
			drawPage(H, 16, page, F, sample.Description)
		}

		err = page.Close()
		if err != nil {
			return err
		}
	}

	return doc.Close()
}

func drawPage(H font.Font, nRow int, page *document.Page, F font.Layouter, desc string) {
	paper := &pdf.Rectangle{URx: 10 + 16*20, URy: 5 + float64(nRow)*20 + 15}
	page.SetPageSize(paper)

	page.PushGraphicsState()
	page.TextSetFont(F, 16)
	geom := F.GetGeometry()
	gid := glyph.ID(0)
	for i := 0; i < 16*nRow; i++ {
		row := i / 16
		col := i % 16

		for int(gid) < len(geom.GlyphExtents) && geom.GlyphExtents[gid].IsZero() {
			gid++
		}
		if int(gid) >= len(geom.GlyphExtents) {
			break
		}
		w := geom.Widths[gid]
		gg := &font.GlyphSeq{
			Seq: []font.Glyph{
				{
					GID:     gid,
					Advance: 16 * w,
				},
			},
		}
		gg.Align(0, 0.5)
		gid++

		page.TextBegin()
		page.TextFirstLine(float64(5+20*col+10), float64(20*nRow-10-20*row))
		page.TextShowGlyphs(gg)
		page.TextEnd()
	}
	page.PopGraphicsState()

	page.TextSetFont(H, 8)
	page.SetFillColor(color.DeviceGray(0.5))
	page.TextBegin()
	page.TextFirstLine(5, paper.URy-10)
	page.TextShow(desc)
	page.TextEnd()
}
