// Copyright 2023 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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

	titleFont, err := standard.TimesBold.New()
	if err != nil {
		return err
	}

	for _, G := range standard.All {
		probe, err := G.New()
		if err != nil {
			return err
		}
		// glyph IDs of probe are valid in any other instance of G, because
		// the glyph ID assignment is deterministic per font
		var gids []glyph.ID
		for gid := range probe.GlyphNames {
			gids = append(gids, glyph.ID(gid))
		}

		for len(gids) > 0 {
			n := min(len(gids), maxCodes)
			err = addPages(doc, paper, titleFont, G, gids[:n])
			if err != nil {
				return err
			}
			gids = gids[n:]
		}
	}

	return doc.Close()
}

// maxCodes is the maximum number of glyphs in one simple font.
const maxCodes = 256

// addPages adds pages which show the given glyphs of a freshly created
// instance of the standard font G.
func addPages(doc *document.MultiPage, paper *pdf.Rectangle, titleFont font.Instance, G standard.Font, gids []glyph.ID) error {
	const (
		k        = 20
		left     = 100
		fontSize = 18
		gap1     = 50
		gap2     = 380
	)

	F, err := G.New()
	if err != nil {
		return err
	}
	geom := F.GetGeometry()

	ascent := geom.Ascent * fontSize
	descent := geom.Descent * fontSize
	leading := ascent - descent

	markerWidth := 1.0 / fontSize * 1000
	markerAscent := ascent / fontSize * 1000
	markerDescent := descent / fontSize * 1000

	// Build content stream for marker glyph
	b := builder.New(content.Glyph, nil, pdf.V2_0)
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

	title := string(G)

	for len(gids) > 0 {
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
		page.TextSetFont(titleFont, 10)
		page.TextFirstLine(left, paper.URy-20)
		page.TextShow(title)
		page.TextEnd()

		page.TextBegin()
		yPos := paper.URy - 30 - ascent
		for i := 0; yPos+descent >= 10 && len(gids) > 0; i++ {
			gid := gids[0]
			gids = gids[1:]

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

			yPos -= leading
		}
		page.TextEnd()

		err = page.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
