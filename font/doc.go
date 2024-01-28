// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

// Package font forms the basis of PDF font handling. Functionality for dealing
// with specific font types is provided by sub-packages.
//
// Fonts can be embedded into a PDF file either as simple fonts or as composite
// fonts.  Simple lead to smaller PDF files, but only allow to use up to 256
// glyphs per embedded copy of the font.  Composite fonts allow to use more
// than 256 glyphs per embedded copy of the font, but lead to larger PDF files.
// There are seven different types of simple fonts in PDF:
//   - Type 1: [seehuhn.de/go/pdf/font/type1.EmbedInfo]
//   - Multiple Master Type 1 (not supported by this library)
//   - CFF font data [seehuhn.de/go/pdf/font/cff.EmbedInfoSimple]
//   - TrueType [seehuhn.de/go/pdf/font/truetype.EmbedInfoSimple]
//   - OpenType with CFF glyph outlines [seehuhn.de/go/pdf/font/opentype.EmbedInfoCFFSimple]
//   - OpenType with "glyf" glyph outlines [seehuhn.de/go/pdf/font/opentype.EmbedInfoGlyfSimple]
//   - Type 3 [seehuhn.de/go/pdf/font/type3.EmbedInfo]
//
// There are four different types of composite fonts:
//   - CFF font data [seehuhn.de/go/pdf/font/cff.EmbedInfoComposite]
//   - TrueType [seehuhn.de/go/pdf/font/truetype.EmbedInfoComposite]
//   - OpenType with CFF glyph outlines [seehuhn.de/go/pdf/font/opentype.EmbedInfoCFFComposite]
//   - OpenType with "glyf" glyph outlines [seehuhn.de/go/pdf/font/opentype.EmbedInfoGlyfComposite]
//
// These different types of font embedding are represented by
// [EmbeddingType].
//
// This library can load the following types of external fonts for embedding
// into PDF files:
//   - TrueType fonts (.ttf files)
//   - OpenType fonts (.otf files)
//   - Type 1 fonts (.pfa or .pfb files, optionally with the corresponding .afm files)
package font
