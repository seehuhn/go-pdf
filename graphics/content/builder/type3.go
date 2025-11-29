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
