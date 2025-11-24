package builder

import (
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
	name := getResourceName(b, "X", obj, &b.Resources.XObject)
	b.emit(content.OpXObject, name)
}
