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
	"math"
)

// This file contains convenience functions for circular arcs.

// Circle appends a circle to the current path, as a closed subpath.
func (p *Writer) Circle(x, y, radius float64) {
	if !p.isValid("Circle", objPage|objPath) {
		return
	}

	p.arc(x, y, radius, 0, 2*math.Pi, true)
	p.ClosePath()
}

// MoveToArc appends a circular arc to the current path,
// starting a new subpath.
func (p *Writer) MoveToArc(x, y, radius, startAngle, endAngle float64) {
	if !p.isValid("MoveToArc", objPage|objPath) {
		return
	}

	p.arc(x, y, radius, startAngle, endAngle, true)
}

// LineToArc appends a circular arc to the current subpath,
// connecting the previous point to the arc using a straight line.
func (p *Writer) LineToArc(x, y, radius, startAngle, endAngle float64) {
	if !p.isValid("LineToArc", objPath) {
		return
	}

	p.arc(x, y, radius, startAngle, endAngle, false)
}

// arc appends a circular arc to the current path.
func (p *Writer) arc(x, y, radius, startAngle, endAngle float64, move bool) {
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
		p.CurveTo(x1, y1, x2, y2, x3, y3)
		x0 = x3
		y0 = y3
	}
}