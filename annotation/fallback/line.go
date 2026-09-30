// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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
	"strings"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/path"
	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
)

func (g *Generator) addLineAppearance(a *annotation.Line) (*form.Form, error) {
	lw := annotation.EffectiveBorderWidth(a)
	dashPattern := annotation.EffectiveBorderDash(a)
	col := paint(a.Color)
	geom := newLineGeometry(a)
	bbox := calculateLineBBox(a, geom, lw, len(dashPattern) > 0)

	// the border width is the thickness of the line itself, and a width of 0
	// draws a hairline rather than nothing; only a file which names no ink
	// leaves the annotation with nothing to draw
	if col == nil || lw < 0 {
		a.Rect = bbox
		return g.harvest(g.begin(), bbox, nil)
	}

	capt := g.layoutCaption(a, geom.main, lw)
	if capt != nil && !capt.bbox.IsZero() {
		textBBox := roundOut(capt.bbox)
		bbox.Extend(&textBBox)
	}
	a.Rect = bbox

	b := g.begin()
	b.SetLineWidth(lw)
	b.SetStrokeColor(col)
	b.SetLineDash(dashPattern, 0)

	drawLine(b, a, geom, capt)
	if capt != nil {
		capt.draw(b, g.ContentFont(), col)
	}

	return g.harvest(b, bbox, a.GetCommon())
}

// calculateLineBBox returns the bounds of the line, its leader lines and its
// line endings, rounded outwards.  The result is the zero rectangle if
// nothing is drawn.
func calculateLineBBox(a *annotation.Line, geom lineGeometry, lw float64, dashed bool) pdf.Rectangle {
	// every piece is a single segment, so it carries caps and no join
	segments := append([][]vec.Vec2{geom.main}, geom.leaders...)
	// zero where the stroke draws nothing, which the line endings take as
	// absent
	bbox, _ := strokeBounds(segments, false, path.StrokeOptions{
		Width:  lw,
		Cap:    graphics.LineCapButt,
		Join:   graphics.LineJoinMiter,
		Dashed: dashed,
	})

	p1, p2 := geom.main[0], geom.main[1]
	le0 := normalizeLE(a.LineEndingStyle[0])
	le1 := normalizeLE(a.LineEndingStyle[1])
	if le0 != annotation.LineEndingStyleNone {
		lineEndingBBox(&bbox, le0, lineEndingInfo{At: p1, Dir: p1.Sub(p2)}, lw)
	}
	if le1 != annotation.LineEndingStyleNone {
		lineEndingBBox(&bbox, le1, lineEndingInfo{At: p2, Dir: p2.Sub(p1)}, lw)
	}
	if bbox.IsZero() {
		return bbox // nothing is drawn
	}

	// the line endings are drawn with two decimals, which can carry them a
	// little outside the exact geometry measured here
	return roundOut(bbox.Grow(pathPrecision + hairlineAllowance(lw)))
}

// lineGeometry is the geometry a Line annotation is drawn with, every point
// rounded to the two decimals it is written with.
type lineGeometry struct {
	// main is the line proper, which carries the line endings
	main []vec.Vec2

	// leaders holds one segment for each leader line, extension included;
	// it is nil for a line without leader lines
	leaders [][]vec.Vec2
}

// newLineGeometry lays out a Line annotation.
//
// Leader lines stand at the two points in L, perpendicular to the line, on
// its left for positive LL and on its right for negative LL.  A leader starts
// LLO away from its point and is |LL| long, so the line proper is drawn
// LLO+|LL| away from the points; the leader then runs on LLE past it.  A line
// too short to have a direction is drawn without leader lines.
func newLineGeometry(a *annotation.Line) lineGeometry {
	from := vec.Vec2{X: a.Coords[0], Y: a.Coords[1]}
	to := vec.Vec2{X: a.Coords[2], Y: a.Coords[3]}

	d := to.Sub(from)
	if a.LL == 0 || d.Length() < 0.1 {
		return lineGeometry{main: roundPoints([]vec.Vec2{from, to})}
	}

	// the side the leader lines grow towards
	side := d.Normal()
	if a.LL < 0 {
		side = side.Mul(-1)
	}
	ll := math.Abs(a.LL)

	offset := side.Mul(a.LLO + ll)
	leaderStart := side.Mul(a.LLO)
	leaderEnd := side.Mul(a.LLO + ll + a.LLE)
	return lineGeometry{
		main: roundPoints([]vec.Vec2{from.Add(offset), to.Add(offset)}),
		leaders: [][]vec.Vec2{
			roundPoints([]vec.Vec2{from.Add(leaderStart), from.Add(leaderEnd)}),
			roundPoints([]vec.Vec2{to.Add(leaderStart), to.Add(leaderEnd)}),
		},
	}
}

// drawLine draws the line proper with its line endings, and the leader
// lines if there are any.  An inline caption leaves a gap in the line proper.
func drawLine(b *builder.Builder, a *annotation.Line, geom lineGeometry, capt *lineCaption) {
	for _, seg := range geom.leaders {
		b.MoveTo(seg[0].X, seg[0].Y)
		b.LineTo(seg[1].X, seg[1].Y)
		b.Stroke()
	}

	le0 := normalizeLE(a.LineEndingStyle[0])
	le1 := normalizeLE(a.LineEndingStyle[1])
	fill := paint(a.FillColor)
	if capt == nil || !capt.gapped {
		drawOpenPolyline(b, geom.main, le0, le1, fill)
		return
	}

	// The two halves run from the line endings inwards, to the edges of the
	// gap.  The endings take their direction from the whole line, so that
	// they keep it even where the caption leaves no line beside them.  A
	// half which the gap reaches into the ending of is left out, since it
	// would run backwards through the ending.
	p1, p2 := geom.main[0], geom.main[1]
	d := p2.Sub(p1)
	length := d.Length()
	along := func(p vec.Vec2) float64 { return p.Sub(p1).Dot(d) / length }
	q := roundPoints([]vec.Vec2{
		p1.Add(d.Mul(capt.gap[0] / length)),
		p1.Add(d.Mul(capt.gap[1] / length)),
	})

	lw := b.State.GState.LineWidth
	e0 := newLineEnding(le0, lineEndingInfo{At: p1, Dir: p1.Sub(p2), FillColor: fill})
	e1 := newLineEnding(le1, lineEndingInfo{At: p2, Dir: p2.Sub(p1), FillColor: fill})
	c := roundPoints([]vec.Vec2{e0.connection(lw), e1.connection(lw)})

	e0.drawShape(b)
	if along(q[0]) > along(c[0]) {
		b.MoveTo(c[0].X, c[0].Y)
		b.LineTo(q[0].X, q[0].Y)
		b.Stroke()
	}
	if along(q[1]) < along(c[1]) {
		// the dashes run on as though the line continued behind the caption
		dash := b.State.GState.DashPattern
		if len(dash) > 0 {
			b.SetLineDash(dash, pdf.Round(along(q[1])-along(c[0]), 2))
		}
		b.MoveTo(q[1].X, q[1].Y)
		b.LineTo(c[1].X, c[1].Y)
		b.Stroke()
		if len(dash) > 0 {
			b.SetLineDash(dash, 0)
		}
	}
	e1.drawShape(b)
}

const (
	// captionFontSize is the size a Line annotation's caption is set in
	captionFontSize = 9

	// captionGap is the space an inline caption leaves between its text and
	// the line on either side
	captionGap = 2

	// captionSlack is the room, as a fraction of the font size, left round
	// the text of a caption in the annotation rectangle.  The glyphs are
	// measured with this library's metrics, while a viewer may draw a font
	// which is not embedded with a substitute whose glyphs reach a little
	// further; the rectangle clips anything beyond it.
	captionSlack = 0.15
)

// lineCaption is the layout of the caption of a Line annotation.
type lineCaption struct {
	// lines holds the lines of the caption text, and tm the text matrix
	// each of them is shown with
	lines []*font.GlyphSeq
	tm    []matrix.Matrix

	// gap is the part of the line proper an inline caption leaves free, as
	// distances from the start of the line; it is used only if gapped is set
	gap    [2]float64
	gapped bool

	// bbox is the rectangle the caption needs: the glyphs, the height of
	// the font and its advance widths, with [captionSlack] round them
	bbox pdf.Rectangle
}

// layoutCaption lays out the caption of a Line annotation along the line
// proper, main.  It returns nil where the annotation asks for no caption or
// has no text for one.
//
// The caption runs in the direction of the line, from its start to its end,
// so that a line running from right to left carries its caption upside down.
// An inline caption is centred on the line, which is broken round it, and a
// caption above the line stands on it, on the left of the line's direction.
// The caption offset moves the text along the line, in its direction, and
// across it, towards the side a caption above the line stands on.
func (g *Generator) layoutCaption(a *annotation.Line, main []vec.Vec2, lw float64) *lineCaption {
	if !a.Caption || a.Contents == "" {
		return nil
	}
	F := g.ContentFont()
	fontGeom := F.GetGeometry()

	p1, p2 := main[0], main[1]
	d := p2.Sub(p1)
	length := d.Length()
	dir := vec.Vec2{X: 1}
	if length >= 0.1 {
		dir = d.Mul(1 / length)
	}

	// the reading direction of the caption, rounded so that the text matrix
	// is written exactly as it is measured here
	right := vec.Vec2{X: pdf.Round(dir.X, 4), Y: pdf.Round(dir.Y, 4)}
	up := right.Rot90()

	var h, v float64
	if len(a.CaptionOffset) == 2 {
		h, v = a.CaptionOffset[0], a.CaptionOffset[1]
	}

	capt := &lineCaption{}
	width := 0.0
	for _, text := range captionLines(a.Contents) {
		seq := F.Layout(nil, captionFontSize, text)
		capt.lines = append(capt.lines, seq)
		width = max(width, seq.TotalWidth())
	}

	// the height of the first baseline above the line
	leading := fontGeom.Leading * captionFontSize
	extra := float64(len(capt.lines)-1) * leading
	var first float64
	if a.CaptionAbove {
		first = lw/2 - fontGeom.Descent*captionFontSize + extra
	} else {
		// Centring the capitals on the line leaves lower-case text, whose
		// bulk lies below the cap height, looking low; the band centred
		// reaches halfway between the x-height and the cap height instead.
		band := (fontGeom.CapHeight + fontGeom.XHeight) / 2 * captionFontSize
		first = (extra - band) / 2
	}

	mid := p1.Add(p2).Mul(0.5)
	for i, seq := range capt.lines {
		x := h - seq.TotalWidth()/2
		y := v + first - float64(i)*leading
		origin := mid.Add(right.Mul(x)).Add(up.Mul(y))
		M := matrix.Matrix{right.X, right.Y, up.X, up.Y,
			pdf.Round(origin.X, 2), pdf.Round(origin.Y, 2)}
		capt.tm = append(capt.tm, M)

		box := pdf.Rectangle{
			LLx: 0,
			LLy: fontGeom.Descent * captionFontSize,
			URx: seq.TotalWidth(),
			URy: fontGeom.Ascent * captionFontSize,
		}
		if ink := fontGeom.BoundingBox(captionFontSize, seq); ink != nil && !ink.IsZero() {
			box.Extend(ink)
		}
		box = box.Grow(captionSlack * captionFontSize)
		placed := pdf.RectangleFromPoints(
			M.Apply(vec.Vec2{X: box.LLx, Y: box.LLy}),
			M.Apply(vec.Vec2{X: box.URx, Y: box.LLy}),
			M.Apply(vec.Vec2{X: box.LLx, Y: box.URy}),
			M.Apply(vec.Vec2{X: box.URx, Y: box.URy}),
		)
		capt.bbox.Extend(&placed)
	}

	if !a.CaptionAbove && length >= 0.1 {
		centre := length/2 + h
		half := width/2 + captionGap
		capt.gap = [2]float64{
			min(max(centre-half, 0), length),
			min(max(centre+half, 0), length),
		}
		capt.gapped = true
	}
	return capt
}

// captionLines splits a caption into its lines, at any of the line breaks
// CR, LF and CR LF.
func captionLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.Split(text, "\n")
}

// draw shows the caption in the given font and colour.
func (capt *lineCaption) draw(b *builder.Builder, F font.Layouter, col color.Color) {
	b.TextBegin()
	b.TextSetFont(F, captionFontSize)
	b.SetFillColor(col)
	b.TextSetHorizontalScaling(1)
	b.TextSetRise(0)
	for i, seq := range capt.lines {
		b.TextSetMatrix(capt.tm[i])
		b.TextShowGlyphs(seq)
	}
	b.TextEnd()
}
