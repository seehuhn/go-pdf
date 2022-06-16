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

package font

import (
	"fmt"
	"math"
	"unicode"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/funit"
	"seehuhn.de/go/pdf/pages"
)

// Font represents a font embedded in the PDF file.
//
// TODO(voss): turn this into an interface?
type Font struct {
	InstName pdf.Name
	Ref      *pdf.Reference

	Layout func([]rune) []Glyph
	Enc    func(GlyphID) pdf.String

	UnitsPerEm         uint16
	Ascent             funit.Int16
	Descent            funit.Int16 // negative
	BaseLineSkip       funit.Int16 // PDF glyph space units
	UnderlinePosition  funit.Int16
	UnderlineThickness funit.Int16

	GlyphExtents []funit.Rect
	Widths       []funit.Int16
}

// NumGlyphs returns the number of glyphs in a font.
func (font *Font) NumGlyphs() int {
	return len(font.Widths)
}

// GlyphID is used to enumerate the glyphs in a font.  The first glyph
// has index 0 and is used to indicate a missing character (usually rendered
// as an empty box).
type GlyphID uint16

// Glyph contains layout information for a single glyph in a run
type Glyph struct {
	Gid     GlyphID
	Text    []rune
	XOffset funit.Int16
	YOffset funit.Int16
	Advance funit.Int16
}

// GlyphPair represents two consecutive glyphs, specified by a pair of
// character codes.  This is used for ligatures and kerning information.
type GlyphPair struct {
	Left  GlyphID
	Right GlyphID
}

func isPrivateRange(r rune) bool {
	return r >= '\uE000' && r <= '\uF8FF' ||
		r >= '\U000F0000' && r <= '\U000FFFFD' ||
		r >= '\U00100000' && r <= '\U0010FFFD'
}

// Typeset computes all glyph and layout information required to typeset a
// string in a PDF file.
func (font *Font) Typeset(s string, ptSize float64) *Layout {
	var runs [][]rune
	var run []rune
	for _, r := range s {
		if unicode.IsGraphic(r) || isPrivateRange(r) {
			run = append(run, r)
		} else if len(run) > 0 {
			runs = append(runs, run)
			run = nil
		}
	}
	if len(run) > 0 {
		runs = append(runs, run)
	}

	var glyphs []Glyph
	for _, run := range runs {
		seq := font.Layout(run)
		glyphs = append(glyphs, seq...)
	}

	return &Layout{
		Font:     font,
		FontSize: ptSize,
		Glyphs:   glyphs,
	}
}

// Layout contains the information needed to typeset a run of text.
type Layout struct {
	Font     *Font
	FontSize float64
	Glyphs   []Glyph
}

// Draw shows the text layout on a page.
//
// TODO(voss): This should maybe not use pages.Page for the first argument.
func (layout *Layout) Draw(page *pages.Page, xPos float64, yPos float64) {
	font := layout.Font

	page.Println("BT")
	_ = font.InstName.PDF(page)
	fmt.Fprintf(page, " %f Tf\n", layout.FontSize)
	fmt.Fprintf(page, "%f %f Td\n", xPos, yPos)

	var run pdf.String
	var data pdf.Array
	flushRun := func() {
		if len(run) > 0 {
			data = append(data, run)
			run = nil
		}
	}
	flush := func() {
		flushRun()
		if len(data) == 0 {
			return
		}
		if len(data) == 1 {
			if s, ok := data[0].(pdf.String); ok {
				_ = s.PDF(page)
				page.Println(" Tj")
				data = nil
				return
			}
		}
		_ = data.PDF(page)
		page.Println(" TJ")
		data = nil
	}

	xOffsFont := 0
	yOffs := 0
	xOffsPDF := 0
	for _, glyph := range layout.Glyphs {
		gid := glyph.Gid
		if int(gid) >= len(font.Widths) {
			gid = 0
		}

		if int(glyph.YOffset) != yOffs {
			flush()
			page.Printf("%.1f Ts\n", glyph.YOffset.AsFloat(1/float64(font.UnitsPerEm)))
			yOffs = int(glyph.YOffset)
		}

		xOffsWanted := xOffsFont + int(glyph.XOffset)

		delta := xOffsWanted - xOffsPDF
		if delta != 0 {
			flushRun()
			deltaScaled := float64(delta) / float64(font.UnitsPerEm) * 1000
			data = append(data, -pdf.Integer(math.Round(deltaScaled)))
			xOffsPDF += delta
		}
		run = append(run, font.Enc(gid)...)

		xOffsFont += int(glyph.Advance)
		xOffsPDF = xOffsWanted + int(font.Widths[gid])
	}
	flush()

	page.Println("ET")
}
