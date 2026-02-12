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

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
)

func main() {
	err := createDocument("test.pdf")
	if err != nil {
		log.Fatal(err)
	}
}

func createDocument(filename string) error {
	paper := document.A4
	doc, err := document.CreateMultiPage(filename, paper, pdf.V1_7, nil)
	if err != nil {
		return err
	}

	F := standard.TimesRoman.New()
	geom := F.GetGeometry()

	const (
		k        = 20
		left     = 100
		fontSize = 18
		gap1     = 50
		gap2     = 380
	)

	ascent := geom.Ascent * fontSize
	descent := geom.Descent * fontSize
	leading := ascent - descent

	markerWidth := 1.0 / fontSize * 1000
	markerAscent := ascent / fontSize * 1000
	markerDescent := descent / fontSize * 1000

	// Build content stream for marker glyph
	b := builder.New(content.Glyph, nil)
	b.Type3UncoloredGlyph(markerWidth, 0, 0, markerDescent, markerWidth, markerAscent)
	b.Rectangle(0, markerDescent, markerWidth, markerAscent-markerDescent)
	b.Fill()
	stream, err := b.Harvest()
	if err != nil {
		return err
	}

	markerFont := &type3.Font{
		Glyphs: []*type3.Glyph{
			{},
			{
				Name:    "I",
				Content: stream,
			},
		},
		FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
	}
	M, err := markerFont.New()
	if err != nil {
		return err
	}

	gid := glyph.ID(0)
	numGlyphs := min(glyph.ID(len(geom.Widths)), 256)

	for gid < numGlyphs {
		page := doc.AddPage()

		page.PushGraphicsState()
		page.SetLineWidth(0.5)

		page.SetStrokeColor(color.DeviceGray(0.85))
		for w := 0.0; w < 1000.1; w += 10 {
			x := left + k*fontSize*w/1000 + gap1 + 0.5
			page.MoveTo(x, paper.LLy)
			page.LineTo(x, paper.URy)
		}
		page.Stroke()

		page.SetStrokeColor(color.DeviceGray(0.7))
		for w := 0.0; w < 1000.1; w += 100 {
			x := left + k*fontSize*w/1000 + gap1 + 0.5
			page.MoveTo(x, paper.LLy)
			page.LineTo(x, paper.URy)
		}
		page.Stroke()

		page.SetLineWidth(1)
		page.SetStrokeColor(color.DeviceRGB{1, 0.5, 0.5})
		x := left + gap1 + 1 + gap2 + 0.5
		page.MoveTo(x, paper.LLy)
		page.LineTo(x, paper.URy)
		page.Stroke()
		page.PopGraphicsState()

		page.TextBegin()
		yPos := paper.URy - 10 - ascent
		for i := 0; yPos+descent >= 10 && gid < numGlyphs; i++ {
			switch i {
			case 0:
				page.TextFirstLine(left, yPos)
			case 1:
				page.TextSecondLine(0, -leading)
			default:
				page.TextNextLine()
			}
			page.TextSetFont(F, fontSize)
			glyphWidth := fontSize * geom.Widths[gid]
			gg := &font.GlyphSeq{
				Seq: make([]font.Glyph, k),
			}
			for j := range k {
				gg.Seq[j] = font.Glyph{
					GID:     gid,
					Advance: glyphWidth,
				}
			}
			gg.Seq[k-1].Advance += gap1
			page.TextShowGlyphs(gg)
			page.TextSetFont(M, fontSize)
			// TODO(voss): why is the final +1 needed?
			page.TextShowAligned("I", gap2-k*glyphWidth+1, 0)
			page.TextShow("I")

			gid++
			yPos -= leading
		}
		page.TextEnd()

		err = page.Close()
		if err != nil {
			return err
		}
	}

	return doc.Close()
}
