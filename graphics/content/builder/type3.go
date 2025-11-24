package builder

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
)

// Type3SetWidthOnly sets the glyph width for a Type 3 font glyph.
//
// This implements the PDF graphics operator "d0".
func (b *Builder) Type3SetWidthOnly(wx, wy float64) {
	b.emit(content.OpType3SetWidthOnly, pdf.Number(wx), pdf.Number(wy))
}

// Type3SetWidthAndBoundingBox sets the glyph width and bounding box for a Type 3 font glyph.
//
// This implements the PDF graphics operator "d1".
func (b *Builder) Type3SetWidthAndBoundingBox(wx, wy, llx, lly, urx, ury float64) {
	b.emit(content.OpType3SetWidthAndBoundingBox,
		pdf.Number(wx), pdf.Number(wy),
		pdf.Number(llx), pdf.Number(lly),
		pdf.Number(urx), pdf.Number(ury))
}
