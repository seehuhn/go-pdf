// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package builder

import (
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
		b.Err = ErrColorForbidden
		return
	}
	name := b.getXObjectName(obj)
	b.emit(content.OpXObject, name)
}

// DrawInlineImageRaw embeds a small image directly in the content stream.
//
// The dict should contain the image parameters using the abbreviated
// inline image keys (W, H, BPC, CS, etc.) as defined in PDF 2.0, Table 91.
// The data contains the (possibly compressed) image samples.
//
// Inline images are limited to small images; for larger images use
// [Builder.DrawXObject] instead.
func (b *Builder) DrawInlineImageRaw(dict pdf.Dict, data []byte) {
	if b.Err != nil {
		return
	}
	if b.State.ColorOpsForbidden && !content.IsInlineImageMask(dict) {
		b.Err = ErrColorForbidden
		return
	}
	if err := content.ValidateInlineImageFilter(dict); err != nil {
		b.Err = err
		return
	}
	b.emit(content.OpInlineImage, dict, pdf.String(data))
}
