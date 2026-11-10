// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package fallback

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/form"
)

// addSoundAppearance generates a fallback appearance for a sound
// annotation, mirroring the layout used by file-attachment annotations:
// a fixed 24×24 icon anchored at the upper-left of the supplied Rect.
// The NoZoom and NoRotate flags are left as the caller set them.
func (g *Generator) addSoundAppearance(a *annotation.Sound) (*form.Form, error) {
	a.Rect = pdf.Rectangle{
		LLx: a.Rect.LLx,
		LLy: a.Rect.URy - iconSize,
		URx: a.Rect.LLx + iconSize,
		URy: a.Rect.URy,
	}

	name := pdf.Name(a.Icon)
	if name == "" {
		name = "Speaker" // the default §12.5.6.16 gives
	}

	b := g.begin()
	g.drawIcon(b, name, iconBackground(a.Color))

	return g.harvest(b, pdf.Rectangle{URx: iconSize, URy: iconSize}, a.GetCommon())
}
