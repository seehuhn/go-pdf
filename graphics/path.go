// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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
)

// MoveTo starts a new path at the given coordinates.
func (p *Page) MoveTo(x, y float64) {
	if !p.valid("MoveTo", objPage, objPath) {
		return
	}
	p.currentObject = objPath
	_, p.Err = fmt.Fprintln(p.Content, p.coord(x), p.coord(y), "m")
}

// LineTo appends a straight line segment to the current path.
func (p *Page) LineTo(x, y float64) {
	if !p.valid("LineTo", objPath, objClippingPath) {
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, p.coord(x), p.coord(y), "l")
}

// CurveTo appends a cubic Bezier curve to the current path.
func (p *Page) CurveTo(x1, y1, x2, y2, x3, y3 float64) {
	if !p.valid("CurveTo", objPath, objClippingPath) {
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, p.coord(x1), p.coord(y1), p.coord(x2), p.coord(y2), p.coord(x3), p.coord(y3), "c")
}

// Rectangle appends a rectangle to the current path as a closed subpath.
func (p *Page) Rectangle(x, y, width, height float64) {
	if !p.valid("Rectangle", objPage, objPath) {
		return
	}
	p.currentObject = objPath
	_, p.Err = fmt.Fprintln(p.Content, p.coord(x), p.coord(y), p.coord(width), p.coord(height), "re")
}

// MoveToArc appends a circular arc to the current path,
// starting a new subpath.
func (p *Page) MoveToArc(x, y, radius, startAngle, endAngle float64) {
	if !p.valid("MoveToArc", objPage, objPath) {
		return
	}
	p.arc(x, y, radius, startAngle, endAngle, true)
}

// LineToArc appends a circular arc to the current subpath,
// connecting the arc to the previous point with a straight line.
func (p *Page) LineToArc(x, y, radius, startAngle, endAngle float64) {
	if !p.valid("LineToArc", objPath) {
		return
	}
	p.arc(x, y, radius, startAngle, endAngle, false)
}

// arc appends a circular to the current path.
func (p *Page) arc(x, y, radius, startAngle, endAngle float64, move bool) {
	p.currentObject = objPath

	// also see https://www.tinaja.com/glib/bezcirc2.pdf
	// from https://pomax.github.io/bezierinfo/ , section 42

	nSegment := int(math.Ceil(math.Abs(endAngle-startAngle) / (0.5 * math.Pi)))
	dPhi := (endAngle - startAngle) / float64(nSegment)
	k := 4.0 / 3.0 * radius * math.Tan(dPhi/4)

	phi := startAngle
	x0 := x + radius*math.Cos(phi)
	y0 := y + radius*math.Sin(phi)
	if move {
		p.MoveTo(x0, y0)
	} else {
		p.LineTo(x0, y0)
	}

	for i := 0; i < nSegment; i++ {
		x1 := x0 - k*math.Sin(phi)
		y1 := y0 + k*math.Cos(phi)
		phi += dPhi
		x3 := x + radius*math.Cos(phi)
		y3 := y + radius*math.Sin(phi)
		x2 := x3 + k*math.Sin(phi)
		y2 := y3 - k*math.Cos(phi)
		_, p.Err = fmt.Fprintln(p.Content, p.coord(x1), p.coord(y1), p.coord(x2), p.coord(y2), p.coord(x3), p.coord(y3), "c")
		x0 = x3
		y0 = y3
	}
}

// Circle appends a circle to the current path, as a closed subpath.
func (p *Page) Circle(x, y, radius float64) {
	p.MoveToArc(x, y, radius, 0, 2*math.Pi)
	p.ClosePath()
}

// ClosePath closes the current subpath.
func (p *Page) ClosePath() {
	if !p.valid("ClosePath", objPath) {
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, "h")
}

// Stroke strokes the current path.
func (p *Page) Stroke() {
	if !p.valid("Stroke", objPath, objClippingPath) {
		return
	}
	p.currentObject = objPage
	_, p.Err = fmt.Fprintln(p.Content, "S")
}

// CloseAndStroke closes and strokes the current path.
func (p *Page) CloseAndStroke() {
	if !p.valid("CloseAndStroke", objPath, objClippingPath) {
		return
	}
	p.currentObject = objPage
	_, p.Err = fmt.Fprintln(p.Content, "s")
}

// Fill fills the current path, using the nonzero winding number rule.  Any
// subpaths that are open are implicitly closed before being filled.
func (p *Page) Fill() {
	if !p.valid("Fill", objPath, objClippingPath) {
		return
	}
	p.currentObject = objPage
	_, p.Err = fmt.Fprintln(p.Content, "f")
}

// Fill fills the current path, using the even-odd rule.  Any
// subpaths that are open are implicitly closed before being filled.
func (p *Page) FillEvenOdd() {
	if !p.valid("FillEvenOdd", objPath, objClippingPath) {
		return
	}
	p.currentObject = objPage
	_, p.Err = fmt.Fprintln(p.Content, "f*")
}

// FillAndStroke fills and strokes the current path.  Any subpaths that are
// open are implicitly closed before being filled.
func (p *Page) FillAndStroke() {
	if !p.valid("FillAndStroke", objPath, objClippingPath) {
		return
	}
	p.currentObject = objPage
	_, p.Err = fmt.Fprintln(p.Content, "B")
}

// EndPath ends the path without filling and stroking it.
// This is for use after the [Page.ClipNonZero] and [Page.ClipEvenOdd] methods.
func (p *Page) EndPath() {
	if !p.valid("EndPath", objPath, objClippingPath) {
		return
	}
	p.currentObject = objPage
	_, p.Err = fmt.Fprintln(p.Content, "n")
}

func (p *Page) ClipNonZero() {
	if !p.valid("ClipNonZero", objPath) {
		return
	}
	p.currentObject = objClippingPath
	_, p.Err = fmt.Fprintln(p.Content, "W")
}

func (p *Page) ClipEvenOdd() {
	if !p.valid("ClipEvenOdd", objPath) {
		return
	}
	p.currentObject = objClippingPath
	_, p.Err = fmt.Fprintln(p.Content, "W*")
}
