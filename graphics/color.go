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
	"seehuhn.de/go/pdf/color"
)

// SetFillColor sets the fill color in the graphics state.
// If col is nil, the fill color is not changed.
func (p *Page) SetFillColor(col color.Color) {
	if !p.valid("SetFillColor", stateGlobal, stateText) {
		return
	}
	if col != nil {
		p.err = col.SetFill(p.content)
	}
}

// SetStrokeColor sets the stroke color in the graphics state.
// If col is nil, the stroke color is not changed.
func (p *Page) SetStrokeColor(col color.Color) {
	if !p.valid("SetStrokeColor", stateGlobal, stateText) {
		return
	}
	if col != nil {
		p.err = col.SetStroke(p.content)
	}
}
