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

import "fmt"

// This file implements the "Path construction operators" and "Path-painting
// operators". These operators are defined in tables 58 and 59 of ISO
// 32000-2:2020.

// MoveTo starts a new path at the given coordinates.
//
// This implements the PDF graphics operator "m".
func (w *Writer) MoveTo(x, y float64) {
	if !w.isValid("MoveTo", objPage|objPath) {
		return
	}
	w.currentObject = objPath

	w.startX, w.startY = x, y
	w.currentX, w.currentY = x, y
	w.pathIsClosed = false

	_, w.Err = fmt.Fprintln(w.Content, w.coord(x), w.coord(y), "m")
}

// LineTo appends a straight line segment to the current path.
//
// This implements the PDF graphics operator "l".
func (w *Writer) LineTo(x, y float64) {
	if !w.isValid("LineTo", objPath) {
		return
	}

	w.currentX, w.currentY = x, y
	w.pathIsClosed = false

	_, w.Err = fmt.Fprintln(w.Content, w.coord(x), w.coord(y), "l")
}

// CurveTo appends a cubic Bezier curve to the current path.
//
// This implements the PDF graphics operators "c", "v", and "y".
func (w *Writer) CurveTo(x1, y1, x2, y2, x3, y3 float64) {
	if !w.isValid("CurveTo", objPath) {
		return
	}

	x0, y0 := w.currentX, w.currentY
	w.currentX, w.currentY = x3, y3
	w.pathIsClosed = false

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

	w.currentX, w.currentY = w.startX, w.startY
	w.pathIsClosed = true

	_, w.Err = fmt.Fprintln(w.Content, "h")
}

// Rectangle appends a rectangle to the current path as a closed subpath.
//
// This implements the PDF graphics operator "re".
func (w *Writer) Rectangle(x, y, width, height float64) {
	if !w.isValid("Rectangle", objPage|objPath) {
		return
	}
	w.currentObject = objPath

	w.startX, w.startY = x, y
	w.currentX, w.currentY = x, y
	w.pathIsClosed = true

	_, w.Err = fmt.Fprintln(w.Content, w.coord(x), w.coord(y), w.coord(width), w.coord(height), "re")
}

// Stroke strokes the current path.
//
// This implements the PDF graphics operator "S".
func (w *Writer) Stroke() {
	if !w.isValid("Stroke", objPath) {
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
func (w *Writer) CloseAndStroke() {
	if !w.isValid("CloseAndStroke", objPath) {
		return
	}
	w.currentObject = objPage

	_, w.Err = fmt.Fprintln(w.Content, "s")
}
