// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package redactmark draws the mark which replaces redacted content: an
// overlay form, or a fill colour with overlay text.
package redactmark

import (
	"math"
	"sort"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/internal/brand"
	"seehuhn.de/go/pdf/internal/daparse"
)

// maxGlyphs limits the overlay text drawn for one mark, so that the work
// stays bounded however long the text and however many the regions.
const maxGlyphs = 50000

// autoFitFrac is the fraction of a region's height which automatically sized
// text fills.
const autoFitFrac = 0.8

// Quad is a quadrilateral given by its corners in the order upper-left,
// upper-right, lower-left, lower-right, as for the QuadPoints of a text
// markup annotation.
type Quad [4]vec.Vec2

// RectQuad returns the quadrilateral with the sides of r.
func RectQuad(r pdf.Rectangle) Quad {
	return Quad{
		{X: r.LLx, Y: r.URy},
		{X: r.URx, Y: r.URy},
		{X: r.LLx, Y: r.LLy},
		{X: r.URx, Y: r.LLy},
	}
}

// QuadsFromPoints splits QuadPoints into quadrilaterals.  Trailing points
// which do not make up a whole quadrilateral are ignored.
func QuadsFromPoints(pts []vec.Vec2) []Quad {
	quads := make([]Quad, 0, len(pts)/4)
	for i := 0; i+4 <= len(pts); i += 4 {
		quads = append(quads, Quad(pts[i:i+4]))
	}
	return quads
}

// Path adds the outline of q to the current path of b, visiting the corners
// in order around the boundary.
func (q Quad) Path(b *builder.Builder) {
	b.MoveTo(pdf.Round(q[0].X, 2), pdf.Round(q[0].Y, 2))
	b.LineTo(pdf.Round(q[1].X, 2), pdf.Round(q[1].Y, 2))
	b.LineTo(pdf.Round(q[3].X, 2), pdf.Round(q[3].Y, 2))
	b.LineTo(pdf.Round(q[2].X, 2), pdf.Round(q[2].Y, 2))
	b.ClosePath()
}

// Text describes the overlay text of a redaction mark.
type Text struct {
	// Font is the font the text is drawn in.
	Font font.Layouter

	// Size is the font size.  The value 0 selects a size which fits the
	// height of each region and, unless Repeat is set, also its width.
	Size float64

	// Color is the colour of the text.
	Color color.Color

	// String is the text to draw.
	String string

	// Align is the horizontal alignment of the text within each region.
	Align pdf.TextAlign

	// Repeat repeats the text, separated by spaces, to fill each region line
	// by line.  Otherwise the text is drawn once, on a single line which is
	// centred vertically.
	Repeat bool
}

// Mark describes what replaces the content of a redacted region.
type Mark struct {
	// Quads is the region.
	Quads []Quad

	// Rect is the annotation rectangle which anchors Overlay.
	Rect pdf.Rectangle

	// Overlay, if not nil, is drawn with its origin at the lower-left corner
	// of Rect.  Fill and Text are then ignored.
	Overlay *form.Form

	// Fill, if not nil, fills the region.
	Fill color.Color

	// Text, if not nil, is drawn over the filled region and clipped to it.
	Text *Text
}

// FromAnnotation returns the mark described by a redaction annotation.  The
// region is given by the annotation's QuadPoints, one quadrilateral per four
// points, or by Rect if there are fewer than four points.  If the annotation
// has overlay text, getFont is called once to obtain the font for the text;
// otherwise getFont is not called.
func FromAnnotation(a *annotation.Redact, getFont func() font.Layouter) *Mark {
	var quads []Quad
	if len(a.QuadPoints) >= 4 {
		quads = QuadsFromPoints(a.QuadPoints)
	} else {
		quads = []Quad{RectQuad(a.Rect)}
	}
	m := &Mark{
		Quads:   quads,
		Rect:    a.Rect,
		Overlay: a.Overlay,
		Fill:    a.FillColor,
	}
	if !a.OverlayText.IsZero() {
		size, col := daparse.Parse(a.DefaultAppearance, brand.Ink)
		m.Text = &Text{
			Font:   getFont(),
			Size:   size,
			Color:  col,
			String: a.OverlayText.String(),
			Align:  a.Align,
			Repeat: a.Repeat,
		}
	}
	return m
}

// Draw draws the mark into b.  The graphics state of b is left as it was.
//
// The text parameters of b must have their default values: no character or
// word spacing, horizontal scaling 100, no text rise and rendering mode 0.
func Draw(b *builder.Builder, m *Mark) {
	_ = DrawCharged(b, m, func(int64) error { return nil })
}

// DrawCharged is like [Draw], and accounts for the work done: before the
// overlay text is laid out, it calls charge with the length of the text in
// bytes, and before glyphs and quadrilaterals are drawn, it calls charge with
// their number.  If charge returns an error, DrawCharged stops and returns
// the error; the content of b is then incomplete.
func DrawCharged(b *builder.Builder, m *Mark, charge func(n int64) error) error {
	if m.Overlay != nil {
		if err := charge(1); err != nil {
			return err
		}
		b.PushGraphicsState()
		b.Transform(matrix.Translate(pdf.Round(m.Rect.LLx, 2), pdf.Round(m.Rect.LLy, 2)))
		b.DrawXObject(m.Overlay)
		b.PopGraphicsState()
		return nil
	}

	if m.Fill != nil {
		if err := charge(int64(len(m.Quads))); err != nil {
			return err
		}
		b.PushGraphicsState()
		b.SetFillColor(m.Fill)
		for _, q := range m.Quads {
			q.Path(b)
		}
		b.Fill()
		b.PopGraphicsState()
	}

	if m.Text != nil && m.Text.String != "" && len(m.Quads) > 0 {
		s := m.Text.String
		if m.Text.Repeat {
			s += " "
		}
		if err := charge(int64(len(s))); err != nil {
			return err
		}
		l := newTextLayout(m.Text.Font, s)
		budget := maxGlyphs
		for _, q := range m.Quads {
			if err := drawText(b, q, m.Text, l, &budget, charge); err != nil {
				return err
			}
		}
	}
	return nil
}

// drawText draws the text in the frame of the quadrilateral, whose x-axis
// runs from the lower-left towards the lower-right corner.  The text is
// given by l, and budget is the number of glyphs which may still be drawn.
// The quadrilateral and the glyphs are charged as for [DrawCharged].
func drawText(b *builder.Builder, q Quad, t *Text, l *textLayout, budget *int, charge func(n int64) error) error {
	if err := charge(1); err != nil {
		return err
	}
	ll := q[2]
	ex := q[3].Sub(ll)
	w := ex.Length()
	if !(w > 0) {
		return nil
	}
	u := ex.Mul(1 / w)
	nrm := vec.Vec2{X: -u.Y, Y: u.X}
	h := min(q[0].Sub(ll).Dot(nrm), q[1].Sub(ll).Dot(nrm))
	if !(h > 0) {
		return nil
	}

	// horizontal extent of the region in the text frame
	lo, hi := 0.0, w
	for _, p := range q[:2] {
		x := p.Sub(ll).Dot(u)
		lo, hi = min(lo, x), max(hi, x)
	}

	geom := t.Font.GetGeometry()
	size := t.Size
	if size <= 0 {
		// without a usable font height only the width bounds the size
		size = math.Inf(1)
		if fh := geom.Ascent - geom.Descent; fh > 0 {
			size = autoFitFrac * h / fh
		}
		if tw := l.width(); !t.Repeat && tw > 0 {
			size = min(size, w/tw)
		}
		if math.IsInf(size, 1) {
			return nil
		}
	}
	size = pdf.Round(size, 2)
	if !(size > 0) {
		return nil
	}

	var qAlign float64
	switch t.Align {
	case pdf.TextAlignCenter:
		qAlign = 0.5
	case pdf.TextAlignRight:
		qAlign = 1
	}

	// the line is reps copies of l, aligned within the width of the region
	period := l.width() * size
	reps := 1.0
	if t.Repeat && period > 0 {
		reps = math.Ceil(w/period) + 1
	}
	x0 := (w - reps*period) * qAlign

	gg, cost := l.line(x0, size, reps, lo, hi, *budget)
	*budget -= cost
	if err := charge(int64(cost)); err != nil {
		return err
	}
	if len(gg.Seq) == 0 {
		return nil
	}

	lines := 1
	var leading float64
	if t.Repeat {
		leading = pdf.Round(geom.Leading*size, 2)
		if leading > 0 {
			more := min(math.Floor(h/leading)-1, float64(*budget/len(gg.Seq)))
			lines += int(max(more, 0))
		}
		*budget -= (lines - 1) * len(gg.Seq)
		if err := charge(int64((lines - 1) * len(gg.Seq))); err != nil {
			return err
		}
	}

	b.PushGraphicsState()
	q.Path(b)
	b.ClipNonZero()
	b.EndPath()
	ux, uy := pdf.Round(u.X, 4), pdf.Round(u.Y, 4)
	b.Transform(matrix.Matrix{ux, uy, -uy, ux, pdf.Round(ll.X, 2), pdf.Round(ll.Y, 2)})

	b.TextBegin()
	b.TextSetFont(t.Font, size)
	b.SetFillColor(t.Color)

	if t.Repeat {
		y := pdf.Round(h-(h-float64(lines)*leading)/2-geom.Ascent*size, 2)
		for i := range lines {
			if i == 0 {
				b.TextFirstLine(0, y)
			} else {
				b.TextSecondLine(0, -leading)
			}
			b.TextShowGlyphs(gg)
		}
	} else {
		b.TextFirstLine(0, pdf.Round(h/2-geom.CapHeight*size/2, 2))
		b.TextShowGlyphs(gg)
	}

	b.TextEnd()
	b.PopGraphicsState()
	return nil
}

// textLayout is a string laid out at font size 1, with the horizontal ink
// bounds needed to find the glyphs which can be visible in a window.
type textLayout struct {
	glyphs []font.Glyph

	// start[i] is the position of glyph i, start[len(glyphs)] the end of
	// the string
	start []float64

	// inkHi[i] is the right-most ink edge among the glyphs 0, ..., i, and
	// inkLo[i] the left-most ink edge among the glyphs i, ..., n-1, so that
	// both are non-decreasing in i
	inkHi, inkLo []float64
}

func newTextLayout(F font.Layouter, s string) *textLayout {
	seq := F.Layout(nil, 1, s)
	geom := F.GetGeometry()

	n := len(seq.Seq)
	l := &textLayout{
		glyphs: seq.Seq,
		start:  make([]float64, n+1),
		inkHi:  make([]float64, n),
		inkLo:  make([]float64, n),
	}
	x := seq.Skip
	hi := math.Inf(-1)
	for i, g := range seq.Seq {
		l.start[i] = x
		hi = max(hi, x+geom.GlyphExtents[g.GID].URx)
		l.inkHi[i] = hi
		x += g.Advance
	}
	l.start[n] = x
	lo := math.Inf(1)
	for i := n - 1; i >= 0; i-- {
		lo = min(lo, l.start[i]+geom.GlyphExtents[l.glyphs[i].GID].LLx)
		l.inkLo[i] = lo
	}
	return l
}

// width returns the advance width of the string at font size 1.
func (l *textLayout) width() float64 {
	return l.start[len(l.glyphs)]
}

// line lays out reps copies of the string at the given font size, the first
// starting at x0 and each following directly after the one before.  Of
// these glyphs, line returns those whose ink can fall inside the window
// from lo to hi, at most limit of them.  The returned cost, at most limit,
// is the number of glyphs returned plus the number of copies which
// contributed none.
func (l *textLayout) line(x0, size, reps, lo, hi float64, limit int) (*font.GlyphSeq, int) {
	seq := &font.GlyphSeq{}
	n := len(l.glyphs)
	if n == 0 || limit <= 0 {
		return seq, 0
	}

	// copies whose ink can reach the window
	period := l.width() * size
	kMin, kMax := 0.0, 0.0
	if period > 0 {
		kMin = max(math.Ceil((lo-x0-size*l.inkHi[n-1])/period), 0)
		kMax = min(math.Floor((hi-x0-size*l.inkLo[0])/period), reps-1)
	}

	cost := 0
	var end float64 // where the last glyph in seq ends
	for k := kMin; k <= kMax && cost < limit; k++ {
		base := x0 + k*period
		i := sort.Search(n, func(i int) bool { return base+size*l.inkHi[i] >= lo })
		j := sort.Search(n, func(i int) bool { return base+size*l.inkLo[i] > hi })
		j = min(j, i+limit-cost)
		if i >= j {
			cost++
			continue
		}
		cost += j - i

		pos := base + size*l.start[i]
		if len(seq.Seq) == 0 {
			seq.Skip = pos
		} else {
			seq.Seq[len(seq.Seq)-1].Advance += pos - end
		}
		for _, g := range l.glyphs[i:j] {
			g.Advance *= size
			g.Rise *= size
			seq.Seq = append(seq.Seq, g)
		}
		end = base + size*l.start[j]
	}
	return seq, cost
}
