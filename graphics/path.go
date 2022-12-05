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
	if !p.valid("MoveTo", stateGlobal, statePath) {
		return
	}
	p.state = statePath
	_, p.err = fmt.Fprintln(p.w, p.coord(x), p.coord(y), "m")
}

// LineTo appends a straight line segment to the current path.
func (p *Page) LineTo(x, y float64) {
	if !p.valid("LineTo", statePath, stateClipped) {
		return
	}
	_, p.err = fmt.Fprintln(p.w, p.coord(x), p.coord(y), "l")
}

// Rectangle appends a rectangle to the current path as a closed subpath.
func (p *Page) Rectangle(x, y, width, height float64) {
	if !p.valid("Rectangle", stateGlobal, statePath) {
		return
	}
	p.state = statePath
	_, p.err = fmt.Fprintln(p.w, p.coord(x), p.coord(y), p.coord(width), p.coord(height), "re")
}

// MoveToArc appends a circular arc to the current path,
// starting a new subpath.
func (p *Page) MoveToArc(x, y, radius, startAngle, endAngle float64) {
	if !p.valid("MoveToArc", stateGlobal, statePath) {
		return
	}
	p.arc(x, y, radius, startAngle, endAngle, true)
}

// LineToArc appends a circular arc to the current subpath,
// connecting the arc to the previous point with a straight line.
func (p *Page) LineToArc(x, y, radius, startAngle, endAngle float64) {
	if !p.valid("LineToArc", statePath) {
		return
	}
	p.arc(x, y, radius, startAngle, endAngle, false)
}

// arc appends a circular to the current path.
func (p *Page) arc(x, y, radius, startAngle, endAngle float64, move bool) {
	p.state = statePath

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
		_, p.err = fmt.Fprintln(p.w, p.coord(x1), p.coord(y1), p.coord(x2), p.coord(y2), p.coord(x3), p.coord(y3), "c")
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
	if !p.valid("ClosePath", statePath) {
		return
	}
	_, p.err = fmt.Fprintln(p.w, "h")
}

// Stroke strokes the current path.
func (p *Page) Stroke() {
	if !p.valid("Stroke", statePath, stateClipped) {
		return
	}
	p.state = stateGlobal
	_, p.err = fmt.Fprintln(p.w, "S")
}

// CloseAndStroke closes and strokes the current path.
func (p *Page) CloseAndStroke() {
	if !p.valid("CloseAndStroke", statePath, stateClipped) {
		return
	}
	p.state = stateGlobal
	_, p.err = fmt.Fprintln(p.w, "s")
}

// Fill fills the current path.  Any subpaths that are open are implicitly
// closed before being filled.
func (p *Page) Fill() {
	if !p.valid("Fill", statePath, stateClipped) {
		return
	}
	p.state = stateGlobal
	_, p.err = fmt.Fprintln(p.w, "f")
}

// FillAndStroke fills and strokes the current path.  Any subpaths that are
// open are implicitly closed before being filled.
func (p *Page) FillAndStroke() {
	if !p.valid("FillAndStroke", statePath, stateClipped) {
		return
	}
	p.state = stateGlobal
	_, p.err = fmt.Fprintln(p.w, "B")
}
