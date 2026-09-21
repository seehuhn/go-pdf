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
	"math"

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
)

func (g *Generator) addCircleAppearance(a *annotation.Circle) (*form.Form, error) {
	lw := annotation.EffectiveBorderWidth(a)
	dashPattern := annotation.EffectiveBorderDash(a)
	col := paint(a.Color)

	// the outer edge of the ellipse's own border; see
	// [Generator.addSquareAppearance], which follows the same rule
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

	// a pen wider than this would turn the ellipse it is drawn round inside
	// out
	if m := min(outer.Dx(), outer.Dy()); lw > m/2 {
		lw = m / 2
	}
	pen := 0.0
	if hasOutline {
		pen = lw
	}
	rect := outer.Grow(-pen / 2)

	b := g.begin()

	if hasOutline {
		b.SetLineWidth(lw)
		b.SetStrokeColor(col)
		if len(dashPattern) > 0 {
			b.SetLineDash(dashPattern, 0)
		}
	}
	if hasFill {
		b.SetFillColor(a.FillColor)
	}

	// the curls of a cloudy border bulge outside the ellipse they are drawn
	// round, and Rect grows to take them in
	if isCloudy {
		if verts := flattenEllipse(rect); verts != nil {
			ink := drawCloudyBorder(b, verts, be.Intensity, lw, hasFill, hasOutline)
			return g.harvest(b, fitToInk(&a.Common, &a.Margin, outer, ink.Grow(pen/2)), a.GetCommon())
		}
	}

	xMid := (rect.LLx + rect.URx) / 2
	yMid := (rect.LLy + rect.URy) / 2
	rx := rect.Dx() / 2
	ry := rect.Dy() / 2

	k := (math.Sqrt2 - 1.0) * 4 / 3

	b.MoveTo(xMid+rx, yMid)
	b.CurveTo(xMid+rx, yMid+ry*k, xMid+rx*k, yMid+ry, xMid, yMid+ry)
	b.CurveTo(xMid-rx*k, yMid+ry, xMid-rx, yMid+ry*k, xMid-rx, yMid)
	b.CurveTo(xMid-rx, yMid-ry*k, xMid-rx*k, yMid-ry, xMid, yMid-ry)
	b.CurveTo(xMid+rx*k, yMid-ry, xMid+rx, yMid-ry*k, xMid+rx, yMid)
	b.ClosePath()
	switch {
	case hasOutline && hasFill:
		b.FillAndStroke()
	case hasFill:
		b.Fill()
	default: // hasOutline
		b.Stroke()
	}

	return g.harvest(b, roundOut(a.Rect), a.GetCommon())
}

// flattenEllipse approximates the ellipse inscribed in rect as
// a polygon with enough vertices for smooth cloud curls.
func flattenEllipse(rect pdf.Rectangle) []vec.Vec2 {
	xMid := (rect.LLx + rect.URx) / 2
	yMid := (rect.LLy + rect.URy) / 2
	rx := rect.Dx() / 2
	ry := rect.Dy() / 2

	if rx < 0.5 || ry < 0.5 {
		return nil
	}

	// approximate perimeter (Ramanujan)
	h := (rx - ry) * (rx - ry) / ((rx + ry) * (rx + ry))
	perimeter := math.Pi * (rx + ry) * (1 + 3*h/(10+math.Sqrt(4-3*h)))

	// The spacing is fixed, so the vertex count grows with the size of the
	// ellipse.  It is capped at the number of points the cloudy border can
	// resolve, which is all the polygon is used for, so nothing is lost: a
	// rectangle with coordinates far outside any page would otherwise ask for
	// an unbounded amount of memory, and the conversion to int is undefined
	// for a count beyond the range of int64.
	n := 12
	if k := math.Ceil(perimeter / 4); k > 12 {
		n = int(min(k, maxSamplePoints))
	}

	verts := make([]vec.Vec2, n)
	for i := range n {
		theta := 2 * math.Pi * float64(i) / float64(n)
		verts[i] = vec.Vec2{
			X: xMid + rx*math.Cos(theta),
			Y: yMid + ry*math.Sin(theta),
		}
	}
	return verts
}
