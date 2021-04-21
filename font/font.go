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
	"math"
	"unicode"

	"seehuhn.de/go/pdf"
)

// Font represents a font embedded in the PDF file.
// TODO(voss): make sure that there is a good way to determine the number
// of glyphs in the font?
type Font struct {
	Name pdf.Name
	Ref  *pdf.Reference

	CMap map[rune]GlyphID
	Enc  func(GlyphID) []byte

	Substitute func(glyphs []GlyphID) []GlyphID
	Layout     func(glyphs []GlyphID) []GlyphPos

	Ligatures map[GlyphPair]GlyphID
	Kerning   map[GlyphPair]int

	GlyphUnits int

	GlyphExtent []Rect
	Width       []int

	Ascent  float64 // Ascent in glyph coordinate units
	Descent float64 // Descent in glyph coordinate units, as a negative number
	LineGap float64 // TODO(voss): remove?
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

type Layout struct {
	Font     *Font
	FontSize float64
	Glyphs   []GlyphPos
}

type GlyphPos struct {
	Gid     GlyphID
	XOffset int
	YOffset int
	Advance int
}

func (font *Font) TypeSet(s string, ptSize float64) *Layout {
	var runs [][]rune
	var run []rune
	for _, r := range s {
		if unicode.IsGraphic(r) {
			run = append(run, r)
		} else if len(run) > 0 {
			runs = append(runs, run)
			run = nil
		}
	}
	if len(run) > 0 {
		runs = append(runs, run)
		run = nil
	}

	// introduce ligatures etc.
	var glyphs []GlyphID
	for _, run := range runs {
		pos := len(glyphs)
		for _, r := range run {
			glyphs = append(glyphs, font.CMap[r])
		}
		glyphs = append(glyphs[:pos], font.Substitute(glyphs[pos:])...)
	}

	layout := font.Layout(glyphs)
	return &Layout{
		Font:     font,
		FontSize: ptSize,
		Glyphs:   layout,
	}
}

// GlyphPair represents two consecutive glyphs, specified by a pair of
// character codes.  This is used for ligatures and kerning information.
type GlyphPair [2]GlyphID

// OldLayout contains the information needed to typeset a text.
type OldLayout struct {
	FontSize  float64
	Fragments [][]byte
	Kerns     []int
	Width     float64
	Height    float64
	Depth     float64
}

// OldTypeset determines the layout of a string using the given font.  The
// function takes ligatures and kerning information into account.
func (font *Font) OldTypeset(s string, ptSize float64) *OldLayout {
	// for _, repl := range ligTab {
	// 	if font.CMap[repl.lig] == 0 {
	// 		continue
	// 	}
	// 	s = strings.ReplaceAll(s, repl.letters, string([]rune{repl.lig}))
	// }

	q := ptSize / float64(font.GlyphUnits)

	var glyphs []GlyphID
	var last GlyphID
	for _, r := range s {
		if !unicode.IsGraphic(r) {
			continue
		}
		c := font.CMap[r]
		if len(glyphs) > 0 {
			pair := GlyphPair{last, c}
			lig, ok := font.Ligatures[pair]
			if ok {
				glyphs = glyphs[:len(glyphs)-1]
				c = lig
			}
		}
		glyphs = append(glyphs, c)
		last = c
	}

	ll := &OldLayout{
		FontSize: ptSize,
	}
	if len(glyphs) == 0 {
		return ll
	}

	width := 0.0
	height := math.Inf(-1)
	depth := math.Inf(-1)
	pos := 0
	for i, c := range glyphs {
		bbox := &font.GlyphExtent[c]
		if !bbox.IsZero() {
			thisDepth := -float64(bbox.LLy) * q
			if thisDepth > depth {
				depth = thisDepth
			}
			thisHeight := float64(bbox.URy) * q
			if thisHeight > height {
				height = thisHeight
			}
		}
		width += float64(font.Width[c]) * q

		if i == len(glyphs)-1 {
			var enc []byte
			for _, gid := range glyphs[pos:] {
				enc = append(enc, font.Enc(gid)...)
			}
			ll.Fragments = append(ll.Fragments, enc)
			break
		}

		kern := font.Kerning[GlyphPair{c, glyphs[i+1]}]
		if kern == 0 {
			continue
		}

		width += float64(kern) * q
		var enc []byte
		for _, gid := range glyphs[pos : i+1] {
			enc = append(enc, font.Enc(gid)...)
		}
		ll.Fragments = append(ll.Fragments, enc)
		ll.Kerns = append(ll.Kerns, -kern)
		pos = i + 1
	}
	ll.Width = width
	ll.Height = height
	ll.Depth = depth

	return ll
}
