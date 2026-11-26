// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package fallback

import (
	"math"

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/extgstate"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/internal/redactmark"
)

// redactTintAlpha is the opacity of the tint over a region marked for
// redaction.
const redactTintAlpha = 0.3

// redactDefaultColor is the outline colour of a redaction annotation which
// names none.
var redactDefaultColor = color.DeviceRGB{1, 0, 0}

// addRedactAppearance builds the appearances of a redaction annotation.  The
// normal appearance marks the region as pending: an outline with a light
// tint.  The second appearance, used for rollover and down, previews the
// applied redaction, with the outline kept on top so the extent stays
// visible.
//
// The annotation rectangle is left alone, since it anchors the overlay form.
// Both appearances have the rectangle as their bounding box, so parts of the
// quadrilaterals outside it are clipped.
func (g *Generator) addRedactAppearance(a *annotation.Redact) (normal, preview *form.Form, err error) {
	mark := redactmark.FromAnnotation(a, g.ContentFont)
	quads := mark.Quads

	col := color.Color(redactDefaultColor)
	if a.Color != nil {
		col = paint(a.Color)
	}
	lw := annotation.EffectiveBorderWidth(a)

	// normal appearance
	b := g.begin()
	if col != nil {
		b.PushGraphicsState()
		b.SetExtGState(&extgstate.ExtGState{
			Set:       graphics.StateBlendMode | graphics.StateFillAlpha,
			BlendMode: graphics.BlendMode{graphics.BlendModeMultiply},
			FillAlpha: redactTintAlpha,
			SingleUse: true,
		})
		b.SetFillColor(col)
		for _, q := range quads {
			q.Path(b)
		}
		b.Fill()
		b.PopGraphicsState()
	}
	drawRedactOutline(b, quads, col, lw)
	normal, err = g.harvest(b, a.Rect, a.GetCommon())
	if err != nil {
		return nil, nil, err
	}

	// preview of the applied redaction
	b = g.begin()
	redactmark.Draw(b, mark)
	drawRedactOutline(b, quads, col, lw)
	preview, err = g.harvest(b, a.Rect, a.GetCommon())
	if err != nil {
		return nil, nil, err
	}
	return normal, preview, nil
}

// drawRedactOutline strokes the quadrilaterals, drawn inside their edges so
// that the stroke stays within the region.
func drawRedactOutline(b *builder.Builder, quads []redactmark.Quad, col color.Color, lw float64) {
	if col == nil || !(lw > 0) {
		return
	}
	b.PushGraphicsState()
	b.SetStrokeColor(col)
	for _, q := range quads {
		// a pen wider than the region would turn the outline inside out
		pen := min(lw, quadThickness(q)/2)
		if !(pen > 0) {
			continue
		}
		b.SetLineWidth(pen)
		insetQuad(q, pen/2).Path(b)
		b.Stroke()
	}
	b.PopGraphicsState()
}

// quadThickness returns the smallest distance from a side of the convex
// quadrilateral q to a corner not on that side, or 0 if a side has zero
// length.
func quadThickness(q redactmark.Quad) float64 {
	ring := [4]vec.Vec2{q[0], q[1], q[3], q[2]}
	t := math.Inf(1)
	for i := range ring {
		p := ring[i]
		e := ring[(i+1)%4].Sub(p)
		l := e.Length()
		if !(l > 0) {
			return 0
		}
		for _, c := range []vec.Vec2{ring[(i+2)%4], ring[(i+3)%4]} {
			t = min(t, math.Abs(e.Cross(c.Sub(p)))/l)
		}
	}
	return t
}

// insetQuad moves every side of the convex quadrilateral q inwards by d.
func insetQuad(q redactmark.Quad, d float64) redactmark.Quad {
	// corners in boundary order
	ring := [4]vec.Vec2{q[0], q[1], q[3], q[2]}

	var area float64
	for i := range ring {
		p, n := ring[i], ring[(i+1)%4]
		area += p.X*n.Y - n.X*p.Y
	}
	sign := 1.0
	if area < 0 {
		sign = -1
	}

	// unit normals of the sides, pointing inwards
	var normal [4]vec.Vec2
	for i := range ring {
		e := ring[(i+1)%4].Sub(ring[i])
		if l := e.Length(); l > 0 {
			e = e.Mul(1 / l)
		}
		normal[i] = vec.Vec2{X: -e.Y * sign, Y: e.X * sign}
	}

	var out [4]vec.Vec2
	for i := range ring {
		n0, n1 := normal[(i+3)%4], normal[i]
		den := 1 + n0.X*n1.X + n0.Y*n1.Y
		if !(den > 1e-6) {
			out[i] = ring[i]
			continue
		}
		out[i] = ring[i].Add(n0.Add(n1).Mul(d / den))
	}
	return redactmark.Quad{out[0], out[1], out[3], out[2]}
}
