// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
