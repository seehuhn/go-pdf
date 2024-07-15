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

// Package pattern provides functionality for creating PDF patterns. Three
// pattern types are supported:
//   - Colored tiling patterns (PatternType 1, PaintType 1).
//     These patterns repeat periodically in the plane
//     and color is specified as part of the pattern.
//     Use [NewColoredBuilder] to create this type of pattern.
//   - Uncolored tiling patterns (PatternType 1, PaintType 2)
//     These patterns repeat periodically in the plane.
//     The drawing color must specified when the pattern is used.
//     Use [NewUncoloredBuilder] to create this type of pattern.
//   - Shading patterns (PatternType 2)
//     These patterns are non-repeating,
//     the color is specified by a shading object.
//     Use [Type2] structs for pattern objects of this type.
package pattern
