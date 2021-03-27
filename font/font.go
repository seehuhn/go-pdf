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
	"strings"
	"unicode"

	"seehuhn.de/go/pdf"
)

type GlyphIndex uint16

type Font struct {
	Ref *pdf.Reference

	CMap      map[rune]GlyphIndex
	Enc       func(...GlyphIndex) []byte
	Ligatures map[GlyphPair]GlyphIndex
	Kerning   map[GlyphPair]int

	GlyphExtent []Rect
	Width       []int

	Ascent  float64
	Descent float64
	LineGap float64
}

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

// ---------------------

// OldFont represents information about a PDF font at a given font size.
type OldFont struct {
	BaseFont  string
	FullName  string
	CapHeight float64
	XHeight   float64
	Ascender  float64
	Descender float64

	Encoding Encoding

	Width     map[byte]float64
	BBox      map[byte]*OldRect
	Ligatures map[OldGlyphPair]byte
	Kerning   map[OldGlyphPair]float64
}

// TODO(voss): is the distinction between NewFont and FontFile really useful?

type OtherFont interface {
	GetGlyphs(string) []GlyphIndex
	EncodeGlyph(GlyphIndex) []byte
}

type FontFile interface {
	Close() error
	GetInfo() (*Info, error)
	Embed(w *pdf.Writer, subset map[rune]bool) (*pdf.Reference, OtherFont, error)
}

// Layout contains the information needed to typeset a text.
type Layout struct {
	FontSize  float64
	Fragments [][]byte
	Kerns     []int
	Width     float64
	Depth     float64
	Height    float64
}

// Typeset determines the layout of a string using the given font.  The
// function takes ligatures and kerning information into account.
func (font *Font) Typeset(s string, ptSize float64) *Layout {
	for _, repl := range ligTab {
		if font.CMap[repl.lig] == 0 {
			continue
		}
		s = strings.ReplaceAll(s, repl.letters, string([]rune{repl.lig}))
	}

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
	height := 0.0 // TODO(voss): -oo ?
	depth := 0.0  // TODO(voss): -oo ?
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
			fmt.Println(codes[pos:], "->", font.Enc(codes[pos:]...))
			ll.Fragments = append(ll.Fragments, font.Enc(codes[pos:]...))
			break
		}

		kern := font.Kerning[GlyphPair{c, codes[i+1]}]
		if kern == 0 {
			continue
		}

		width += float64(kern) * q
		fmt.Println(codes[pos : i+1])
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

// OldGlyphPair represents two consecutive glyphs, specified by a pair of
// character codes.  This is used for ligatures and kerning information.
type OldGlyphPair [2]byte

// OldRect represents a rectangle in the PDF coordinate space.
// TODO(voss): replace with pdf.Rectangle
type OldRect struct {
	LLx, LLy, URx, URy float64
}

// IsPrint returns whether the glyph leaves marks on the page.
func (rect *OldRect) IsPrint() bool {
	return rect.LLx != 0 || rect.LLy != 0 || rect.URx != 0 || rect.URy != 0
}
