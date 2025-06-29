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
// (TrueType, CFF, Type1) that all produce identical PDF glyph space values.
// The fonts are designed to test unit conversion consistency and font geometry
// calculations.
//
// # Standard Values
//
// All fonts in this package are designed to produce these PDF glyph space values:
//
//   - Square glyph bounding box: [100, 500] × [200, 600] (400×400 square)
//   - Ascent: 800
//   - Descent: -200
//   - Leading: 1200
//   - UnderlinePosition: -100
//   - UnderlineThickness: 50
//
// # Glyphs
//
// Each font contains exactly three glyphs:
//
//   - GID 0: ".notdef" (blank glyph, width 500)
//   - GID 1: "space" (blank glyph, width 250)
//   - GID 2: "A" (filled square, width 500)
//
// The "A" glyph is a simple filled rectangle that forms a 400×400 square
// when rendered in PDF glyph space coordinates.
//
// # Font Variants
//
// TrueType fonts (3 variants):
//   - TrueType-500: 500 units per em
//   - TrueType-1000: 1000 units per em
//   - TrueType-2000: 2000 units per em
//
// CFF fonts (4 variants):
//   - CFF-500: FontMatrix equivalent to 500 UPM
//   - CFF-1000: FontMatrix equivalent to 1000 UPM
//   - CFF-2000: FontMatrix equivalent to 2000 UPM
//   - CFF-Asymmetric: Asymmetric FontMatrix for edge case testing
//
// Type1 fonts (4 variants):
//   - Type1-500: FontMatrix equivalent to 500 UPM
//   - Type1-1000: FontMatrix equivalent to 1000 UPM
//   - Type1-2000: FontMatrix equivalent to 2000 UPM
//   - Type1-Asymmetric: Asymmetric FontMatrix for edge case testing
package squarefont
