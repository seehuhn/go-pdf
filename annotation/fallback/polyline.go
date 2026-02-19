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
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/extgstate"
	"seehuhn.de/go/pdf/graphics/form"
)

func (s *Style) addPolyLineAppearance(a *annotation.PolyLine) *form.Form {
	lw := getBorderLineWidth(a.Common.Border, a.BorderStyle)
	dashPattern := getBorderDashPattern(a.Common.Border, a.BorderStyle)
	col := a.Color

	if col == nil || lw <= 0 {
		return &form.Form{
			Content: nil,
			Res:     &content.Resources{},
			BBox:    a.Rect,
		}
	}

	points := polylineVertices(a)
	if len(points) < 2 {
		return &form.Form{
			Content: nil,
			Res:     &content.Resources{},
			BBox:    a.Rect,
		}
	}

	startLE := normalizeLE(a.LineEndingStyle[0])
	endLE := normalizeLE(a.LineEndingStyle[1])

	bbox := openPolylineBBox(points, lw, startLE, endLE)
	a.Rect = bbox

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

	b.SetLineWidth(lw)
	b.SetStrokeColor(col)
	if len(dashPattern) > 0 {
		b.SetLineDash(dashPattern, 0)
	}

	drawOpenPolyline(b, points, startLE, endLE, a.FillColor)

	return &form.Form{
		Content: b.Stream,
		Res:     b.Resources,
		BBox:    bbox,
	}
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
