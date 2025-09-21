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

// == NEW API=================================================================

type InstanceNew interface {
	WritingMode() WritingMode

	// GetCodec returns the character code codec for the encoding used by this
	// font.
	GetCodec() *charcode.Codec

	Codes(s pdf.String) iter.Seq[*Code]

	// GetName returns a human-readable name for the font.
	// For most font types, this is the PostScript name of the font.
	GetName() string

	pdf.Embedder[pdf.Unused]
}

type EncoderNew interface {
	InstanceNew

	// Encode converts a glyph ID to a character code (for use with the
	// instance's codec).  The arguments width and text are hints for choosing
	// an appropriate advance width and text representation for the character
	// code, in case a new code is allocated.
	//
	// The function returns the character code, the PDF advance width and a
	// boolean indicating whether the encoding was successful.  If the function
	// returns false, the glyph ID cannot be encoded with this font instance.
	//
	// Use the Codec to append the character code to PDF strings. The returned
	// width is the advance width from the PDF font dictionary, and if a
	// pre-existing code is re-used this may be different from the width
	// argument.
	Encode(gid glyph.ID, width float64, text string) (charcode.Code, float64, bool)

	// Capacity returns the number of character codes that can still be
	// allocated in this font instance.
	Capacity() int
}

type LayouterNew interface {
	EncoderNew

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

// == OLD API=================================================================

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
	// If seq is nil, a new glyph sequence is allocated.  If seq is not
	// nil, the return value is guaranteed to be equal to seq.
	Layout(seq *GlyphSeq, ptSize float64, s string) *GlyphSeq
}

// Embedded represents a font which is already embedded in a PDF file.
//
// The functions of this interface provide the information required to
// keep track of the current text position in a PDF content stream.
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

// FromFile represents an immutable font read from a PDF file.
type FromFile interface {
	Font
	Embedded

	// GetDict returns the font dictionary of this font.
	GetDict() Dict
}

// Dict represents a font dictionary in a PDF file.
//
// This interface is implemented by the following types, corresponding to the
// different font dictionary types supported by PDF:
//   - [seehuhn.de/go/pdf/font/dict.Type1]
//   - [seehuhn.de/go/pdf/font/dict.TrueType]
//   - [seehuhn.de/go/pdf/font/dict.Type3]
//   - [seehuhn.de/go/pdf/font/dict.CIDFontType0]
//   - [seehuhn.de/go/pdf/font/dict.CIDFontType2]
type Dict interface {
	// WriteToPDF adds this font dictionary to the PDF file using the given
	// reference.
	//
	// The resource manager is used to deduplicate child objects
	// like encoding dictionaries, CMap streams, etc.
	WriteToPDF(*pdf.ResourceManager, pdf.Reference) error

	// MakeFont returns a new font object that can be used to typeset text.
	// The font is immutable, i.e. no new glyphs can be added and no new codes
	// can be defined via the returned font object.
	MakeFont() FromFile

	// FontInfo returns information about the embedded font file.
	// The information can be used to load the font file and to extract
	// the the glyph corresponding to a character identifier.
	// The result is a pointer to one of the FontInfo* types
	// defined in the font/dict package.
	FontInfo() any

	// Codec allows to interpret character codes for the font.
	Codec() *charcode.Codec

	Characters() iter.Seq2[charcode.Code, Code]
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

	// Text is the textual representation of the character.
	Text string

	// UseWordSpacing indicates whether PDF word spacing is added for this
	// code. This is true if and only if the character code is a single byte
	// with the value 0x20 (irrespective of whether the character actually
	// represents a space).
	UseWordSpacing bool
}
