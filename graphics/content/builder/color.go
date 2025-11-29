// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package builder

import (
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
)

// SetStrokeColor sets the color to use for stroking operations.
func (b *Builder) SetStrokeColor(c color.Color) {
	b.setColor(c, false)
}

// SetFillColor sets the color to use for non-stroking operations.
func (b *Builder) SetFillColor(c color.Color) {
	b.setColor(c, true)
}

func (b *Builder) setColor(c color.Color, fill bool) {
	if b.Err != nil {
		return
	}

	var cur color.Color
	if fill {
		if b.isSet(graphics.StateFillColor) {
			cur = b.State.Param.FillColor
		}
	} else {
		if b.isSet(graphics.StateStrokeColor) {
			cur = b.State.Param.StrokeColor
		}
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
		name := b.getColorSpaceName(cs)
		if b.Err != nil {
			return
		}

		var op content.OpName = content.OpSetStrokeColorSpace
		if fill {
			op = content.OpSetFillColorSpace
		}
		b.emit(op, name)
		if b.Err != nil {
			return
		}
		cur = cs.Default()
	}

	if cur != c {
		var args []pdf.Object

		values, pattern, op := color.Operator(c)
		for _, val := range values {
			args = append(args, pdf.Number(val))
		}
		if pattern != nil {
			name := b.getPatternName(pattern)
			if b.Err != nil {
				return
			}
			args = append(args, name)
		}
		if fill {
			op = strings.ToLower(op)
		}
		b.emit(content.OpName(op), args...)
	}
}

// DrawShading paints the given shading, subject to the current clipping path.
// The current colour in the graphics state is neither used nor altered.
//
// This implements the PDF graphics operator "sh".
func (b *Builder) DrawShading(shading graphics.Shading) {
	if b.Err != nil {
		return
	}
	name := b.getShadingName(shading)
	b.emit(content.OpShading, name)
}
