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
	"iter"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
)

// A Layouter is a font which can typeset new text.
//
// Fonts which implement this interface need to contain the following
// information:
//   - How to convert characters to Glyph IDs
//   - Kerning and Ligature information, if applicable
//   - Global font metrics, e.g. ascent, descent, line height
//   - Glyph metrics: advance width, bounding box
type Layouter interface {
	Font

	// GetGeometry returns font metrics required for typesetting.
	GetGeometry() *Geometry

	// Layout appends a string to a glyph sequence.  The string is typeset at
	// the given point size and the resulting GlyphSeq is returned.
	//
	// If seq is non-nil, a new glyph sequence is allocated.  If seq is not
	// nil, the return value is guaranteed to be equal to seq.
	Layout(seq *GlyphSeq, ptSize float64, s string) *GlyphSeq
}

// Font represents a font instance which can be embedded in a PDF file.
//
// This interface implements [pdf.Embedder] and font objects are normally
// embedded using [pdf.ResourceManagerEmbed].  As a consequence, each font
// instance is embedded into a PDF file only once.  If more than one embedded
// copy is required, separate Font instances must be used.
type Font interface {
	// PostScriptName returns the PostScript name of the font.
	PostScriptName() string

	pdf.Embedder[Embedded]
}

// Embedded represents a font which is already embedded in a PDF file.
//
// The functions of this interface provide the information required to
// convert Glyph IDs into PDF character codes, and to keep track of the
// current text position in a PDF content stream.
type Embedded interface {
	WritingMode() WritingMode

	ForeachWidth(s pdf.String, yield func(width float64, isSpace bool))

	// CodeAndWidth converts a glyph ID (corresponding to the given text) into
	// a PDF character code The character code is appended to s. The function
	// returns the new string s, the width of the glyph in PDF text space units
	// (still to be multiplied by the font size), and a value indicating
	// whether PDF word spacing adjustment applies to this glyph.
	//
	// As a side effect, this function may allocate codes for the given
	// glyph/text combination in the font's encoding.
	CodeAndWidth(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64, bool)
}

// WritingMode is the "writing mode" of a PDF font (horizontal or vertical).
type WritingMode int

const (
	// Horizontal indicates horizontal writing mode.
	Horizontal WritingMode = iota

	// Vertical indicates vertical writing mode.
	Vertical
)

type EmbeddedNew interface {
	WritingMode() WritingMode

	// Width returns the width corresponding to a CID (for composite fonts) or
	// a character code (for simple fonts).  The width is given in PDF text
	// space units.
	Width(cid CID) float64

	// Code converts a Glyph ID (with corresponding text) into a PDF character code.
	Code(gid glyph.ID, rr []rune) CID

	// Append appends the character code for a CID to a string.
	Append(s pdf.String, cid CID) pdf.String

	// AllCharacters iterates over all character codes in the font.
	AllCharacters(s pdf.String) iter.Seq[CID]
}

// CID represents a character ID in a composite font, or a (single byte)
// character code in a simple font.
type CID uint32
