// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
)

func (g *Generator) addSquareAppearance(a *annotation.Square) (*form.Form, error) {
	lw := annotation.EffectiveBorderWidth(a)
	dashPattern := annotation.EffectiveBorderDash(a)
	col := paint(a.Color)

	// Rect less the margins (§12.5.6.8 /RD) is the outer edge of the square's
	// own border, which the stroke reaches and stops at.  A plain border is
	// all there is, so Rect and the margins are left as the file gave them.
	// The curls of a cloudy one bulge past that edge, and Rect grows to take
	// them in, so that nothing is drawn outside it (§12.5.4).  Either way the
	// outer edge stays put, and the appearance built a second time from the
	// result is the same one.
	outer := applyMargins(a.Rect, a.Margin)

	be := a.BorderEffect
	isCloudy := be != nil && be.Style == "C" && be.Intensity > 0

	hasOutline := col != nil && lw > 0
	hasFill := paint(a.FillColor) != nil
	if !(hasOutline || hasFill) {
		return &form.Form{
			Content: nil,
			Res:     &content.Resources{},
			BBox:    roundOut(a.Rect),
		}, nil
	}

	// a pen wider than this would turn the square it is drawn round inside
	// out
	if m := min(outer.Dx(), outer.Dy()); lw > m/2 {
		lw = m / 2
	}
	pen := 0.0
	if hasOutline {
		pen = lw
	}

	b := g.begin()

	if hasOutline {
		b.SetLineWidth(lw)
		b.SetStrokeColor(col)
		b.SetLineDash(dashPattern, 0)
	}
	if hasFill {
		b.SetFillColor(a.FillColor)
	}

	// The square is the outer edge pulled in by half the line width, so that
	// the stroke centred on it reaches the outer edge and no further: a wider
	// pen thickens the border inwards rather than pushing it out of Rect.
	rect := outer.Grow(-pen / 2)

	if isCloudy {
		// The curls bulge outside the square they are drawn round, so the
		// annotation rectangle grows to take them in and the margins record
		// where the square's border ends: this is the case §12.5.6.8
		// describes, where a border effect pushes Rect out beyond the square.
		ink := drawCloudyBorder(b, squareVertices(rect), be.Intensity, lw, hasFill, hasOutline)
		return g.harvest(b, fitToInk(&a.Common, &a.Margin, outer, ink.Grow(pen/2)), a.GetCommon())
	}

	b.Rectangle(rect.LLx, rect.LLy, rect.Dx(), rect.Dy())
	switch {
	case hasOutline && hasFill:
		b.FillAndStroke()
	case hasFill:
		b.Fill()
	default: // hasOutline
		b.Stroke()
	}

	// The bounding box is the annotation rectangle, which the appearance is
	// fitted to, rounded outwards so that a caller's floating-point noise is
	// not written to the file a second time.  Rounding it outwards rather
	// than to nearest shrinks the drawing by the hundredth of a point it
	// adds, instead of pushing it past the edge of the box.
	return g.harvest(b, roundOut(a.Rect), a.GetCommon())
}

// squareVertices returns the corners of rect, counter-clockwise so that a
// cloudy border drawn round them bulges outwards.
func squareVertices(rect pdf.Rectangle) []vec.Vec2 {
	return []vec.Vec2{
		{X: rect.LLx, Y: rect.LLy},
		{X: rect.URx, Y: rect.LLy},
		{X: rect.URx, Y: rect.URy},
		{X: rect.LLx, Y: rect.URy},
	}
}
