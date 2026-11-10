// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package fallback

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
)

// pilcrowSize is the font size, in points, of the paragraph symbol drawn
// under a caret whose Symbol is P.
const pilcrowSize = 10.0

func (g *Generator) addCaretAppearance(a *annotation.Caret) (*form.Form, error) {
	col := paint(a.Color)
	if col == nil {
		return &form.Form{
			Content: nil,
			Res:     &content.Resources{},
			BBox:    a.Rect,
		}, nil
	}

	inner := applyMargins(a.Rect, a.Margin)

	// When Sy=P, the pilcrow is drawn below the caret box, and Rect grows
	// where it does not yet take in the pilcrow, with Margin keeping the
	// caret box where it is.  A rectangle which already has room, such as
	// one this code grew before, stays as it is, so that generating the
	// appearance again leaves the annotation unchanged.
	if a.Symbol == "P" {
		// The pilcrow's baseline sits 0.8 of its size below the caret.
		// The glyph starts 0.3 of its size left of the caret's centre.
		// The bounds used here allow 0.3 of its size below the baseline
		// and 0.7 of its size for its width, where the glyph needs about
		// 0.19 and 0.52.  The resulting headroom of at least 1pt keeps
		// the glyph inside Rect even where Rect falls short of the bounds
		// by the slack allowed below.
		cx := (inner.LLx + inner.URx) / 2
		needLLx := pdf.Round(cx-pilcrowSize*0.4, 2)
		needLLy := pdf.Round(inner.LLy-pilcrowSize*1.1, 2)
		needURx := pdf.Round(cx+pilcrowSize*0.4, 2)

		// Margin is rounded down, so that the caret box it gives can only
		// grow; a rounded box could otherwise collapse, and the next run
		// would then take all of Rect for the caret box.  The grown box can
		// move the rounded pilcrow bounds by 0.01.  Rect therefore grows
		// only where it falls short by more than that, and when it grows it
		// takes in the pilcrow on every side, so that the next run finds no
		// shortfall beyond the slack.
		const slack = 0.015
		r := a.Rect
		if r.LLx > needLLx+slack || r.LLy > needLLy+slack || r.URx < needURx-slack {
			r.LLx = min(r.LLx, needLLx)
			r.LLy = min(r.LLy, needLLy)
			r.URx = max(r.URx, needURx)
			a.Rect = r
			a.Margin = []float64{
				pdf.Floor(inner.LLx-r.LLx, 2),
				pdf.Floor(inner.LLy-r.LLy, 2),
				pdf.Floor(r.URx-inner.URx, 2),
				pdf.Floor(r.URy-inner.URy, 2),
			}
			// draw from the box the rounded Margin gives, as later runs do
			inner = applyMargins(a.Rect, a.Margin)
		}
	}

	b := g.begin()

	// The caret is the proofreader's insertion mark drawn with a pen: two
	// strokes that leave the top of the mark together, run down the neck
	// leaning apart a little, and curve out into the feet.  Its width and
	// height follow the box, the line width does not: it is the border
	// width, as for any drawn markup, so the mark keeps the same weight
	// at every size.  A width of 0 leaves the mark undrawn, as it does
	// the border of any other annotation.  The strokes stay inside the
	// box by half their width, caps included.
	lw := annotation.EffectiveBorderWidth(a)
	if m := min(inner.Dx(), inner.Dy()); lw > m/2 {
		lw = m / 2
	}
	box := inner
	box.LLx += lw / 2
	box.LLy += lw / 2
	box.URx -= lw / 2
	box.URy -= lw / 2
	x := (box.LLx + box.URx) / 2
	w := box.Dx()
	h := box.Dy()

	// The neck takes the upper neckFraction of the height, the feet the
	// rest.  The neck leans out by lean of the half width.  The foot's
	// curve keeps the neck's direction for the first hold of the foot's
	// height and settles into the foot's own direction only within the
	// last turn of the half width.
	const (
		neckFraction = 0.65
		lean         = 0.1
		hold         = 0.4
		turn         = 0.3
	)
	neckH := h * neckFraction
	footH := h - neckH
	yb := box.LLy
	yt := box.URy
	yj := yb + footH

	if lw > 0 {
		rnd := func(v float64) float64 { return pdf.Round(v, 2) }
		b.SetStrokeColor(col)
		b.SetLineWidth(lw)
		b.SetLineCap(graphics.LineCapRound)
		b.SetLineJoin(graphics.LineJoinRound)
		for _, s := range []float64{-1, 1} {
			nx := x + s*w/2*lean
			// the neck's direction, continued into the first control point
			dx := (nx - x) / neckH
			b.MoveTo(rnd(x), rnd(yt))
			b.LineTo(rnd(nx), rnd(yj))
			b.CurveTo(
				rnd(nx+dx*footH*hold), rnd(yj-footH*hold),
				rnd(x+s*w/2*(1-turn)), rnd(yb+footH*turn*0.6),
				rnd(x+s*w/2), rnd(yb),
			)
		}
		b.Stroke()
	}

	y0 := yb

	// pilcrow below the caret base, at a fixed size: it is a symbol to be
	// read, not part of the mark's geometry, so it does not follow the box
	if a.Symbol == "P" {
		cx := (inner.LLx + inner.URx) / 2
		b.SetFillColor(col)
		b.TextBegin()
		b.TextSetFont(g.icons(), pilcrowSize)
		b.TextSetHorizontalScaling(1)
		b.TextFirstLine(pdf.Round(cx-pilcrowSize*0.3, 2), pdf.Round(y0-pilcrowSize*0.8, 2))
		b.TextShow("\u00B6")
		b.TextEnd()
	}

	return g.harvest(b, a.Rect, a.GetCommon())
}
