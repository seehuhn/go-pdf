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

import "fmt"

// PushGraphicsState saves the current graphics state.
func (p *Page) PushGraphicsState() {
	// TODO(voss): does this require certain states?
	p.stack = append(p.stack, p.state)
	_, err := fmt.Fprintln(p.content, "q")
	if p.err == nil {
		p.err = err
	}
}

// PopGraphicsState restores the previous graphics state.
func (p *Page) PopGraphicsState() {
	// TODO(voss): does this require certain states?
	n := len(p.stack) - 1
	p.state = p.stack[n]
	p.stack = p.stack[:n]
	_, err := fmt.Fprintln(p.content, "Q")
	if p.err == nil {
		p.err = err
	}
}

// Translate moves the origin of the coordinate system.
// Drawing the unit square [0, 1] x [0, 1] after this call is equivalent to
// drawing the rectangle [dx, dx+1] x [dy, dy+1] in the original
// coordinate system.
func (p *Page) Translate(dx, dy float64) {
	if !p.valid("Translate", stateGlobal, stateText) {
		return
	}
	_, p.err = fmt.Fprintln(p.content, 1, 0, 0, 1,
		p.coord(dx), p.coord(dy), "cm")
}

// Scale scales the coordinate system.
// Drawing the unit square [0, 1] x [0, 1] after this call is equivalent to
// drawing the rectangle [0, xScale] x [0, yScale] in the original
// coordinate system.
func (p *Page) Scale(xScale, yScale float64) {
	if !p.valid("Scale", stateGlobal, stateText) {
		return
	}
	_, p.err = fmt.Fprintln(p.content, p.coord(xScale), 0, 0, p.coord(yScale),
		0, 0, "cm")
}

// TranslateAndScale scales the coordinate system.
// Drawing the unit square [0, 1] x [0, 1] after this call is equivalent to
// drawing the rectangle [dx, dx+xScale] x [dy, dy+yScale] in the original
// coordinate system.
func (p *Page) TranslateAndScale(dx, dy, xScale, yScale float64) {
	if !p.valid("TranslateAndScale", stateGlobal, stateText) {
		return
	}
	_, p.err = fmt.Fprintln(p.content, p.coord(xScale), 0, 0, p.coord(yScale),
		p.coord(dx), p.coord(dy), "cm")
}

// SetLineWidth sets the line width.
func (p *Page) SetLineWidth(width float64) {
	if !p.valid("SetLineWidth", stateGlobal, stateText) {
		return
	}
	_, p.err = fmt.Fprintln(p.content, p.coord(width), "w")
}

// SetLineCap sets the line cap style.
func (p *Page) SetLineCap(cap LineCapStyle) {
	if !p.valid("SetLineCap", stateGlobal, stateText) {
		return
	}
	_, p.err = fmt.Fprintln(p.content, int(cap), "J")
}

// LineCapStyle is the style of the end of a line.
type LineCapStyle uint8

// Possible values for LineCapStyle.
// See section 8.4.3.3 of PDF 32000-1:2008.
const (
	LineCapButt   LineCapStyle = 0
	LineCapRound  LineCapStyle = 1
	LineCapSquare LineCapStyle = 2
)

// SetLineJoin sets the line join style.
func (p *Page) SetLineJoin(join LineJoinStyle) {
	if !p.valid("SetLineJoin", stateGlobal, stateText) {
		return
	}
	_, p.err = fmt.Fprintln(p.content, int(join), "j")
}

// LineJoinStyle is the style of the corner of a line.
type LineJoinStyle uint8

// Possible values for LineJoinStyle.
const (
	LineJoinMiter LineJoinStyle = 0
	LineJoinRound LineJoinStyle = 1
	LineJoinBevel LineJoinStyle = 2
)
