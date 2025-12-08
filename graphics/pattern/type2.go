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

package pattern

import (
	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

// Type2 represents the pattern dictionary for a shading pattern
// (pattern type 2).
//
// See section 8.7.4 (Type2 patterns) of ISO 32000-2:2020.
type Type2 struct {
	Shading   graphics.Shading
	Matrix    matrix.Matrix
	ExtGState *graphics.ExtGState

	SingleUse bool
}

var _ color.Pattern = (*Type2)(nil)

// PatternType returns 2 for shading patterns.
// This implements the [color.Pattern] interface.
func (p *Type2) PatternType() int {
	return 2
}

// PaintType returns 1 to indicate that shading patterns are colored.
// This implements the [color.Pattern] interface.
func (p *Type2) PaintType() int {
	return 1
}

// Embed returns the pattern dictionary for the shading pattern.
// This implements the [seehuhn.de/go/pdf/graphics/color.Pattern] interface.
func (p *Type2) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {

	sh, err := rm.Embed(p.Shading)
	if err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		// "Type":        pdf.Name("Pattern"),
		"PatternType": pdf.Integer(2),
		"Shading":     sh,
	}
	if p.Matrix != matrix.Identity && p.Matrix != matrix.Zero {
		dict["Matrix"] = toPDF(p.Matrix[:])
	}
	if p.ExtGState != nil {
		gs, err := rm.Embed(p.ExtGState)
		if err != nil {
			return nil, err
		}
		dict["ExtGState"] = gs
	}

	if p.SingleUse {
		return dict, nil
	}
	ref := rm.Alloc()
	err = rm.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}
	return ref, nil
}
