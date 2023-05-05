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

package cmap

// https://adobe-type-tools.github.io/font-tech-notes/pdfs/5014.CIDFont_Spec.pdf
// https://adobe-type-tools.github.io/font-tech-notes/pdfs/5099.CMapResources.pdf

// A CMap specifies a mapping from character codes to
// a font number (always 0 for PDF), and a character selector (the CID).
//
// A character identifier (CID) gives the index of character in a character
// collection. The character collection is specified by the CIDSystemInfo
// dictionary.

// character code (sequence of bytes) -> CID -> glyph identifier (GID)

// --------------------

// CID (short for character identifier) gives the index of character in a
// character collection.
//
// TODO(voss): should this be a uint16?
type CID uint32

type Info struct{}
