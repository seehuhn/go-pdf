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

// SetFillGray sets the fill color to the given gray value.
// The value must be in the range from 0 (black) to 1 (white).
func (p *Page) SetFillGray(g float64) {
	if !p.valid("SetFillGray", stateGlobal, stateText) {
		return
	}
	_, p.err = fmt.Fprintf(p.w, "%g g\n", g)
}

// SetStrokeGray sets the stroke color to the given gray value.
// The value must be in the range from 0 (black) to 1 (white).
func (p *Page) SetStrokeGray(g float64) {
	if !p.valid("SetStrokeGray", stateGlobal, stateText) {
		return
	}
	_, p.err = fmt.Fprintf(p.w, "%g G\n", g)
}

// SetFillRGB sets the fill color to the given RGB values.
// Each component must be in the range [0, 1].
func (p *Page) SetFillRGB(r, g, b float64) {
	if !p.valid("SetFillRGB", stateGlobal, stateText) {
		return
	}
	_, p.err = fmt.Fprintf(p.w, "%g %g %g rg\n", r, g, b)
}

// SetStrokeRGB sets the stroke color to the given RGB values.
// Each component must be in the range [0, 1].
func (p *Page) SetStrokeRGB(r, g, b float64) {
	if !p.valid("SetStrokeRGB", stateGlobal, stateText) {
		return
	}
	_, p.err = fmt.Fprintf(p.w, "%g %g %g RG\n", r, g, b)
}
