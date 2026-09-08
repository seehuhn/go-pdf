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

func (g *Generator) addTextMarkupAppearance(a *annotation.TextMarkup) (*form.Form, error) {
	col := a.Color

	if len(a.QuadPoints) < 4 {
		return &form.Form{
			Content: nil,
			Res:     &content.Resources{},
			BBox:    a.Rect,
		}, nil
	}

	if col == nil {
		b := builder.New(content.Form, nil, g.version)
		g.reset(b)
		return harvest(b, a.Rect)
	}

	// line width for the stroked types; a highlight is a fill and uses none
	var lw float64
	switch a.Type {
	case annotation.TextMarkupTypeSquiggly:
		lw = squigglyLineWidth
	case annotation.TextMarkupTypeUnderline, annotation.TextMarkupTypeStrikeOut:
		lw = textMarkupLineWidth
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
		expand = lw/2 + squigglyAmplitude
	}
	bbox.LLx -= expand
	bbox.LLy -= expand
	bbox.URx += expand
	bbox.URy += expand
	bbox.IRound(2)
	a.Rect = bbox

	b := builder.New(content.Form, nil, g.version)
	g.reset(b)

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
		// corners come as upper-left, upper-right, lower-left, lower-right,
		// so the outline visits the lower two in reverse
		for i := range numQuads {
			q := a.QuadPoints[i*4 : i*4+4]
			b.MoveTo(q[0].X, q[0].Y)
			b.LineTo(q[1].X, q[1].Y)
			b.LineTo(q[3].X, q[3].Y)
			b.LineTo(q[2].X, q[2].Y)
			b.ClosePath()
			b.Fill()
		}

	case annotation.TextMarkupTypeUnderline:
		b.SetLineWidth(lw)
		b.SetStrokeColor(col)
		// along the bottom edge, shifted inward by lw/2 so the stroke fits
		// inside the quad
		for i := range numQuads {
			q := a.QuadPoints[i*4 : i*4+4]
			off := inwardOffset(q[2], q[0], lw/2)
			b.MoveTo(pdf.Round(q[2].X+off.X, 2), pdf.Round(q[2].Y+off.Y, 2))
			b.LineTo(pdf.Round(q[3].X+off.X, 2), pdf.Round(q[3].Y+off.Y, 2))
			b.Stroke()
		}

	case annotation.TextMarkupTypeStrikeOut:
		b.SetLineWidth(lw)
		b.SetStrokeColor(col)
		for i := range numQuads {
			q := a.QuadPoints[i*4 : i*4+4]
			p0 := q[2].Add(q[0].Sub(q[2]).Mul(strikeOutHeight))
			p1 := q[3].Add(q[1].Sub(q[3]).Mul(strikeOutHeight))
			b.MoveTo(pdf.Round(p0.X, 2), pdf.Round(p0.Y, 2))
			b.LineTo(pdf.Round(p1.X, 2), pdf.Round(p1.Y, 2))
			b.Stroke()
		}

	case annotation.TextMarkupTypeSquiggly:
		b.SetLineWidth(lw)
		b.SetStrokeColor(col)
		b.SetLineCap(graphics.LineCapRound)
		b.SetLineJoin(graphics.LineJoinRound)
		for i := range numQuads {
			q := a.QuadPoints[i*4 : i*4+4]
			drawSquigglyLine(b, q[2], q[3], squigglyAmplitude, squigglyHalfPeriod)
		}
	}

	return harvest(b, bbox)
}

// The widths the stroked text markup types are drawn with, and the shape of
// the squiggly underline's wave, in user space units.  A text markup
// annotation has no entry which sets these: its dictionary holds only the
// markup type and the quadrilaterals it covers, and Border describes the
// rectangle drawn around an annotation rather than the markup inside it.
const (
	textMarkupLineWidth = 1.0
	squigglyLineWidth   = 0.7

	squigglyAmplitude  = 1.0
	squigglyHalfPeriod = 2.0
)

// strikeOutHeight is where the strike-out line crosses a quad, as a
// fraction of the way from its bottom edge to its top edge.  A quad
// spans the text's descent to its ascent, and with typical font metrics
// (ascent about 0.9 em, descent about 0.2 em, x-height about 0.5 em)
// the middle of the lower-case letters lies here; the quad's own middle
// would cut through the upper part of the x-height.
const strikeOutHeight = 0.42

// inwardOffset returns a vector of the given length pointing from outer
// toward inner (e.g. from lower-left toward upper-left of a quad).
func inwardOffset(outer, inner vec.Vec2, dist float64) vec.Vec2 {
	return inner.Sub(outer).Normalize().Mul(dist)
}

// drawSquigglyLine draws a wavy line along the segment from p0 to p1.
// The wave is a cosine of the given amplitude about the segment,
// starting a little past its trough on the way up, so it neither begins
// flat nor cuts across the segment at once.  Each stretch between
// consecutive extremes is one cubic Bezier, as are the partial stretches
// at either end.
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

	// phase along the wave, in units of half periods, at distance t
	const startPhase = 1.0 / 6
	phase := func(t float64) float64 {
		return startPhase + t/step
	}
	// signed offset from the segment and its derivative with respect to t
	offset := func(t float64) float64 {
		return -amplitude * math.Cos(math.Pi*phase(t))
	}
	slope := func(t float64) float64 {
		return amplitude * math.Sin(math.Pi*phase(t)) * math.Pi / step
	}
	point := func(t, off float64) (float64, float64) {
		return pdf.Round(p0.X+t*u.X+off*n.X, 2), pdf.Round(p0.Y+t*u.Y+off*n.Y, 2)
	}

	// segment boundaries: the start, each extreme, and the end
	ts := []float64{0}
	for k := 1; k <= nSteps; k++ {
		t := (float64(k) - startPhase) * step
		if t < length-0.01 {
			ts = append(ts, t)
		}
	}
	ts = append(ts, length)

	b.MoveTo(point(0, offset(0)))
	for i := 1; i < len(ts); i++ {
		ta, tb := ts[i-1], ts[i]
		h := (tb - ta) / 3
		// cubic Hermite interpolation of the wave between the boundaries
		cp1x, cp1y := point(ta+h, offset(ta)+h*slope(ta))
		cp2x, cp2y := point(tb-h, offset(tb)-h*slope(tb))
		ex, ey := point(tb, offset(tb))
		b.CurveTo(cp1x, cp1y, cp2x, cp2y, ex, ey)
	}
	b.Stroke()
}
