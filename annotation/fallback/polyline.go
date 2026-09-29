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
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
)

func (g *Generator) addPolyLineAppearance(a *annotation.PolyLine) (*form.Form, error) {
	lw := annotation.EffectiveBorderWidth(a)
	dashPattern := annotation.EffectiveBorderDash(a)
	col := paint(a.Color)

	if col == nil || lw <= 0 {
		return &form.Form{
			Content: nil,
			Res:     &content.Resources{},
			BBox:    a.Rect,
		}, nil
	}

	points := roundPoints(polylineVertices(a))
	if len(points) < 2 {
		return &form.Form{
			Content: nil,
			Res:     &content.Resources{},
			BBox:    a.Rect,
		}, nil
	}

	startLE := normalizeLE(a.LineEndingStyle[0])
	endLE := normalizeLE(a.LineEndingStyle[1])

	bbox := openPolylineBBox(points, lw, graphics.LineJoinMiter, startLE, endLE)
	a.Rect = bbox

	b := g.begin()

	b.SetLineWidth(lw)
	b.SetStrokeColor(col)
	if len(dashPattern) > 0 {
		b.SetLineDash(dashPattern, 0)
	}

	drawOpenPolyline(b, points, startLE, endLE, paint(a.FillColor))

	return g.harvest(b, bbox, a.GetCommon())
}

// PolyLineRect returns the rectangle the fallback appearance of a PolyLine
// annotation is drawn into: the bounds of the stroke along the vertices,
// with its miter joins and line endings, rounded outwards to two decimals.
// The generator sets Rect to this value when it draws the appearance; a
// caller which edits the vertices or the border width can use it to keep
// Rect valid until then.  The second result is false if the annotation has
// fewer than two vertices.
func PolyLineRect(a *annotation.PolyLine) (pdf.Rectangle, bool) {
	points := roundPoints(polylineVertices(a))
	if len(points) < 2 {
		return pdf.Rectangle{}, false
	}
	lw := annotation.EffectiveBorderWidth(a)
	startLE := normalizeLE(a.LineEndingStyle[0])
	endLE := normalizeLE(a.LineEndingStyle[1])
	return openPolylineBBox(points, lw, graphics.LineJoinMiter, startLE, endLE), true
}

// polylineVertices extracts the vertex list from a polyline annotation.
func polylineVertices(a *annotation.PolyLine) []vec.Vec2 {
	if len(a.Vertices) >= 4 {
		n := len(a.Vertices) / 2
		verts := make([]vec.Vec2, n)
		for i := range n {
			verts[i] = vec.Vec2{X: a.Vertices[2*i], Y: a.Vertices[2*i+1]}
		}
		return verts
	}
	return nil
}
