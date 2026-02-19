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
	"math"

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/extgstate"
	"seehuhn.de/go/pdf/graphics/form"
)

func (s *Style) addTextMarkupAppearance(a *annotation.TextMarkup) *form.Form {
	col := a.Color

	if len(a.QuadPoints) < 4 {
		return &form.Form{
			Content: nil,
			Res:     &content.Resources{},
			BBox:    a.Rect,
		}
	}

	if col == nil {
		b := builder.New(content.Form, nil)
		b.SetExtGState(s.reset)
		return &form.Form{
			Content: b.Stream,
			Res:     b.Resources,
			BBox:    a.Rect,
		}
	}

	bw := getBorderLineWidth(a.Common.Border, nil)

	// line width for stroke-based types
	var lw float64
	switch a.Type {
	case annotation.TextMarkupTypeHighlight:
		lw = 0
	case annotation.TextMarkupTypeSquiggly:
		lw = 0.7 * bw
	default: // Underline, StrikeOut
		lw = bw
	}

	// bounding box from all quad points
	bbox := pdf.Rectangle{
		LLx: a.QuadPoints[0].X,
		LLy: a.QuadPoints[0].Y,
		URx: a.QuadPoints[0].X,
		URy: a.QuadPoints[0].Y,
	}
	for _, p := range a.QuadPoints[1:] {
		bbox.ExtendVec(p)
	}
	var expand float64
	if a.Type == annotation.TextMarkupTypeSquiggly {
		expand = lw/2 + squigglyBaseAmplitude*bw
	}
	bbox.LLx -= expand
	bbox.LLy -= expand
	bbox.URx += expand
	bbox.URy += expand
	bbox.IRound(1)
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

	numQuads := len(a.QuadPoints) / 4

	switch a.Type {
	case annotation.TextMarkupTypeHighlight:
		gs := &extgstate.ExtGState{
			Set:       graphics.StateBlendMode,
			BlendMode: graphics.BlendMode{graphics.BlendModeMultiply},
			SingleUse: true,
		}
		b.SetExtGState(gs)
		b.SetFillColor(col)
		for i := range numQuads {
			q := a.QuadPoints[i*4 : i*4+4]
			b.MoveTo(q[0].X, q[0].Y)
			b.LineTo(q[1].X, q[1].Y)
			b.LineTo(q[2].X, q[2].Y)
			b.LineTo(q[3].X, q[3].Y)
			b.ClosePath()
			b.Fill()
		}

	case annotation.TextMarkupTypeUnderline:
		b.SetLineWidth(lw)
		b.SetStrokeColor(col)
		// shift inward by lw/2 so the stroke fits inside the quad
		for i := range numQuads {
			q := a.QuadPoints[i*4 : i*4+4]
			off := inwardOffset(q[0], q[3], lw/2)
			b.MoveTo(q[0].X+off.X, q[0].Y+off.Y)
			b.LineTo(q[1].X+off.X, q[1].Y+off.Y)
			b.Stroke()
		}

	case annotation.TextMarkupTypeStrikeOut:
		b.SetLineWidth(lw)
		b.SetStrokeColor(col)
		for i := range numQuads {
			q := a.QuadPoints[i*4 : i*4+4]
			mx0 := (q[0].X + q[3].X) / 2
			my0 := (q[0].Y + q[3].Y) / 2
			mx1 := (q[1].X + q[2].X) / 2
			my1 := (q[1].Y + q[2].Y) / 2
			b.MoveTo(mx0, my0)
			b.LineTo(mx1, my1)
			b.Stroke()
		}

	case annotation.TextMarkupTypeSquiggly:
		b.SetLineWidth(lw)
		b.SetStrokeColor(col)
		b.SetLineCap(graphics.LineCapRound)
		b.SetLineJoin(graphics.LineJoinRound)
		amplitude := squigglyBaseAmplitude * bw
		halfPeriod := squigglyBaseHalfPeriod * bw
		for i := range numQuads {
			q := a.QuadPoints[i*4 : i*4+4]
			drawSquigglyLine(b, q[0], q[1], amplitude, halfPeriod)
		}
	}

	return &form.Form{
		Content: b.Stream,
		Res:     b.Resources,
		BBox:    bbox,
	}
}

// inwardOffset returns a vector of the given length pointing from outer
// toward inner (e.g. from bottom-left toward top-left of a quad).
func inwardOffset(outer, inner vec.Vec2, dist float64) vec.Vec2 {
	return inner.Sub(outer).Normalize().Mul(dist)
}

// base squiggly parameters at border width 1
const (
	squigglyBaseAmplitude  = 1.0
	squigglyBaseHalfPeriod = 2.0
)

// drawSquigglyLine draws a wavy line from p0 to p1 using alternating
// cubic Bezier arcs.
func drawSquigglyLine(b *builder.Builder, p0, p1 vec.Vec2, amplitude, halfPeriod float64) {
	d := p1.Sub(p0)
	length := d.Length()
	if length < 0.01 {
		return
	}

	// unit direction and perpendicular
	u := d.Normalize()
	n := u.Rot90()

	nSteps := max(int(math.Round(length/halfPeriod)), 1)
	step := length / float64(nSteps)

	// cubic Bezier control point factor for a smooth bump
	const k = 4.0 / 3.0

	b.MoveTo(pdf.Round(p0.X, 2), pdf.Round(p0.Y, 2))
	sign := 1.0
	for i := range nSteps {
		t0 := float64(i) * step
		t1 := t0 + step
		ex := pdf.Round(p0.X+t1*u.X, 2)
		ey := pdf.Round(p0.Y+t1*u.Y, 2)

		off := sign * amplitude * k
		cp1x := pdf.Round(p0.X+(t0+step/3)*u.X+off*n.X, 2)
		cp1y := pdf.Round(p0.Y+(t0+step/3)*u.Y+off*n.Y, 2)
		cp2x := pdf.Round(p0.X+(t0+2*step/3)*u.X+off*n.X, 2)
		cp2y := pdf.Round(p0.Y+(t0+2*step/3)*u.Y+off*n.Y, 2)

		b.CurveTo(cp1x, cp1y, cp2x, cp2y, ex, ey)
		sign = -sign
	}
	b.Stroke()
}
