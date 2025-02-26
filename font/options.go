// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/sfnt/glyph"
)

// Options allows to customize fonts for embedding into PDF files.
// Not all fields apply to all font types.
// type Options struct {
// 	Language language.Tag

// 	GsubFeatures map[string]bool
// 	GposFeatures map[string]bool

// 	// Composite specifies whether to embed the font as a composite font.
// 	Composite bool

// 	// WritingMode gives the writing direction (horizontal or vertical)
// 	// for the font.  Vertical writing is only possible with composite fonts.
// 	WritingMode WritingMode

// 	MakeGIDToCID func() GIDToCID // only used for composite fonts

// 	MakeEncoder func(cid0Width float64, wMode WritingMode) CIDEncoder // only used for composite fonts
// }

// GIDToCID encodes a mapping from Glyph Identifier (GID) values to Character
// Identifier (CID) values.
type GIDToCID interface {
	// TODO(voss): change the second argument to string
	CID(glyph.ID, []rune) cid.CID

	GID(cid.CID) glyph.ID

	ROS() *CIDSystemInfo

	GIDToCID(numGlyph int) []cid.CID
}

// A CIDEncoder maps character codes to CIDs, glyph widths and text content.
type CIDEncoder interface {
	// WritingMode indicates whether the font is for horizontal or vertical
	// writing.
	WritingMode() WritingMode

	// Codes iterates over the character codes in a PDF string.
	// The iterator returns the information stored for each code.
	Codes(s pdf.String) iter.Seq[*Code]

	// MappedCodes iterates over all codes known to the encoder.
	MappedCodes() iter.Seq2[charcode.Code, *Code]

	// AllocateCode assigns a new code to a CID and stores the text and width.
	AllocateCode(cidVal cid.CID, text string, width float64) (charcode.Code, error)

	// DecodeWidth decodes the first character in a PDF string and returns its width.
	// The second return value is the number of bytes consumed from the input string.
	DecodeWidth(s pdf.String) (float64, int)

	Codec() *charcode.Codec

	GetCode(cid cid.CID, text string) (charcode.Code, bool)

	Width(code charcode.Code) float64
}
