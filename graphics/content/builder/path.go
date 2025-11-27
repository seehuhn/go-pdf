package builder

import (
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
)

// Path Construction Operators

// MoveTo starts a new path at the given coordinates.
//
// This implements the PDF graphics operator "m".
func (b *Builder) MoveTo(x, y float64) {
	b.emit(content.OpMoveTo, pdf.Number(x), pdf.Number(y))
}

// LineTo appends a straight line segment to the current path.
//
// This implements the PDF graphics operator "l".
func (b *Builder) LineTo(x, y float64) {
	b.emit(content.OpLineTo, pdf.Number(x), pdf.Number(y))
}

// CurveTo appends a cubic Bezier curve to the current path.
//
// This implements the PDF graphics operators "c", "v", and "y".
func (b *Builder) CurveTo(x1, y1, x2, y2, x3, y3 float64) {
	x0, y0 := b.State.Param.CurrentX, b.State.Param.CurrentY
	if nearlyEqual(x0, x1) && nearlyEqual(y0, y1) {
		// first control point at current point → "v"
		b.emit(content.OpCurveToV,
			pdf.Number(x2), pdf.Number(y2),
			pdf.Number(x3), pdf.Number(y3))
	} else if nearlyEqual(x2, x3) && nearlyEqual(y2, y3) {
		// second control point at end point → "y"
		b.emit(content.OpCurveToY,
			pdf.Number(x1), pdf.Number(y1),
			pdf.Number(x3), pdf.Number(y3))
	} else {
		b.emit(content.OpCurveTo,
			pdf.Number(x1), pdf.Number(y1),
			pdf.Number(x2), pdf.Number(y2),
			pdf.Number(x3), pdf.Number(y3))
	}
}

// ClosePath closes the current subpath.
//
// This implements the PDF graphics operator "h".
func (b *Builder) ClosePath() {
	b.emit(content.OpClosePath)
}

// Rectangle appends a rectangle to the current path as a closed subpath.
//
// This implements the PDF graphics operator "re".
func (b *Builder) Rectangle(x, y, width, height float64) {
	b.emit(content.OpRectangle,
		pdf.Number(x), pdf.Number(y),
		pdf.Number(width), pdf.Number(height))
}

// Path Painting Operators

// Stroke strokes the current path.
//
// This implements the PDF graphics operator "S".
func (b *Builder) Stroke() {
	b.emit(content.OpStroke)
}

// CloseAndStroke closes and strokes the current path.
//
// This implements the PDF graphics operator "s".
func (b *Builder) CloseAndStroke() {
	b.emit(content.OpCloseAndStroke)
}

// Fill fills the current path using the nonzero winding number rule.
//
// This implements the PDF graphics operator "f".
func (b *Builder) Fill() {
	b.emit(content.OpFill)
}

// FillEvenOdd fills the current path using the even-odd rule.
//
// This implements the PDF graphics operator "f*".
func (b *Builder) FillEvenOdd() {
	b.emit(content.OpFillEvenOdd)
}

// FillAndStroke fills and strokes the current path.
//
// This implements the PDF graphics operator "B".
func (b *Builder) FillAndStroke() {
	b.emit(content.OpFillAndStroke)
}

// FillAndStrokeEvenOdd fills and strokes the current path using the even-odd rule.
//
// This implements the PDF graphics operator "B*".
func (b *Builder) FillAndStrokeEvenOdd() {
	b.emit(content.OpFillAndStrokeEvenOdd)
}

// CloseFillAndStroke closes, fills and strokes the current path.
//
// This implements the PDF graphics operator "b".
func (b *Builder) CloseFillAndStroke() {
	b.emit(content.OpCloseFillAndStroke)
}

// CloseFillAndStrokeEvenOdd closes, fills and strokes the current path
// using the even-odd rule.
//
// This implements the PDF graphics operator "b*".
func (b *Builder) CloseFillAndStrokeEvenOdd() {
	b.emit(content.OpCloseFillAndStrokeEvenOdd)
}

// EndPath ends the path without filling or stroking it.
//
// This implements the PDF graphics operator "n".
func (b *Builder) EndPath() {
	b.emit(content.OpEndPath)
}

// Clipping Path Operators

// ClipNonZero sets the current clipping path using the nonzero winding number rule.
//
// This implements the PDF graphics operator "W".
func (b *Builder) ClipNonZero() {
	b.emit(content.OpClipNonZero)
}

// ClipEvenOdd sets the current clipping path using the even-odd rule.
//
// This implements the PDF graphics operator "W*".
func (b *Builder) ClipEvenOdd() {
	b.emit(content.OpClipEvenOdd)
}

// Convenience Path Methods

// Circle appends a circle to the current path, as a closed subpath.
//
// This is a convenience function, which uses [Builder.MoveTo] and
// [Builder.CurveTo] to draw the circle.
func (b *Builder) Circle(x, y, radius float64) {
	b.arc(x, y, radius, 0, 2*math.Pi, true)
	b.ClosePath()
}

// MoveToArc appends a circular arc to the current path,
// starting a new subpath.
//
// This is a convenience function, which uses [Builder.MoveTo] and
// [Builder.CurveTo] to draw the arc.
func (b *Builder) MoveToArc(x, y, radius, startAngle, endAngle float64) {
	b.arc(x, y, radius, startAngle, endAngle, true)
}

// LineToArc appends a circular arc to the current subpath,
// connecting the previous point to the arc using a straight line.
//
// This is a convenience function, which uses [Builder.LineTo] and
// [Builder.CurveTo] to draw the arc.
func (b *Builder) LineToArc(x, y, radius, startAngle, endAngle float64) {
	b.arc(x, y, radius, startAngle, endAngle, false)
}

// arc appends a circular arc to the current path.
func (b *Builder) arc(x, y, radius, startAngle, endAngle float64, move bool) {
	// rounding precision based on radius
	digits := max(1, 2-int(math.Round(math.Log10(radius))))

	// also see https://www.tinaja.com/glib/bezcirc2.pdf
	// from https://pomax.github.io/bezierinfo/ , section 42

	nSegment := int(math.Ceil(math.Abs(endAngle-startAngle) / (0.5 * math.Pi)))
	dPhi := (endAngle - startAngle) / float64(nSegment)
	k := 4.0 / 3.0 * radius * math.Tan(dPhi/4)

	phi := startAngle
	x0 := x + radius*math.Cos(phi)
	y0 := y + radius*math.Sin(phi)
	if move {
		b.MoveTo(pdf.Round(x0, digits), pdf.Round(y0, digits))
	} else {
		b.LineTo(pdf.Round(x0, digits), pdf.Round(y0, digits))
	}

	for range nSegment {
		x1 := x0 - k*math.Sin(phi)
		y1 := y0 + k*math.Cos(phi)
		phi += dPhi
		x3 := x + radius*math.Cos(phi)
		y3 := y + radius*math.Sin(phi)
		x2 := x3 + k*math.Sin(phi)
		y2 := y3 - k*math.Cos(phi)
		b.CurveTo(
			pdf.Round(x1, digits), pdf.Round(y1, digits),
			pdf.Round(x2, digits), pdf.Round(y2, digits),
			pdf.Round(x3, digits), pdf.Round(y3, digits))
		x0 = x3
		y0 = y3
	}
}
