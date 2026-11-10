// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package font

import (
	"seehuhn.de/go/geom/path"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/sfnt/glyph"
)

// Outliner is implemented by font instances which hold their glyph program
// in memory, so that a consumer can read glyph outlines from the instance
// directly rather than from the font dictionary the instance would embed.
//
// The mapping from CIDs to glyphs may still grow while the instance is in
// use, as [Layouter.Encode] allocates codes.  Implementations are safe for
// any number of concurrent readers while that happens, and a CID once
// assigned a glyph keeps it.  A glyph's outline never changes.
type Outliner interface {
	Instance

	// GlyphID returns the glyph a CID selects.  ok is false for a CID the
	// font has assigned no glyph to.  CID 0 always selects the notdef glyph.
	GlyphID(c cid.CID) (gid glyph.ID, ok bool)

	// Outline returns a glyph's outline in text space, where a font size of
	// 1 maps one em to one unit.  The path is empty for a glyph with nothing
	// to draw.
	Outline(gid glyph.ID) path.Path
}
