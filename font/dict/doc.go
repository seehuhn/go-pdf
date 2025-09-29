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

// Package dict implements reading and writing of PDF font dictionaries.
//
// The package supports all PDF font dictionary types defined in the PDF
// specification. Each struct provides a one-to-one representation of its
// corresponding dictionary type:
//   - [Type1]
//   - [TrueType]
//   - [Type3]
//   - [CIDFontType0]
//   - [CIDFontType2]
//
// All of these structs implement the [Dict] interface.
//
// Simple fonts select glyphs either by name or via the built-in encoding of a
// font. By contrast, composite fonts select glyphs via a character identifier
// (CID). To make simple fonts more consistent with composite fonts, this
// library introduces artificial CIDs for simple fonts. These values are
// defined to be 0 for unused codes, and the character code plus one for all
// other codes.
package dict

import _ "seehuhn.de/go/pdf/font"
