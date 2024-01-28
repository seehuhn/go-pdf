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

// Package opentype implements OpenType fonts embedded into PDF files.
//
// Use of this package should be avoided when creating new documents: If the
// given font contains CFF glyphs, then it is more efficient to embed the font
// via [cff.NewSimple] or [cff.NewComposite] instead.  If the given font
// contains TrueType glyphs, [truetype.NewSimple] or [truetype.NewComposite]
// should be used instead of the function from this package.
package opentype
