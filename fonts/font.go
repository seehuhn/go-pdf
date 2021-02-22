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

package fonts

// Box represents a rectangle in the PDF coordinate space.
type Box struct {
	LLx, LLy, URx, URy float64
}

// IsPrint returns whether the glyph makes marks on the page.
func (box *Box) IsPrint() bool {
	return box.LLx != 0 || box.LLy != 0 || box.URx != 0 || box.URy != 0
}

// Font represents information about a PDF font.
type Font struct {
	FontName  string
	FullName  string
	CapHeight float64
	XHeight   float64
	Ascender  float64
	Descender float64
	Encoding  Encoding
	Width     map[byte]float64
	BBox      map[byte]*Box
	Ligatures map[GlyphPair]byte
	Kerning   map[GlyphPair]float64
}

// GlyphPair represents two consecutive glyphs, specified by a pair of
// character codes.  This is used for detecting ligatures and computing kerning
// information.
type GlyphPair [2]byte
