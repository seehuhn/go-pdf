// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package cidenc

import (
	"errors"
	"iter"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
)

// TODO(voss): add CIDEncoder implementations for the standard PDF CMaps.

// TODO(voss): disentangle the width information from the CMap information.

type Info struct {
	CID   cid.CID
	Width float64
	Text  string
}

// A CIDEncoder maps character codes to CIDs, glyph widths and text content.
type CIDEncoder interface {
	// WritingMode indicates whether the font is for horizontal or vertical
	// writing.
	WritingMode() font.WritingMode

	// Codes iterates over the character codes in a PDF string.
	// The iterator returns the information stored for each code.
	Codes(s pdf.String) iter.Seq[*font.Code]

	// MappedCodes iterates over all codes known to the encoder.
	MappedCodes() iter.Seq2[charcode.Code, *Info]

	// AllocateCode assigns a new code to a CID and stores the text and width.
	AllocateCode(cidVal cid.CID, text string, width float64) (charcode.Code, error)

	CMap(ros *cid.SystemInfo) *cmap.File

	Codec() *charcode.Codec

	GetCode(cid cid.CID, text string) (charcode.Code, bool)

	Width(code charcode.Code) float64

	// ToUnicode returns a ToUnicode CMap representing the text content
	// of the mapped codes.
	ToUnicode() *cmap.ToUnicodeFile
}

// TODO(voss): include the width?
type key struct {
	cid  cid.CID
	text string
}

type codeInfo struct {
	CID   cid.CID
	Width float64 // PDF glyph space units
	Text  string
}

var (
	ErrDuplicateCode = errors.New("duplicate code")
	ErrOverflow      = errors.New("too many glyphs")
)
