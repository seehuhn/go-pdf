package pages

import "seehuhn.de/go/pdf"

// Rectangle represents a PDF rectangle, given by the coordinates of
// two diagonally opposite corners.
type Rectangle struct {
	LLx, LLy, URx, URy float64
}

// ToObject converts the rectangle to its representation in a PDF file.
func (rect *Rectangle) ToObject() pdf.Array {
	res := pdf.Array{}
	for _, x := range []float64{rect.LLx, rect.LLy, rect.URx, rect.URy} {
		if i := pdf.Integer(x); float64(i) == x {
			res = append(res, i)
		} else {
			res = append(res, pdf.Real(x))
		}
	}
	return res
}

// Attributes specifies inheritable Page Attributes.
//
// These attributes are documented in sections 7.7.3.3 and 7.7.3.4 of
// PDF 32000-1:2008.
type Attributes struct {
	Resources pdf.Dict
	MediaBox  *Rectangle
	CropBox   *Rectangle
	Rotate    int
}

// Default paper sizes as PDF rectangles.
var (
	A4     = &Rectangle{0, 0, 595.276, 841.890}
	A5     = &Rectangle{0, 0, 419.528, 595.276}
	Letter = &Rectangle{0, 0, 612, 792}
	Legal  = &Rectangle{0, 0, 612, 1008}
)
