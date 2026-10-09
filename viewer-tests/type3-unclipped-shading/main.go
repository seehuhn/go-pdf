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
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/shading"
	"seehuhn.de/go/pdf/graphics/text"
)

func main() {
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

const (
	margin    = 72.0
	wrapWidth = 450.0
)

func createDocument(filename string) error {
	page, err := document.CreateSinglePage(filename, document.A4, pdf.V1_7, nil)
	if err != nil {
		return err
	}

	type3Font, err := makeType3Font()
	if err != nil {
		return err
	}
	titleFont := font.Must(standard.TimesBold.New())
	title := text.F{Font: titleFont, Size: 10, Color: color.DeviceGray(0.2)}
	noteFont := font.Must(standard.TimesRoman.New())
	note := text.F{Font: noteFont, Size: 10, Color: color.DeviceGray(0.1)}

	// The Type 3 glyph comes first, with no clipping path in force.  The
	// glyph is coloured, so the magenta fill colour must not show.
	page.SetFillColor(color.DeviceRGB{1, 0, 1})
	page.TextBegin()
	page.TextSetFont(type3Font, 20)
	page.TextFirstLine(margin, 700)
	page.TextShow("a")
	page.TextEnd()

	text.Show(page.Builder,
		text.M{X: margin, Y: 812},
		title, "Unbounded shading in a Type 3 glyph", text.NL,
		note, text.Wrap(wrapWidth,
			"The first operation on this page shows one glyph of a Type 3",
			"font, with no clipping path set.  The glyph procedure starts",
			"with d0 and consists of a single sh operator, which paints an",
			"axial shading in light green.  The shading has Extend [true",
			"true] and no BBox, so it is unbounded.  The glyph declares no",
			"bounding box, and the FontBBox is [0 0 0 0].  This text is",
			"shown afterwards.",
		),
		text.NL,
		title, "How to read this page", text.NL,
		note, text.Wrap(wrapWidth,
			"An unbounded shading paints the entire clipping region.  A",
			"viewer which follows the specification shows the whole page",
			"in light green, with this text on top.  A viewer which shows",
			"a white page, or green only in a small area around the",
			"glyph, does not use the page as the clipping region here.",
		),
		text.NL,
		note, text.Wrap(wrapWidth,
			"Before the glyph is shown, the fill colour is set to magenta.",
			"The glyph starts with d0, so it sets its own colours and the",
			"magenta must not appear.  A viewer which fills the page in",
			"magenta treats the glyph as an uncoloured stencil, as if it",
			"started with d1.",
		),
	)

	return page.Close()
}

// makeType3Font returns a Type 3 font with one glyph "a".  The glyph
// procedure only paints an unbounded shading.
func makeType3Font() (font.Instance, error) {
	b := builder.New(content.Glyph, nil, pdf.V1_7)
	b.Type3ColoredGlyph(1000, 0)
	b.DrawShading(&shading.Type2{
		Common: shading.Common{ColorSpace: color.SpaceDeviceRGB},
		P0:     vec.Vec2{X: 0, Y: 0},
		P1:     vec.Vec2{X: 1000, Y: 0},
		F: &function.Type2{
			XMin: 0,
			XMax: 1,
			C0:   []float64{0.8, 1, 0.8},
			C1:   []float64{0.8, 1, 0.8},
			N:    1,
		},
		TMin:        0,
		TMax:        1,
		ExtendStart: true,
		ExtendEnd:   true,
		SingleUse:   true,
	})
	stream, err := b.Harvest()
	if err != nil {
		return nil, err
	}
	F := &type3.Font{
		Glyphs: []*type3.Glyph{
			{}, // .notdef
			{Name: "a", Content: stream},
		},
		Resources:  b.Resources,
		FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
	}
	return F.New()
}
