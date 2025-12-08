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

	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
)

type Instance interface {
	// PostScriptName returns a human-readable name for the font.
	// For most font types, this is the PostScript name of the font.
	PostScriptName() string

	// WritingMode returns whether the font is used for horizontal or vertical
	// writing.
	WritingMode() WritingMode

	// Codec returns the codec for the encoding used by this font.
	Codec() *charcode.Codec

	// Codes iterates over the character codes in a PDF string, yielding
	// information about each character code.
	//
	// The returned pointer points to memory that is reused across iterations.
	// The caller must not modify the pointed-to structure.
	Codes(s pdf.String) iter.Seq[*Code]

	// FontInfo returns information required to load the font file and to
	// extract the the glyph corresponding to a character identifier. The
	// result is a pointer to one of the FontInfo* types defined in the
	// font/dict package.
	FontInfo() any

	pdf.Embedder
}

type Layouter interface {
	Instance

	// Encode converts a glyph ID to a character code (for use with the
	// instance's codec).  The text argument is a hint for choosing
	// an appropriate text representation for the character code, in case
	// a new code is allocated.  The glyph's width is taken from the font.
	//
	// The function returns the character code, and a boolean indicating
	// whether the encoding was successful.  If the function returns false, the
	// glyph ID cannot be encoded with this font instance.
	//
	// Use the Codec to append the character code to PDF strings.
	Encode(gid glyph.ID, text string) (charcode.Code, bool)

	// CodesRemaining returns the number of character codes that can still be
	// allocated in this font instance.
	CodesRemaining() int

	// Layout appends a string to a glyph sequence.  The string is typeset at
	// the given point size and the resulting GlyphSeq is returned.
	//
	// If seq is nil, a new glyph sequence is allocated.  If seq is not
	// nil, the return value is guaranteed to be equal to seq.
	Layout(seq *GlyphSeq, ptSize float64, s string) *GlyphSeq

	// GetGeometry returns font metrics required for typesetting.
	GetGeometry() *Geometry

	// IsBlank reports whether the glyph with the given ID is a blank glyph.
	IsBlank(gid glyph.ID) bool
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

// InstancesEqual compares two font instances for semantic equality.
// Two fonts are equal if they have the same PostScript name, writing mode,
// and code space range.
func InstancesEqual(a, b Instance) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	if a.PostScriptName() != b.PostScriptName() {
		return false
	}

	if a.WritingMode() != b.WritingMode() {
		return false
	}

	codecA, codecB := a.Codec(), b.Codec()
	if (codecA == nil) != (codecB == nil) {
		return false
	}
	if codecA != nil && !codecA.CodeSpaceRange().Equivalent(codecB.CodeSpaceRange()) {
		return false
	}

	return true
}

type Code struct {
	// CID allows to look up the glyph in the underlying font.
	CID cid.CID

	// Notdef specifies which glyph to show if the requested glyph is not
	// present in the font.
	Notdef cid.CID

	// Width is the glyph width in PDF text space units.
	// This still needs to be scaled by the font size.
	Width float64

	// Text is the textual representation of the character.
	Text string

	// UseWordSpacing indicates whether PDF word spacing is added for this
	// code. This is true if and only if the character code is a single byte
	// with the value 0x20 (irrespective of whether the character actually
	// represents a space).
	UseWordSpacing bool
}
