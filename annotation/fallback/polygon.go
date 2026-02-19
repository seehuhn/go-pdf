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
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/extgstate"
	"seehuhn.de/go/pdf/graphics/form"
)

func (s *Style) addPolygonAppearance(a *annotation.Polygon) *form.Form {
	lw := getBorderLineWidth(a.Common.Border, a.BorderStyle)
	dashPattern := getBorderDashPattern(a.Common.Border, a.BorderStyle)
	col := a.Color

	bbox := a.Rect

	if m := min(bbox.Dx(), bbox.Dy()); lw > m/2 {
		lw = m / 2
	}

	be := a.BorderEffect
	isCloudy := be != nil && be.Style == "C" && be.Intensity > 0

	hasOutline := col != nil && lw > 0
	hasFill := a.FillColor != nil
	if !(hasOutline || hasFill) {
		return &form.Form{
			Content: nil,
			Res:     &content.Resources{},
			BBox:    bbox,
		}
	}

	b := builder.New(content.Form, nil)

	b.SetExtGState(s.reset)
	if a.StrokingTransparency != 0 || a.NonStrokingTransparency != 0 {
		gs := &extgstate.ExtGState{
			Set:         graphics.StateStrokeAlpha | graphics.StateFillAlpha,
			StrokeAlpha: 1 - a.StrokingTransparency,
			FillAlpha:   1 - a.NonStrokingTransparency,
			SingleUse:   true,
		}
		b.SetExtGState(gs)
	}

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
	if isCloudy {
		verts := polygonVertices(a)
		if len(verts) >= 3 {
			cloudBBox := drawCloudyBorder(b, verts, be.Intensity, lw, hasFill, hasOutline)
			bbox = pdf.Rectangle{
				LLx: cloudBBox.LLx - lw/2,
				LLy: cloudBBox.LLy - lw/2,
				URx: cloudBBox.URx + lw/2,
				URy: cloudBBox.URy + lw/2,
			}
			bbox.IRound(1)
			a.Rect = bbox
			drawn = true
		}
	}
	if !drawn {
		drawPolygonPath(b, a)
		switch {
		case hasOutline && hasFill:
			b.FillAndStroke()
		case hasFill:
			b.Fill()
		default: // hasOutline
			b.Stroke()
		}
	}

	return &form.Form{
		Content: b.Stream,
		Res:     b.Resources,
		BBox:    bbox,
	}
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

// drawPolygonPath draws the original (non-cloudy) polygon path.
func drawPolygonPath(b *builder.Builder, a *annotation.Polygon) {
	if len(a.Vertices) >= 4 {
		b.MoveTo(a.Vertices[0], a.Vertices[1])
		for i := 2; i+1 < len(a.Vertices); i += 2 {
			b.LineTo(a.Vertices[i], a.Vertices[i+1])
		}
	}
	b.ClosePath()
}
