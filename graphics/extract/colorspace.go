// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package extract

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
)

// ColorSpace extracts a color space from a PDF file.
func ColorSpace(c pdf.Cursor, obj pdf.Object, isDirect bool) (color.Space, error) {
	return color.ExtractSpace(c, obj, isDirect)
}
