// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

// Package outline extracts glyph outlines from font programs.
//
// The font backends share this code so that a glyph has the same outline
// however the font was embedded.  Both functions compose the per-glyph font
// matrix with the top-level one, which for a CID-keyed CFF font is what puts
// the glyphs of every font DICT on a common grid.
package outline

import (
	"seehuhn.de/go/geom/path"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
)

// SFNT returns a glyph's outline in text space, where a font size of 1 maps
// one em to one unit.  The path is empty for a glyph with nothing to draw.
func SFNT(info *sfnt.Font, gid glyph.ID) path.Path {
	o := info.Outlines
	return o.Path(gid).Transform(o.GlyphMatrix(info.FontMatrix, gid))
}

// CFF returns a glyph's outline in text space, where a font size of 1 maps
// one em to one unit.  The path is empty for a glyph with nothing to draw.
func CFF(f *cff.Font, gid glyph.ID) path.Path {
	return f.Path(gid).Transform(f.GlyphMatrix(f.FontMatrix, gid))
}
