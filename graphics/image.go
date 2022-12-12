package graphics

import (
	"fmt"

	"seehuhn.de/go/pdf"
)

// DrawImage draws an image on the page.
func (p *Page) DrawImage(imageRef *pdf.Reference) {
	// TODO(voss): check that the states are correct
	// if !p.valid("Stroke", statePath, stateClipped) {
	// 	return
	// }

	if p.imageNames == nil {
		p.imageNames = make(map[pdf.Reference]pdf.Name)
	}

	name, ok := p.imageNames[*imageRef]
	if !ok {
		name = pdf.Name(fmt.Sprintf("Im%d", len(p.imageNames)+1))
		p.imageNames[*imageRef] = name
		if p.resources == nil {
			p.resources = &pdf.Resources{}
		}
		if p.resources.XObject == nil {
			p.resources.XObject = pdf.Dict{}
		}
		p.resources.XObject[name] = imageRef
	}

	err := name.PDF(p.content)
	if err != nil {
		p.err = err
		return
	}
	_, p.err = fmt.Fprintln(p.content, "", "Do")
}
