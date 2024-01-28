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

// Package font implements the foundations of PDF font handling.  Support for
// specific font types is provided by sub-packages.
//
// # Embedding fonts into PDF files
//
// This library can load the following types of external fonts for embedding
// into PDF files:
//   - OpenType fonts (.otf files)
//   - TrueType fonts (.ttf files)
//   - Type 1 fonts (.pfa or .pfb files, optionally with the corresponding .afm files)
//
// # Data Types for Representing Fonts
//
//   - [Font]
//   - [Layouter]
//   - [Embedded]
//
// # Fonts included in the library
//
//   - The 14 standard PDF fonts are available as [seehuhn.de/go/pdf/font/type1.All].
//   - The Go Font family is available as [seehuhn.de/go/pdf/font/gofont.All].
//
// # Font Data Inside PDF Files
//
// Fonts can be embedded into a PDF file either as "simple fonts" or as
// "composite fonts".  Simple fonts lead to smaller PDF files, but only allow
// to use up to 256 glyphs per embedded copy of the font.  Composite fonts
// allow to use more than 256 glyphs per embedded copy of the font, but lead to
// larger PDF files. The different ways of embedding fonts in a PDF file are
// represented by values of type [EmbeddingType]. There are seven different
// types of embedded simple fonts:
//   - Type 1: see [seehuhn.de/go/pdf/font/type1.EmbedInfo]
//   - Multiple Master Type 1 (not supported by this library)
//   - CFF font data: see [seehuhn.de/go/pdf/font/cff.EmbedInfoSimple]
//   - TrueType: see [seehuhn.de/go/pdf/font/truetype.EmbedInfoSimple]
//   - OpenType with CFF glyph outlines: see [seehuhn.de/go/pdf/font/opentype.EmbedInfoCFFSimple]
//   - OpenType with "glyf" glyph outlines: see [seehuhn.de/go/pdf/font/opentype.EmbedInfoGlyfSimple]
//   - Type 3: see [seehuhn.de/go/pdf/font/type3.EmbedInfo]
//
// There are four different types of embedded composite fonts:
//   - CFF font data: see [seehuhn.de/go/pdf/font/cff.EmbedInfoComposite]
//   - TrueType: see [seehuhn.de/go/pdf/font/truetype.EmbedInfoComposite]
//   - OpenType with CFF glyph outlines: see [seehuhn.de/go/pdf/font/opentype.EmbedInfoCFFComposite]
//   - OpenType with "glyf" glyph outlines: see [seehuhn.de/go/pdf/font/opentype.EmbedInfoGlyfComposite]
package font
