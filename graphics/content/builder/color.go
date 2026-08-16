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
	"fmt"
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
	if err := b.checkColorAllowed(); err != nil {
		b.Err = err
		return
	}

	var cur color.Color
	if fill {
		if b.isSet(graphics.StateFillColor) {
			cur = b.State.GState.FillColor
		}
	} else {
		if b.isSet(graphics.StateStrokeColor) {
			cur = b.State.GState.StrokeColor
		}
	}

	method := "SetStrokeColor"
	if fill {
		method = "SetFillColor"
	}

	cs := c.ColorSpace()
	values, pattern := color.Values(c)
	if err := checkComponentRanges(method, cs, values); err != nil {
		b.Err = err
		return
	}

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

		var op = content.OpSetStrokeColorSpace
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

		op := color.Operator(c)
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
	if err := b.checkColorAllowed(); err != nil {
		b.Err = err
		return
	}
	name := b.getShadingName(shading)
	b.emit(content.OpShading, name)
}

// checkComponentRanges reports an error if a colour has components outside the
// range its colour space allows.  The method name is used in the error message.
//
// The reader clips such components into range when it reads a content stream,
// as §8.4.1 requires for the numeric parameters of the graphics state.  Writing
// them would therefore produce a file describing a colour other than the one
// the caller asked for.  The builder tracks its own graphics state by replaying
// each emitted operator through that same reader, so without this check the
// tracked colour and the caller's would silently disagree.
func checkComponentRanges(method string, cs color.Space, values []float64) error {
	for i := range min(len(values), cs.Channels()) {
		lo, hi := cs.ComponentRange(i)
		// asking the reader's own clip whether it would change the value keeps
		// the two in step; a NaN is caught because it compares unequal to
		// everything, including itself
		if color.ClipComponent(values[i], lo, hi) != values[i] {
			return fmt.Errorf("%s: component %d = %g outside [%g, %g]",
				method, i, values[i], lo, hi)
		}
	}
	return nil
}
