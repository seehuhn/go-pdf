// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package fallback

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/form"
)

// The icon is drawn in a fixed 24×24 local frame.  The annotation's Rect
// is pinned to a 24×24 square anchored at the caller's upper-left corner
// (the (LLx, URy) point of the supplied Rect), and NoZoom | NoRotate are
// forced — matching the fixed-icon convention that §12.5.6.4 imposes on
// text annotations implicitly, but which FileAttachment does not get for
// free.  See §12.5.3: "the annotation's position is defined by the
// coordinates of the upper-left corner of its annotation rectangle".
func (g *Generator) addFileAttachmentAppearance(a *annotation.FileAttachment) (*form.Form, error) {
	a.Rect = pdf.Rectangle{
		LLx: a.Rect.LLx,
		LLy: a.Rect.URy - iconSize,
		URx: a.Rect.LLx + iconSize,
		URy: a.Rect.URy,
	}
	a.Flags |= annotation.FlagNoZoom | annotation.FlagNoRotate

	name := pdf.Name(a.Icon)
	if name == "" {
		name = "PushPin" // the default §12.5.6.15 gives
	}

	b := g.begin()
	g.drawIcon(b, name, iconBackground(a.Color))

	return g.harvest(b, pdf.Rectangle{URx: iconSize, URy: iconSize}, a.GetCommon())
}
