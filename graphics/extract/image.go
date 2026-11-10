// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package extract

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/image"
)

// Image extracts an image dictionary from a PDF stream.
func Image(c pdf.Cursor, obj pdf.Object, isDirect bool) (*image.Dict, error) {
	return image.ExtractDict(c, obj, isDirect)
}

// ImageMask extracts an image mask from a PDF stream.
func ImageMask(c pdf.Cursor, obj pdf.Object, isDirect bool) (*image.Mask, error) {
	return image.ExtractMask(c, obj, isDirect)
}

// SoftMaskImage extracts a soft mask from a PDF stream.
func SoftMaskImage(c pdf.Cursor, obj pdf.Object, isDirect bool) (*image.SoftMask, error) {
	return image.ExtractSoftMask(c, obj, isDirect)
}
