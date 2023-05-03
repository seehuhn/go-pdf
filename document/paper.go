package document

import "seehuhn.de/go/pdf"

// Default paper sizes as PDF rectangles.
var (
	A4     = &pdf.Rectangle{URx: 595.276, URy: 841.890}
	A5     = &pdf.Rectangle{URx: 420.945, URy: 595.276}
	Letter = &pdf.Rectangle{URx: 612, URy: 792}
)
