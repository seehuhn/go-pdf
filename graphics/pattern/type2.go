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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/matrix"
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

// IsColored returns true.
// This implements the [seehuhn.de/go/pdf/graphics/color.Pattern] interface.
func (p *Type2) IsColored() bool {
	return true
}

// Embed returns the pattern dictionary for the shading pattern.
// This implements the [seehuhn.de/go/pdf/graphics/color.Pattern] interface.
func (p *Type2) Embed(rm *pdf.ResourceManager) (pdf.Res, error) {
	var zero pdf.Res

	sh, err := pdf.ResourceManagerEmbed(rm, p.Shading)
	if err != nil {
		return zero, err
	}

	dict := pdf.Dict{
		// "Type":        pdf.Name("Pattern"),
		"PatternType": pdf.Integer(2),
		"Shading":     sh.PDFObject(),
	}
	if p.Matrix != matrix.Identity && p.Matrix != matrix.Zero {
		dict["Matrix"] = toPDF(p.Matrix[:])
	}
	if p.ExtGState != nil {
		gs, err := pdf.ResourceManagerEmbed(rm, p.ExtGState)
		if err != nil {
			return zero, err
		}
		dict["ExtGState"] = gs.PDFObject()
	}

	var data pdf.Object = dict
	if p.SingleUse {
		ref := rm.Out.Alloc()
		err := rm.Out.Put(ref, dict)
		if err != nil {
			return zero, err
		}
		data = ref
	}

	return pdf.Res{
		Data: data,
	}, nil
}
