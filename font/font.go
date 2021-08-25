// seehuhn.de/go/pdf - support for reading and writing PDF files
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
	"unicode"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pages"
)

// Font represents a font embedded in the PDF file.
type Font struct {
	Name pdf.Name
	Ref  *pdf.Reference

	CMap   map[rune]GlyphID
	Layout func([]Glyph) []Glyph
	Enc    func(GlyphID) pdf.String

	GlyphUnits int
	Ascent     float64 // Ascent in glyph coordinate units
	Descent    float64 // Descent in glyph coordinate units, as a negative number

	GlyphExtent []Rect // TODO(voss): needed?
	Width       []int  // TODO(voss): needed?
}

// MakeGlyphs converts a string to a series of glyphs.  Use the function
// font.Layout() to substitute ligatures and to apply kerning to the result.
// If the font cannot represent all runes in s, an error is returned.
func (font *Font) MakeGlyphs(s string) ([]Glyph, error) {
	rr := []rune(s)
	gg := make([]Glyph, len(rr))
	for i, r := range rr {
		gid, ok := font.CMap[r]
		if !ok {
			return nil, fmt.Errorf("font %q cannot encode rune %04x %q",
				font.Name, r, string([]rune{r}))
		}
		gg[i].Gid = gid
		gg[i].Chars = []rune{r}
		gg[i].Advance = font.Width[gid]
	}
	return gg, nil
}

// Draw emits PDF text mode commands to show the glyphs on the page.
// This must be used between BT and ET, with the correct font already
// set up.
func (font *Font) Draw(page *pages.Page, glyphs []Glyph) {
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
				s.PDF(page)
				page.Println(" Tj")
				data = nil
				return
			}
		}
		data.PDF(page)
		page.Println(" TJ")
		data = nil
	}

	xOffsAuto := 0
	xOffs := 0
	yOffs := 0
	for _, glyph := range glyphs {
		if glyph.YOffset != yOffs {
			flush()
			yOffs = glyph.YOffset
			page.Printf("%d Ts\n", yOffs)
		}

		xOffsWanted := xOffs + glyph.XOffset

		if xOffsWanted != xOffsAuto {

			flushRun()
			data = append(data, -pdf.Integer(xOffsWanted-xOffsAuto))
		}
		run = append(run, font.Enc(glyph.Gid)...)

		xOffs += glyph.Advance
		xOffsAuto = xOffsWanted + font.Width[glyph.Gid]
	}
	flush()
}

// GlyphID is used to enumerate the glyphs in a font.  The first glyph
// has index 0 and is used to indicate a missing character (usually rendered
// as an empty box).
type GlyphID uint16

// Rect represents a rectangle with integer coordinates.
type Rect struct {
	LLx, LLy, URx, URy int
}

// IsZero returns whether the glyph leaves marks on the page.
func (rect *Rect) IsZero() bool {
	return rect.LLx == 0 && rect.LLy == 0 && rect.URx == 0 && rect.URy == 0
}

// Glyph contains layout information for a single glyph in a run
type Glyph struct {
	Gid     GlyphID
	Chars   []rune
	XOffset int
	YOffset int
	Advance int
}

// GlyphPair represents two consecutive glyphs, specified by a pair of
// character codes.  This is used for ligatures and kerning information.
type GlyphPair [2]GlyphID

func isPrivateRange(r rune) bool {
	return r >= '\uE000' && r <= '\uF8FF' ||
		r >= '\U000F0000' && r <= '\U000FFFFD' ||
		r >= '\U00100000' && r <= '\U0010FFFD'
}

// Typeset computes all glyph and layout information required to typeset a
// string in a PDF file.
func (font *Font) Typeset(s string, ptSize float64) (*Layout, error) {
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
		gg, err := font.MakeGlyphs(string(run))
		if err != nil {
			return nil, err
		}
		gg = font.Layout(gg)
		glyphs = append(glyphs, gg...)
	}

	return &Layout{
		Font:     font,
		FontSize: ptSize,
		Glyphs:   glyphs,
	}, nil
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
	page.Println("BT")
	layout.Font.Name.PDF(page)
	fmt.Fprintf(page, " %f Tf\n", layout.FontSize)
	fmt.Fprintf(page, "%f %f Td\n", xPos, yPos)

	layout.Font.Draw(page, layout.Glyphs)

	page.Println("ET")
}
