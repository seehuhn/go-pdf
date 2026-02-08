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
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
)

// DrawXObject draws a PDF XObject on the page.
//
// This implements the PDF graphics operator "Do".
func (b *Builder) DrawXObject(obj graphics.XObject) {
	if b.Err != nil {
		return
	}
	// In uncolored patterns and Type 3 glyphs with d1, images are forbidden
	// but image masks are allowed.
	if b.State.ColorOpsForbidden && obj.Subtype() == "Image" && !graphics.IsImageMask(obj) {
		b.Err = errors.New("images not allowed (only image masks)")
		return
	}
	name := b.getXObjectName(obj)
	b.emit(content.OpXObject, name)
}

// DrawInlineImageRaw embeds a small image directly in the content stream.
//
// The dict should contain the image parameters using the abbreviated
// inline image keys (W, H, BPC, CS, etc.) as defined in PDF table 90.
// The data contains the (possibly compressed) image samples.
//
// Inline images are limited to small images; for larger images use
// [Builder.DrawXObject] instead.
func (b *Builder) DrawInlineImageRaw(dict pdf.Dict, data []byte) {
	if b.Err != nil {
		return
	}
	if b.State.ColorOpsForbidden {
		b.Err = errors.New("inline images not allowed in this context")
		return
	}
	b.emit(content.OpInlineImage, dict, pdf.String(data))
}
