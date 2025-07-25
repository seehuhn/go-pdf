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

package graphics

import (
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
)

// This file implements functions to set the stroke and fill colors.
// The operators used here are defined in table 73 of ISO 32000-2:2020.

// SetStrokeColor sets the color to use for stroking operations.
func (w *Writer) SetStrokeColor(c color.Color) {
	if !w.isValid("SetStrokeColor", objPage|objText) {
		return
	}
	w.setColor(c, false)
}

// SetFillColor sets the color to use for non-stroking operations.
func (w *Writer) SetFillColor(c color.Color) {
	if !w.isValid("SetFillColor", objPage|objText) {
		return
	}
	w.setColor(c, true)
}

func (w *Writer) setColor(c color.Color, fill bool) {
	var cur color.Color
	if fill {
		if w.isSet(StateFillColor) {
			cur = w.FillColor
		}
		w.FillColor = c
		w.Set |= StateFillColor
	} else {
		if w.isSet(StateStrokeColor) {
			cur = w.StrokeColor
		}
		w.StrokeColor = c
		w.Set |= StateStrokeColor
	}

	cs := c.ColorSpace()
	var needsColorSpace bool
	switch cs.Family() {
	case color.FamilyDeviceGray, color.FamilyDeviceRGB, color.FamilyDeviceCMYK:
		needsColorSpace = false
	default:
		needsColorSpace = cur == nil || cur.ColorSpace() != cs
	}

	if needsColorSpace {
		name, _, err := writerGetResourceName(w, catColorSpace, cs)
		if err != nil {
			w.Err = err
			return
		}

		var op pdf.Operator = "CS"
		if fill {
			op = "cs"
		}
		w.writeObjects(name, op)
		if w.Err != nil {
			return
		}
		cur = cs.Default()
	}

	if cur != c {
		var out []pdf.Object

		values, pattern, op := color.Operator(c)
		for _, val := range values {
			out = append(out, pdf.Number(val))
		}
		if pattern != nil {
			name, _, err := writerGetResourceName(w, catPattern, pattern)
			if err != nil {
				w.Err = err
				return
			}
			out = append(out, name)
		}
		if fill {
			op = strings.ToLower(op)
		}
		out = append(out, pdf.Operator(op))
		w.writeObjects(out...)
	}
}

// DrawShading paints the given shading, subject to the current clipping path.
// The current colour in the graphics state is neither used nor altered.
//
// All coordinates in the shading dictionary are interpreted relative to the
// current user space. The "Background" entry in the shading pattern (if any)
// is ignored.
//
// This implements the PDF graphics operator "sh".
func (w *Writer) DrawShading(shading Shading) {
	if !w.isValid("DrawShading", objPage) {
		return
	}
	if pdf.GetVersion(w.RM.Out) < pdf.V1_3 {
		w.Err = &pdf.VersionError{
			Operation: "shading objects",
			Earliest:  pdf.V1_3,
		}
		return
	}

	name, _, err := writerGetResourceName(w, catShading, shading)
	if err != nil {
		w.Err = err
		return
	}

	w.writeObjects(name, pdf.Operator("sh"))
}
