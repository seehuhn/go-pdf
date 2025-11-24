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

	key := resKey{"X", obj}
	name, ok := b.resName[key]
	if !ok {
		if b.Resources.XObject == nil {
			b.Resources.XObject = make(map[pdf.Name]graphics.XObject)
		}
		name = allocateName("X", b.Resources.XObject)
		b.Resources.XObject[name] = obj
		b.resName[key] = name
	}

	b.emit(content.OpXObject, name)
}
