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

func (s *Style) addInkAppearance(a *annotation.Ink) (*form.Form, error) {
	lw := getBorderLineWidth(a.Common.Border, a.BorderStyle)
	dashPattern := getBorderDashPattern(a.Common.Border, a.BorderStyle)
	col := a.Color

	if col == nil || lw <= 0 || !hasInkPoints(a.InkList) {
		return &form.Form{
			Content: nil,
			Res:     &content.Resources{},
			BBox:    a.Rect,
		}, nil
	}

	// We draw InkList only. The optional PDF 2.0 Path entry is used by
	// viewers for click hit-testing, not for the drawn line. The wording in
	// Table 185 appears to conflate the two roles and is best ignored here.
	bbox := inkBBox(a.InkList, lw)
	a.Rect = bbox

	b := builder.New(content.Form, nil, s.version)

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

	// Round caps + round joins for a pen-like look, and so that
	// single-point sub-paths render as filled dots.
	b.SetLineCap(graphics.LineCapRound)
	b.SetLineJoin(graphics.LineJoinRound)

	b.SetLineWidth(lw)
	b.SetStrokeColor(col)
	if len(dashPattern) > 0 {
		b.SetLineDash(dashPattern, 0)
	}

	for _, pts := range a.InkList {
		if len(pts) == 0 {
			continue
		}
		x0, y0 := pdf.Round(pts[0].X, 2), pdf.Round(pts[0].Y, 2)
		b.MoveTo(x0, y0)
		if len(pts) == 1 {
			// zero-length segment + round cap = filled disk of diameter lw
			b.LineTo(x0, y0)
			continue
		}
		for _, p := range pts[1:] {
			b.LineTo(pdf.Round(p.X, 2), pdf.Round(p.Y, 2))
		}
	}
	b.Stroke()

	return harvest(b, bbox)
}

// hasInkPoints reports whether any sub-path contains at least one point.
func hasInkPoints(paths [][]vec.Vec2) bool {
	for _, p := range paths {
		if len(p) > 0 {
			return true
		}
	}
	return false
}

// inkBBox returns the tight bounding box of all points across every sub-path,
// inflated by lw/2 to cover the stroke. Empty sub-paths are skipped.
func inkBBox(paths [][]vec.Vec2, lw float64) pdf.Rectangle {
	var bbox pdf.Rectangle
	seeded := false
	for _, pts := range paths {
		for _, p := range pts {
			if !seeded {
				bbox = pdf.Rectangle{LLx: p.X, LLy: p.Y, URx: p.X, URy: p.Y}
				seeded = true
				continue
			}
			bbox.LLx = min(bbox.LLx, p.X)
			bbox.LLy = min(bbox.LLy, p.Y)
			bbox.URx = max(bbox.URx, p.X)
			bbox.URy = max(bbox.URy, p.Y)
		}
	}
	if !seeded {
		return pdf.Rectangle{}
	}
	bbox.LLx -= lw / 2
	bbox.LLy -= lw / 2
	bbox.URx += lw / 2
	bbox.URy += lw / 2
	bbox.IRound(2)
	return bbox
}
