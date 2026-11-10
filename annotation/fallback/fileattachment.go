// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package fallback

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/form"
)

// The icon is drawn in a fixed 24×24 local frame.  The annotation's Rect
// is pinned to a 24×24 square anchored at the caller's upper-left corner
// (the (LLx, URy) point of the supplied Rect), the point §12.5.3 fixes
// for an annotation which does not zoom or rotate with the page.  The
// NoZoom and NoRotate flags are left as the caller set them.
func (g *Generator) addFileAttachmentAppearance(a *annotation.FileAttachment) (*form.Form, error) {
	a.Rect = pdf.Rectangle{
		LLx: a.Rect.LLx,
		LLy: a.Rect.URy - iconSize,
		URx: a.Rect.LLx + iconSize,
		URy: a.Rect.URy,
	}

	name := pdf.Name(a.Icon)
	if name == "" {
		name = "PushPin" // the default §12.5.6.15 gives
	}

	b := g.begin()
	g.drawIcon(b, name, iconBackground(a.Color))

	return g.harvest(b, pdf.Rectangle{URx: iconSize, URy: iconSize}, a.GetCommon())
}
