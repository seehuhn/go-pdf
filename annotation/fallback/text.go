// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package fallback

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/form"
)

func (g *Generator) addTextAppearance(a *annotation.Text) (*form.Form, error) {
	// §12.5.6.4: text annotations behave as if NoZoom and NoRotate were
	// always set, anchored at the upper-left corner of Rect.  Pin Rect to
	// a 24×24 square at that corner so the §12.5.5 scale-to-Rect algorithm
	// is a no-op on viewers that don't honour the implicit flags.
	a.Rect = pdf.Rectangle{
		LLx: a.Rect.LLx,
		LLy: a.Rect.URy - iconSize,
		URx: a.Rect.LLx + iconSize,
		URy: a.Rect.URy,
	}

	name := pdf.Name(a.Icon)
	if name == "" {
		name = "Note" // the default §12.5.6.4 gives
	}

	b := g.begin()
	g.drawIcon(b, name, iconBackground(a.Color))

	return g.harvest(b, pdf.Rectangle{URx: iconSize, URy: iconSize}, a.GetCommon())
}
