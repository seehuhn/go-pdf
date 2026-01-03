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

package dict

import (
	"iter"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
)

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
	pdf.Embedder

	// MakeFont returns a new font object that can be used to typeset text.
	// The font is immutable, i.e. no new glyphs can be added and no new codes
	// can be defined via the returned font object.
	MakeFont() font.Instance

	// FontInfo returns information about the embedded font file.
	// The information can be used to load the font file and to extract
	// the the glyph corresponding to a character identifier.
	// The result is a pointer to one of the FontInfo* types.
	FontInfo() any

	// Codec allows to interpret character codes for the font.
	Codec() *charcode.Codec

	// TODO(voss): remove? keep?
	Characters() iter.Seq2[charcode.Code, font.Code]
}
