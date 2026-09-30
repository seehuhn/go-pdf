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
	"seehuhn.de/go/geom/path"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
)

func (g *Generator) addPolygonAppearance(a *annotation.Polygon) (*form.Form, error) {
	lw := annotation.EffectiveBorderWidth(a)
	dashPattern := annotation.EffectiveBorderDash(a)
	col := paint(a.Color)

	verts := polygonVertices(a)
	pts := roundPoints(verts)

	bbox := a.Rect
	derived := false
	if bbox.IsZero() && len(pts) >= 2 {
		// a file which leaves Rect out still says where the polygon is, in
		// its vertices; without a rectangle the appearance would have no
		// bounding box and could not be written back out
		bbox = polygonPathBBox(pts, lw, len(dashPattern) > 0)
		a.Rect = bbox
		derived = true
	}

	// a rectangle the file supplied has to hold the border, which a thin one
	// may be too small for; one derived from the path already fits it
	if m := min(bbox.Dx(), bbox.Dy()); !derived && lw > m/2 {
		lw = m / 2
	}

	be := a.BorderEffect
	isCloudy := be != nil && be.Style == "C" && be.Intensity > 0

	// a width of 0 draws a hairline
	hasOutline := col != nil && lw >= 0
	hasFill := paint(a.FillColor) != nil
	if !(hasOutline || hasFill) {
		return &form.Form{
			Content: nil,
			Res:     &content.Resources{},
			BBox:    bbox,
		}, nil
	}

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

	drawn := false
	if isCloudy && len(verts) >= 3 {
		if ink, ok := drawCloudyBorder(b, verts, be.Intensity, lw, hasFill, hasOutline); ok {
			bbox = roundOut(ink.Grow(pathPrecision))
			a.Rect = bbox
			drawn = true
		}
	}
	if !drawn && len(pts) > 0 {
		drawPolygonPath(b, pts)
		switch {
		case hasOutline && hasFill:
			b.FillAndStroke()
		case hasFill:
			b.Fill()
		default: // hasOutline
			b.Stroke()
		}
	}

	return g.harvest(b, bbox, a.GetCommon())
}

// PolygonRect returns the rectangle the fallback appearance of a Polygon
// annotation without a cloudy border needs: the bounds of the stroke along
// the vertices, with its miter joins, rounded outwards to two decimals.  The
// generator derives Rect this way where the annotation has none, and keeps
// a Rect it is given; a caller which edits the vertices or the border width
// can use this to give the annotation a rectangle the stroke fits in.  The
// second result is false if the annotation has fewer than two vertices.
func PolygonRect(a *annotation.Polygon) (pdf.Rectangle, bool) {
	pts := roundPoints(polygonVertices(a))
	if len(pts) < 2 {
		return pdf.Rectangle{}, false
	}
	return polygonPathBBox(pts, annotation.EffectiveBorderWidth(a),
		len(annotation.EffectiveBorderDash(a)) > 0), true
}

// polygonPathBBox is the rectangle bounding the stroke drawn along a
// polygon's vertices, rounded outwards.  The path is closed, so every vertex
// carries a miter join, and a sharp corner reaches beyond the border width.
// With dashed set, a dash can also end at a vertex with a butt cap, and the
// bounds hold for any dash pattern and phase.  The vertices must be the
// rounded ones the polygon is drawn through; see [roundPoints].
func polygonPathBBox(verts []vec.Vec2, lw float64, dashed bool) pdf.Rectangle {
	r, ok := strokeBounds([][]vec.Vec2{verts}, true, path.StrokeOptions{
		Width:  lw,
		Cap:    graphics.LineCapButt,
		Join:   graphics.LineJoinMiter,
		Dashed: dashed,
	})
	if !ok {
		return pdf.Rectangle{}
	}
	return roundOut(r.Grow(hairlineAllowance(lw)))
}

// polygonVertices extracts the vertex list from a polygon annotation.
func polygonVertices(a *annotation.Polygon) []vec.Vec2 {
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

// drawPolygonPath draws a closed path through pts, which must be non-empty
// and already rounded; see [roundPoints].
func drawPolygonPath(b *builder.Builder, pts []vec.Vec2) {
	b.MoveTo(pts[0].X, pts[0].Y)
	for _, p := range pts[1:] {
		b.LineTo(p.X, p.Y)
	}
	b.ClosePath()
}
