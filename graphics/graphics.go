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
	if !p.valid("PushGraphicsState", stateGlobal, stateText) {
		return
	}
	_, p.err = fmt.Fprintln(p.content, "q")
}

// PopGraphicsState restores the previous graphics state.
func (p *Page) PopGraphicsState() {
	if !p.valid("PopGraphicsState", stateGlobal, stateText) {
		return
	}
	_, p.err = fmt.Fprintln(p.content, "Q")
}

// Translate moves the origin of the coordinate system.
func (p *Page) Translate(x, y float64) {
	if !p.valid("Translate", stateGlobal, stateText) {
		return
	}
	_, p.err = fmt.Fprintln(p.content, 1, 0, 0, 1, p.coord(x), p.coord(y), "cm")
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
