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
// Each struct in this package provides a direct, one-to-one representation of
// a specific PDF font dictionary type as defined in the PDF specification. The
// structs are designed to preserve all information that can be stored in a PDF
// file introducing only limited. All structs implement the [font.Dict]
// interface to enable polymorphic operations.
//
// Each type of font dictionary is represented by a Go structure which holds
// all the information from the dictionary. Functions with names starting with
// "Extract" can read a font dictionary from a PDF file. Methods named
// "WriteToPDF" can write a font dictionary to a PDF file. Writing followed by
// reading will yield the same font dictionary.
//
// This package deals exclusively with font dictionary structures and does not
// handle the embedding of font programs (glyph outlines). When embedding
// fonts, the caller must separately embed the font program data and provide
// its reference via the FontRef field.
//
// Simple fonts select glyphs either by name or via the built-in encoding of a
// font. By contrast, composite fonts select glyphs via a character identifier
// (CID). To make simple fonts more similar to composite fonts, this library
// introduces artificial CIDs for simple fonts. These values are defined to be
// 0 for codes which map to the .notdef glyph, and the character code plus one
// for all other codes.
package dict

import _ "seehuhn.de/go/pdf/font"
