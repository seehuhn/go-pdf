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

// Package graphics allows to draw on a PDF page.
package graphics

import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/color"
	"seehuhn.de/go/pdf/font"
)

type graphicsState struct {
	object      objectType
	fillColor   color.Color
	strokeColor color.Color

	font     font.Embedded
	fontSize float64
}

// PushGraphicsState saves the current graphics state.
func (p *Page) PushGraphicsState() {
	// TODO(voss): does this require certain states?

	state := &graphicsState{
		object:      p.currentObject,
		fillColor:   p.fillColor,
		strokeColor: p.strokeColor,
		font:        p.font,
		fontSize:    p.fontSize,
	}
	p.stack = append(p.stack, state)

	_, err := fmt.Fprintln(p.Content, "q")
	if p.Err == nil {
		p.Err = err
	}
}

// PopGraphicsState restores the previous graphics state.
func (p *Page) PopGraphicsState() {
	// TODO(voss): does this require certain states?

	n := len(p.stack) - 1
	state := p.stack[n]
	p.stack = p.stack[:n]

	p.currentObject = state.object
	p.fillColor = state.fillColor
	p.strokeColor = state.strokeColor
	p.font = state.font
	p.fontSize = state.fontSize

	_, err := fmt.Fprintln(p.Content, "Q")
	if p.Err == nil {
		p.Err = err
	}
}

// SetExtGState sets selected graphics state parameters.
// The argument dictName must be the name of a graphics state dictionary
// that has been defined using the [Page.AddExtGState] method.
func (p *Page) SetExtGState(dictName pdf.Name) {
	if !p.valid("SetGraphicsState", objPage, objText) {
		return
	}

	err := dictName.PDF(p.Content)
	if err != nil {
		p.Err = err
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, " gs")
}

func (p *Page) AddExtGState(name pdf.Name, dict pdf.Dict) {
	if p.Resources == nil {
		p.Resources = &pdf.Resources{}
	}
	if p.Resources.ExtGState == nil {
		p.Resources.ExtGState = pdf.Dict{}
	}
	p.Resources.ExtGState[name] = dict
}

// Translate moves the origin of the coordinate system.
// Drawing the unit square [0, 1] x [0, 1] after this call is equivalent to
// drawing the rectangle [dx, dx+1] x [dy, dy+1] in the original
// coordinate system.
func (p *Page) Translate(dx, dy float64) {
	if !p.valid("Translate", objPage, objText) {
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, 1, 0, 0, 1,
		p.coord(dx), p.coord(dy), "cm")
}

// Scale scales the coordinate system.
// Drawing the unit square [0, 1] x [0, 1] after this call is equivalent to
// drawing the rectangle [0, xScale] x [0, yScale] in the original
// coordinate system.
func (p *Page) Scale(xScale, yScale float64) {
	if !p.valid("Scale", objPage, objText) {
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, p.coord(xScale), 0, 0, p.coord(yScale),
		0, 0, "cm")
}

// TranslateAndScale scales the coordinate system.
// Drawing the unit square [0, 1] x [0, 1] after this call is equivalent to
// drawing the rectangle [dx, dx+xScale] x [dy, dy+yScale] in the original
// coordinate system.
func (p *Page) TranslateAndScale(dx, dy, xScale, yScale float64) {
	if !p.valid("TranslateAndScale", objPage, objText) {
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, p.coord(xScale), 0, 0, p.coord(yScale),
		p.coord(dx), p.coord(dy), "cm")
}

// SetLineWidth sets the line width.
func (p *Page) SetLineWidth(width float64) {
	if !p.valid("SetLineWidth", objPage, objText) {
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, p.coord(width), "w")
}

// SetLineCap sets the line cap style.
func (p *Page) SetLineCap(cap LineCapStyle) {
	if !p.valid("SetLineCap", objPage, objText) {
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, int(cap), "J")
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
	if !p.valid("SetLineJoin", objPage, objText) {
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, int(join), "j")
}

// LineJoinStyle is the style of the corner of a line.
type LineJoinStyle uint8

// Possible values for LineJoinStyle.
const (
	LineJoinMiter LineJoinStyle = 0
	LineJoinRound LineJoinStyle = 1
	LineJoinBevel LineJoinStyle = 2
)

// SetFillColor sets the fill color in the graphics state.
// If col is nil, the fill color is not changed.
func (p *Page) SetFillColor(col color.Color) {
	if !p.valid("SetFillColor", objPage, objText) {
		return
	}
	if col == nil || col == p.fillColor {
		return
	}

	p.fillColor = col

	p.Err = col.SetFill(p.Content)
}

// SetStrokeColor sets the stroke color in the graphics state.
// If col is nil, the stroke color is not changed.
func (p *Page) SetStrokeColor(col color.Color) {
	if !p.valid("SetStrokeColor", objPage, objText) {
		return
	}
	if col == nil || col == p.strokeColor {
		return
	}

	p.strokeColor = col

	p.Err = col.SetStroke(p.Content)
}
