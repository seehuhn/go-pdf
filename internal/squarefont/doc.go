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

// Package squarefont provides standardized test fonts for unit testing font geometry.
//
// This package creates minimal test fonts across different font technologies
// (TrueType, CFF, Type 1, Type 3) that all produce identical PDF glyph space
// values.  The fonts are designed to test unit conversion consistency and font
// geometry calculations.
//
// The values every font in the collection reproduces are the constants in this
// package; [All] lists the fonts themselves, one per technology and units per
// em, including asymmetric font matrices for edge case testing.
//
// # Glyphs
//
// Each font contains exactly three glyphs:
//
//   - GID 0: ".notdef" (blank glyph, width [NotdefWidth])
//   - GID 1: "space" (blank glyph, width [SpaceWidth])
//   - GID 2: "A" (filled square, width [SquareWidth])
//
// The "A" glyph is a simple filled rectangle that forms a 400×400 square
// when rendered in PDF glyph space coordinates.
package squarefont
