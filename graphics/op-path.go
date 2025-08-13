// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package graphics

import (
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
)

// This file implements the "Path construction operators" and "Path-painting
// operators".  The operators implemented here are defined in tables 58, 59
// and 60 of ISO 32000-2:2020.

// MoveTo starts a new path at the given coordinates.
//
// This implements the PDF graphics operator "m".
func (w *Writer) MoveTo(x, y float64) {
	if !w.isValid("MoveTo", objPage|objPath) {
		return
	}
	w.currentObject = objPath

	w.StartX, w.StartY = x, y
	w.CurrentX, w.CurrentY = x, y

	_, w.Err = fmt.Fprintln(w.Content, w.coord(x), w.coord(y), "m")
}

// LineTo appends a straight line segment to the current path.
//
// This implements the PDF graphics operator "l".
func (w *Writer) LineTo(x, y float64) {
	if !w.isValid("LineTo", objPath) {
		return
	}

	w.CurrentX, w.CurrentY = x, y

	_, w.Err = fmt.Fprintln(w.Content, w.coord(x), w.coord(y), "l")
}

// CurveTo appends a cubic Bezier curve to the current path.
//
// This implements the PDF graphics operators "c", "v", and "y".
func (w *Writer) CurveTo(x1, y1, x2, y2, x3, y3 float64) {
	if !w.isValid("CurveTo", objPath) {
		return
	}

	x0, y0 := w.CurrentX, w.CurrentY
	w.CurrentX, w.CurrentY = x3, y3

	if nearlyEqual(x0, x1) && nearlyEqual(y0, y1) {
		_, w.Err = fmt.Fprintln(w.Content, w.coord(x2), w.coord(y2), w.coord(x3), w.coord(y3), "v")
	} else if nearlyEqual(x2, x3) && nearlyEqual(y2, y3) {
		_, w.Err = fmt.Fprintln(w.Content, w.coord(x1), w.coord(y1), w.coord(x3), w.coord(y3), "y")
	} else {
		_, w.Err = fmt.Fprintln(w.Content, w.coord(x1), w.coord(y1), w.coord(x2), w.coord(y2), w.coord(x3), w.coord(y3), "c")
	}
}

// ClosePath closes the current subpath.
//
// This implements the PDF graphics operator "h".
func (w *Writer) ClosePath() {
	if !w.isValid("ClosePath", objPath) {
		return
	}

	w.CurrentX, w.CurrentY = w.StartX, w.StartY

	_, w.Err = fmt.Fprintln(w.Content, "h")
}

// Rectangle appends a rectangle to the current path as a closed subpath.
//
// Note that the final two arguments are the width and height of the rectangle,
// not the coordinates of the upper right corner.
//
// This implements the PDF graphics operator "re".
func (w *Writer) Rectangle(x, y, width, height float64) {
	if !w.isValid("Rectangle", objPage|objPath) {
		return
	}
	w.currentObject = objPath

	w.StartX, w.StartY = x, y
	w.CurrentX, w.CurrentY = x, y

	_, w.Err = fmt.Fprintln(w.Content, w.coord(x), w.coord(y), w.coord(width), w.coord(height), "re")
}

// Stroke strokes the current path.
//
// This implements the PDF graphics operator "S".
func (w *Writer) Stroke() {
	if !w.isValid("Stroke", objPath|objClippingPath) {
		return
	}
	w.currentObject = objPage

	if err := w.mustBeSet(strokeStateBits); err != nil {
		w.Err = err
		return
	}

	_, w.Err = fmt.Fprintln(w.Content, "S")
}

// CloseAndStroke closes and strokes the current path.
// This has the same effect as [Writer.ClosePath] followed by [Writer.Stroke].
//
// This implements the PDF graphics operator "s".
func (w *Writer) CloseAndStroke() {
	if !w.isValid("CloseAndStroke", objPath|objClippingPath) {
		return
	}
	w.currentObject = objPage

	if err := w.mustBeSet(strokeStateBits); err != nil {
		w.Err = err
		return
	}

	_, w.Err = fmt.Fprintln(w.Content, "s")
}

// Fill fills the current path, using the nonzero winding number rule.  Any
// subpaths that are open are implicitly closed before being filled.
//
// This implements the PDF graphics operator "f".
func (w *Writer) Fill() {
	if !w.isValid("Fill", objPath|objClippingPath) {
		return
	}
	w.currentObject = objPage

	if err := w.mustBeSet(fillStateBits); err != nil {
		w.Err = err
		return
	}

	_, w.Err = fmt.Fprintln(w.Content, "f")
}

// FillEvenOdd fills the current path, using the even-odd rule.  Any
// subpaths that are open are implicitly closed before being filled.
//
// This implements the PDF graphics operator "f*".
func (w *Writer) FillEvenOdd() {
	if !w.isValid("FillEvenOdd", objPath|objClippingPath) {
		return
	}
	w.currentObject = objPage

	if err := w.mustBeSet(fillStateBits); err != nil {
		w.Err = err
		return
	}

	_, w.Err = fmt.Fprintln(w.Content, "f*")
}

// FillAndStroke fills and strokes the current path.  Any subpaths that are
// open are implicitly closed before being filled.
//
// This implements the PDF graphics operator "B".
func (w *Writer) FillAndStroke() {
	if !w.isValid("FillAndStroke", objPath|objClippingPath) {
		return
	}
	w.currentObject = objPage

	if err := w.mustBeSet(strokeStateBits | fillStateBits); err != nil {
		w.Err = err
		return
	}

	_, w.Err = fmt.Fprintln(w.Content, "B")
}

// FillAndStrokeEvenOdd fills and strokes the current path, using the even-odd
// rule for filling.  Any subpaths that are open are implicitly closed before
// being filled.
//
// This implements the PDF graphics operator "B*".
func (w *Writer) FillAndStrokeEvenOdd() {
	if !w.isValid("FillAndStroke", objPath|objClippingPath) {
		return
	}
	w.currentObject = objPage

	if err := w.mustBeSet(strokeStateBits | fillStateBits); err != nil {
		w.Err = err
		return
	}

	_, w.Err = fmt.Fprintln(w.Content, "B*")
}

// CloseFillAndStroke closes, fills and strokes the current path. This has the
// same effect as [Writer.ClosePath] followed by [Writer.FillAndStroke].
//
// This implements the PDF graphics operator "b".
func (w *Writer) CloseFillAndStroke() {
	if !w.isValid("FillAndStroke", objPath|objClippingPath) {
		return
	}
	w.currentObject = objPage

	if err := w.mustBeSet(strokeStateBits | fillStateBits); err != nil {
		w.Err = err
		return
	}

	_, w.Err = fmt.Fprintln(w.Content, "b")
}

// CloseFillAndStrokeEvenOdd closes, fills and strokes the current path, using
// the even-odd rule for filling.  This has the same effect as
// [Writer.ClosePath] followed by [Writer.FillAndStrokeEvenOdd].
//
// This implements the PDF graphics operator "b*".
func (w *Writer) CloseFillAndStrokeEvenOdd() {
	if !w.isValid("FillAndStroke", objPath|objClippingPath) {
		return
	}
	w.currentObject = objPage

	if err := w.mustBeSet(strokeStateBits | fillStateBits); err != nil {
		w.Err = err
		return
	}

	_, w.Err = fmt.Fprintln(w.Content, "b*")
}

// EndPath ends the path without filling and stroking it.
// This is for use after the [Writer.ClipNonZero] and [Writer.ClipEvenOdd] methods.
//
// This implements the PDF graphics operator "n".
func (w *Writer) EndPath() {
	if !w.isValid("EndPath", objPath|objClippingPath) {
		return
	}
	w.currentObject = objPage

	_, w.Err = fmt.Fprintln(w.Content, "n")
}

// ClipNonZero sets the current clipping path using the nonzero winding number
// rule.
//
// This implements the PDF graphics operator "W".
func (w *Writer) ClipNonZero() {
	if !w.isValid("ClipNonZero", objPath) {
		return
	}
	w.currentObject = objClippingPath

	_, w.Err = fmt.Fprintln(w.Content, "W")
}

// ClipEvenOdd sets the current clipping path using the even-odd rule.
//
// This implements the PDF graphics operator "W*".
func (w *Writer) ClipEvenOdd() {
	if !w.isValid("ClipEvenOdd", objPath) {
		return
	}
	w.currentObject = objClippingPath

	_, w.Err = fmt.Fprintln(w.Content, "W*")
}

// Circle appends a circle to the current path, as a closed subpath.
//
// This is a convenience function, which uses [Writer.MoveTo] and
// [Writer.CurveTo] to draw the circle.
func (w *Writer) Circle(x, y, radius float64) {
	if !w.isValid("Circle", objPage|objPath) {
		return
	}

	w.arc(x, y, radius, 0, 2*math.Pi, true)
	w.ClosePath()
}

// MoveToArc appends a circular arc to the current path,
// starting a new subpath.
//
// This is a convenience function, which uses [Writer.MoveTo] and
// [Writer.CurveTo] to draw the arc.
func (w *Writer) MoveToArc(x, y, radius, startAngle, endAngle float64) {
	if !w.isValid("MoveToArc", objPage|objPath) {
		return
	}

	w.arc(x, y, radius, startAngle, endAngle, true)
}

// LineToArc appends a circular arc to the current subpath,
// connecting the previous point to the arc using a straight line.
//
// This is a convenience function, which uses [Writer.LineTo] and
// [Writer.CurveTo] to draw the arc.
func (w *Writer) LineToArc(x, y, radius, startAngle, endAngle float64) {
	if !w.isValid("LineToArc", objPath) {
		return
	}

	w.arc(x, y, radius, startAngle, endAngle, false)
}

// arc appends a circular arc to the current path.
func (w *Writer) arc(x, y, radius, startAngle, endAngle float64, move bool) {
	w.currentObject = objPath

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
		w.MoveTo(pdf.Round(x0, digits), pdf.Round(y0, digits))
	} else {
		w.LineTo(pdf.Round(x0, digits), pdf.Round(y0, digits))
	}

	for range nSegment {
		x1 := x0 - k*math.Sin(phi)
		y1 := y0 + k*math.Cos(phi)
		phi += dPhi
		x3 := x + radius*math.Cos(phi)
		y3 := y + radius*math.Sin(phi)
		x2 := x3 + k*math.Sin(phi)
		y2 := y3 - k*math.Cos(phi)
		w.CurveTo(pdf.Round(x1, int(digits)), pdf.Round(y1, int(digits)), pdf.Round(x2, int(digits)), pdf.Round(y2, int(digits)), pdf.Round(x3, int(digits)), pdf.Round(y3, int(digits)))
		x0 = x3
		y0 = y3
	}
}
