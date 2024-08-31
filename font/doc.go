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
// # Loading/Creating Fonts
//
// This library supports the following font types:
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
//   - The 14 standard PDF fonts: [seehuhn.de/go/pdf/font/standard]
//   - Extended versions of the 14 standard PDF fonts: [seehuhn.de/go/pdf/font/extended]
//   - The Go Font family: [seehuhn.de/go/pdf/font/gofont]
//
// # Data Types for Representing Fonts
//
//   - A [Font] represents a font before it is embedded into a PDF file.
//   - A [Layouter] is a font which includes enough information to typeset new text.
//   - The type [Embedded] represents a font within a PDF file.
package font
