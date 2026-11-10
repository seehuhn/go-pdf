// Copyright 2023 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package document

import "seehuhn.de/go/pdf"

// Default paper sizes as PDF rectangles.
var (
	A4     = &pdf.Rectangle{URx: 595.276, URy: 841.890}
	A4r    = &pdf.Rectangle{URx: 841.890, URy: 595.276}
	A5     = &pdf.Rectangle{URx: 420.945, URy: 595.276}
	A5r    = &pdf.Rectangle{URx: 595.276, URy: 420.945}
	Letter = &pdf.Rectangle{URx: 612, URy: 792}
)
