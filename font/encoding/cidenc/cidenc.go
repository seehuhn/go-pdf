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
//
// Implementations are safe for concurrent use: any number of goroutines may
// read an encoder while another allocates codes.  Codes are only ever added,
// never changed or reassigned, so a code once returned stays valid for the
// life of the encoder.  Iterators hold no lock while yielding, so a caller
// may allocate codes from inside a loop over one.
//
// Each method sees the codes allocated when it was called.  Results of
// separate calls need not agree with each other, so a caller which combines
// them, for example to build a font dictionary out of MappedCodes, CMap and
// ToUnicode, must not allow codes to be allocated in between.
type CIDEncoder interface {
	// WritingMode indicates whether the encoding is for horizontal or vertical
	// writing.
	WritingMode() font.WritingMode

	// Codes iterates over the character codes in a PDF string.
	// The iterator returns the information stored for each code.
	Codes(s pdf.String) iter.Seq[font.Code]

	// MappedCodes iterates over all codes known to the encoder.  The
	// [Info] pointer is only valid until the next iteration; a caller
	// which needs to keep the data must copy it.
	MappedCodes() iter.Seq2[charcode.Code, *Info]

	// Encode returns the code for a CID and text, allocating a new one on
	// first use and storing the text and width with it.  A repeat of the
	// same CID and text yields the code already allocated, and the width
	// given is ignored.
	//
	// An encoder backed by a fixed CMap holds one text and one width per
	// CID, since the CMap gives each CID a single code.  Such an encoder
	// returns an error if the text or the width conflicts with the one
	// already stored, rather than encoding a value it cannot represent.
	Encode(cidVal cid.CID, text string, width float64) (charcode.Code, error)

	// CodesRemaining returns the number of unallocated codes.
	CodesRemaining() int

	// CMap returns a CMap object which describes the mapping
	// from character codes to CIDs.
	CMap(ros *cid.SystemInfo) *cmap.File

	Codec() *charcode.Codec

	GetCode(cid cid.CID, text string) (charcode.Code, bool)

	Width(code charcode.Code) float64

	// ToUnicode returns a ToUnicode CMap representing the text content
	// of the mapped codes.
	ToUnicode() *cmap.ToUnicodeFile
}

type key struct {
	cid  cid.CID
	text string
}

type codeInfo struct {
	CID   cid.CID
	Width float64 // PDF glyph space units
	Text  string
}

// ErrOverflow is returned by [CIDEncoder.Encode] once no further codes can be
// allocated.
var ErrOverflow = errors.New("too many glyphs")
