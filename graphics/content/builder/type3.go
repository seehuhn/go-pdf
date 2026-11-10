// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package builder

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
)

// Type3ColoredGlyph sets the glyph width for a colored Type 3 font glyph.
// Color operators are allowed after d0.
//
// This implements the PDF graphics operator "d0".
func (b *Builder) Type3ColoredGlyph(wx, wy float64) {
	b.emit(content.OpType3ColoredGlyph, pdf.Number(wx), pdf.Number(wy))
}

// Type3UncoloredGlyph sets the glyph width and bounding box for an
// uncolored Type 3 font glyph. Color operators are NOT allowed after d1.
//
// This implements the PDF graphics operator "d1".
func (b *Builder) Type3UncoloredGlyph(wx, wy, llx, lly, urx, ury float64) {
	b.emit(content.OpType3UncoloredGlyph,
		pdf.Number(wx), pdf.Number(wy),
		pdf.Number(llx), pdf.Number(lly),
		pdf.Number(urx), pdf.Number(ury))
}
