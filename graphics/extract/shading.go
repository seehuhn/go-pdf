// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package extract

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/shading"
)

// Shading extracts a shading from a PDF file.
func Shading(c pdf.Cursor, obj pdf.Object, isDirect bool) (graphics.Shading, error) {
	return shading.Extract(c, obj, isDirect)
}
