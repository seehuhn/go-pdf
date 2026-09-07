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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/extgstate"
	"seehuhn.de/go/pdf/graphics/form"
)

// pilcrowSize is the font size, in points, of the paragraph symbol drawn
// under a caret whose Symbol is P.
const pilcrowSize = 10.0

func (g *Generator) addCaretAppearance(a *annotation.Caret) (*form.Form, error) {
	col := a.Color
	if col == nil {
		return &form.Form{
			Content: nil,
			Res:     &content.Resources{},
			BBox:    a.Rect,
		}, nil
	}

	inner := applyMargins(a.Rect, a.Margin)

	// when Sy=P, expand Rect and Margin downward and sideways to make room
	// for the pilcrow, which is wider than a narrow caret box
	if a.Symbol == "P" {
		// the pilcrow's baseline sits 0.8 of its size below the caret and
		// its stems reach a further 0.3 of its size down
		extraY := pilcrowSize * 1.1
		extraX := pilcrowSize * 0.4
		a.Rect.LLy -= extraY
		a.Rect.LLx -= extraX
		a.Rect.URx += extraX
		if len(a.Margin) == 4 {
			a.Margin = []float64{
				a.Margin[0] + extraX, a.Margin[1] + extraY, a.Margin[2] + extraX, a.Margin[3],
			}
		} else {
			a.Margin = []float64{extraX, extraY, extraX, 0}
		}
	}

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

	return harvest(b, a.Rect)
}
