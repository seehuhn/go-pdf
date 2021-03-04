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
	"errors"
	"strings"
	"unicode"
)

// Font represents information about a PDF font at a given font size.
type Font struct {
	BaseFont  string
	FullName  string
	FontSize  float64
	CapHeight float64
	XHeight   float64
	Ascender  float64
	Descender float64

	Encoding Encoding

	Width        map[byte]float64
	MissingWidth float64
	BBox         map[byte]*Rect
	Ligatures    map[GlyphPair]byte
	Kerning      map[GlyphPair]float64
}

// Rect represents a rectangle in the PDF coordinate space.
type Rect struct {
	LLx, LLy, URx, URy float64
}

// IsPrint returns whether the glyph leaves marks on the page.
func (rect *Rect) IsPrint() bool {
	return rect.LLx != 0 || rect.LLy != 0 || rect.URx != 0 || rect.URy != 0
}

// Layout contains the information needed to typeset a text.
type Layout struct {
	FontSize  float64
	Fragments [][]byte
	Kerns     []float64
	Width     float64
	Depth     float64
	Height    float64
}

// TypeSet determines the layout of a string using a given font.  The function
// takes ligatures and kerning information into account.  If the font cannot
// represent all runes in the string, an error is returned.
func (font *Font) TypeSet(s string, ptSize float64) (*Layout, error) {
	for _, repl := range ligTab {
		_, ok := font.Encoding.Encode(repl.lig)
		if !ok {
			continue
		}
		s = strings.ReplaceAll(s, repl.letters, string([]rune{repl.lig}))
	}

	// Units in an afm file are in 1/1000 of the scale of the font being
	// formatted. Multiplying with the scale factor gives values in 1000*bp.
	q := ptSize / 1000

	var codes []byte
	var last byte
	for _, r := range s {
		if !unicode.IsGraphic(r) {
			continue
		}

		c, ok := font.Encoding.Encode(r)
		if !ok {
			return nil, errors.New("missing glyph for rune " + string([]rune{r}))
		}
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
		return ll, nil
	}

	width := 0.0
	height := 0.0 // TODO(voss): -oo ?
	depth := 0.0  // TODO(voss): -oo ?
	pos := 0
	for i, c := range codes {
		bbox := font.BBox[c]
		if bbox.IsPrint() {
			thisDepth := -bbox.LLy * q
			if thisDepth > depth {
				depth = thisDepth
			}
			thisHeight := bbox.URy * q
			if thisHeight > height {
				height = thisHeight
			}
		}
		width += font.Width[c] * q

		if i == len(codes)-1 {
			ll.Fragments = append(ll.Fragments, codes[pos:])
			break
		}

		kern, ok := font.Kerning[GlyphPair{c, codes[i+1]}]
		if !ok {
			continue
		}

		width += kern * q
		ll.Fragments = append(ll.Fragments, codes[pos:(i+1)])
		ll.Kerns = append(ll.Kerns, -kern)
		pos = i + 1
	}
	ll.Width = width
	ll.Height = height
	ll.Depth = depth

	return ll, nil
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

// GlyphPair represents two consecutive glyphs, specified by a pair of
// character codes.  This is used for ligatures and kerning information.
type GlyphPair [2]byte
