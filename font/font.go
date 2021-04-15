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

// GlyphIndex is used to enumerate the blyphs in a font.  The first glyph
// has index 0 and is used to indicate a missing character (usually rendered
// as an empty box or similar).
type GlyphIndex uint16

// Font represents a font embedded in the PDF file.
type Font struct {
	Name pdf.Name
	Ref  *pdf.Reference

	CMap      map[rune]GlyphIndex
	Enc       func(...GlyphIndex) []byte
	Ligatures map[GlyphPair]GlyphIndex
	Kerning   map[GlyphPair]int

	GlyphExtent []Rect
	Width       []int

	Ascent  float64 // Ascent in glyph coordinate units
	Descent float64 // Descent in glyph coordinate units, as a negative number
	LineGap float64 // TODO(voss): remove?
}

// Rect represents a rectangle with integer coordinates.
type Rect struct {
	LLx, LLy, URx, URy int
}

// IsZero returns whether the glyph leaves marks on the page.
func (rect *Rect) IsZero() bool {
	return rect.LLx == 0 && rect.LLy == 0 && rect.URx == 0 && rect.URy == 0
}

// GlyphPair represents two consecutive glyphs, specified by a pair of
// character codes.  This is used for ligatures and kerning information.
type GlyphPair [2]GlyphIndex

// Layout contains the information needed to typeset a text.
type Layout struct {
	FontSize  float64
	Fragments [][]byte
	Kerns     []int
	Width     float64
	Height    float64
	Depth     float64
}

// Typeset determines the layout of a string using the given font.  The
// function takes ligatures and kerning information into account.
func (font *Font) Typeset(s string, ptSize float64) *Layout {
	// for _, repl := range ligTab {
	// 	if font.CMap[repl.lig] == 0 {
	// 		continue
	// 	}
	// 	s = strings.ReplaceAll(s, repl.letters, string([]rune{repl.lig}))
	// }

	// Units in an afm file are in 1/1000 of the scale of the font being
	// formatted. Multiplying with the scale factor gives values in 1000*bp.
	q := ptSize / 1000

	var codes []GlyphIndex
	var last GlyphIndex
	for _, r := range s {
		if !unicode.IsGraphic(r) {
			continue
		}
		c := font.CMap[r]
		if len(codes) > 0 {
			pair := GlyphPair{last, c}
			lig, ok := font.Ligatures[pair]
			if ok {
				codes = codes[:len(codes)-1]
				c = lig
			}
		}
		codes = append(codes, c)
		last = c
	}

	ll := &Layout{
		FontSize: ptSize,
	}
	if len(codes) == 0 {
		return ll
	}

	width := 0.0
	height := math.Inf(-1)
	depth := math.Inf(-1)
	pos := 0
	for i, c := range codes {
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

		if i == len(codes)-1 {
			ll.Fragments = append(ll.Fragments, font.Enc(codes[pos:]...))
			break
		}

		kern := font.Kerning[GlyphPair{c, codes[i+1]}]
		if kern == 0 {
			continue
		}

		width += float64(kern) * q
		ll.Fragments = append(ll.Fragments, font.Enc(codes[pos:i+1]...))
		ll.Kerns = append(ll.Kerns, -kern)
		pos = i + 1
	}
	ll.Width = width
	ll.Height = height
	ll.Depth = depth

	return ll
}

var ligTab = []struct {
	letters string
	lig     rune
}{
	{"ffi", '\uFB03'},
	{"ffl", '\uFB04'},
	{"fi", '\uFB01'},
	{"fl", '\uFB02'},
	{"ff", '\uFB00'},
}
