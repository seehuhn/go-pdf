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
// A font must be "embedded" into a PDF file before it can be used in the file.
// This library allows to embed the following font types:
//   - OpenType fonts (.otf files)
//   - TrueType fonts (.ttf files)
//   - Type 1 fonts (.pfa or .pfb files, optionally with the corresponding .afm files)
//
// Normally, fonts are embedded using the convenience functions in the
// [seehuhn.de/go/pdf/font/simple] and [seehuhn.de/go/pdf/font/composite]
// sub-packages.
//
// If more control is needed, the following functions can be used for the
// different ways of embedding fonts into PDF files as simple fonts:
//   - Type 1: [seehuhn.de/go/pdf/font/type1.New]
//   - Multiple Master Type 1 (not supported by this library)
//   - CFF font data: see [seehuhn.de/go/pdf/font/cff.NewSimple]
//   - TrueType: see [seehuhn.de/go/pdf/font/truetype.NewSimple]
//   - OpenType with CFF glyph outlines: see [seehuhn.de/go/pdf/font/opentype.NewCFFSimple]
//   - OpenType with "glyf" glyph outlines: see [seehuhn.de/go/pdf/font/opentype.NewGlyfSimple]
//   - Type 3: see [seehuhn.de/go/pdf/font/type3.New]
//
// To embed a font as a composite font, the following functions can be used:
//   - CFF font data: see [seehuhn.de/go/pdf/font/cff.NewComposite]
//   - TrueType: see [seehuhn.de/go/pdf/font/truetype.NewComposite]
//   - OpenType with CFF glyph outlines: see [seehuhn.de/go/pdf/font/opentype.NewCFFComposite]
//   - OpenType with "glyf" glyph outlines: see [seehuhn.de/go/pdf/font/opentype.NewGlyfComposite]
//
// # Data Types for Representing Fonts
//
//   - An [Embedder] represents a font before it is embedded into a PDF file.
//   - A [Layouter] is a font which is embedded and ready to typeset text.
//   - The type [Embedded] represents the minimum information about a font
//     required so that a PDF file can be parsed (but no text can be typeset).
//
// # Fonts included in the library
//
//   - The 14 standard PDF fonts are available as [seehuhn.de/go/pdf/font/type1.Standard].
//   - The fonts from the Go Font family are available as [seehuhn.de/go/pdf/font/gofont.All].
package font
