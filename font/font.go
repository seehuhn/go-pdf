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
	"iter"

	"seehuhn.de/go/postscript/cid"
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

// WritingMode is the "writing mode" of a PDF font (horizontal or vertical).
type WritingMode int

func (m WritingMode) String() string {
	switch m {
	case Horizontal:
		return "horizontal"
	case Vertical:
		return "vertical"
	default:
		return fmt.Sprintf("WritingMode(%d)", m)
	}
}

const (
	// Horizontal indicates horizontal writing mode.
	Horizontal WritingMode = 0

	// Vertical indicates vertical writing mode.
	Vertical WritingMode = 1
)

type Code struct {
	// CID allows to look up the glyph in the underlying font.
	CID cid.CID

	// Notdef specifies which glyph to show if the requested glyph is not
	// present in the font.
	Notdef cid.CID

	// Width is the glyph width in PDF glyph space units.
	Width float64

	// UseWordSpacing whether PDF word spacing should be used for this code.
	// This is true if the character code is a single byte with the value 0x20
	// (irrespective of whether the character actually represents a space).
	UseWordSpacing bool

	// Text is the text representation of the character.
	Text string
}

// Embedded represents a font which is already embedded in a PDF file.
//
// The functions of this interface provide the information required to
// keep track of the current text position in a PDF content stream.
//
// TODO(voss): merge with [EmbeddedLayouter]
type Embedded interface {
	WritingMode() WritingMode

	// Codes iterates over the character codes in a PDF string, yielding
	// information about each character code.
	//
	// The returned pointer points to memory that is reused across iterations.
	// The caller must not modify the pointed-to structure.
	Codes(s pdf.String) iter.Seq[*Code]
}

// EmbeddedLayouter is an embedded font which can typeset new text.
type EmbeddedLayouter interface {
	Embedded

	// AppendEncoded converts a glyph ID (corresponding to the given text) into
	// a PDF character code.  The character code is appended to s. The function
	// returns the new string s and the width of the glyph in PDF text space units
	// (still to be multiplied by the font size).
	//
	// As a side effect, this function may allocate codes for the given
	// glyph/text combination in the font's encoding.
	AppendEncoded(s pdf.String, gid glyph.ID, text string) (pdf.String, float64)
}
