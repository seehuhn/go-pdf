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
// A font must be "embedded" into a PDF file before it can be used.
// This library allows to embed the following font types:
//   - OpenType fonts (.otf files)
//   - TrueType fonts (.ttf files)
//   - Type 1 fonts (.pfa or .pfb files, optionally with the corresponding .afm files)
//   - Type 3 fonts (glyphs drawn using PDF commands)
//
// Normally, fonts are embedded using the convenience functions in
// [seehuhn.de/go/pdf/font/embed].
// If more control is needed, the following functions can be used for the
// different ways of embedding fonts into PDF files as simple fonts:
//   - [seehuhn.de/go/pdf/font/cff.New]
//   - [seehuhn.de/go/pdf/font/truetype.New]
//   - [seehuhn.de/go/pdf/font/opentype.New]
//   - [seehuhn.de/go/pdf/font/type1.New]
//   - [seehuhn.de/go/pdf/font/type3.New]
//
// # Fonts included in the library
//
//   - The 14 standard PDF fonts are available as [seehuhn.de/go/pdf/font/type1.Standard].
//   - The fonts from the Go Font family are available as [seehuhn.de/go/pdf/font/gofont].
//
// # Data Types for Representing Fonts
//
//   - An [Font] represents a font before it is embedded into a PDF file.
//   - A [Layouter] is a font which is embedded and ready to typeset text.
//   - The type [Embedded] represents the minimum information about a font
//     required so that a PDF file can be parsed (but no text can be typeset).
package font
