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
		if b.State.Out&graphics.StateFillColor != 0 {
			cur = b.State.Param.FillColor
		}
	} else {
		if b.State.Out&graphics.StateStrokeColor != 0 {
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

func (b *Builder) getColorSpaceName(cs color.Space) pdf.Name {
	key := resKey{"C", cs}
	if name, ok := b.resName[key]; ok {
		return name
	}
	if b.Resources.ColorSpace == nil {
		b.Resources.ColorSpace = make(map[pdf.Name]color.Space)
	}
	name := allocateName("C", b.Resources.ColorSpace)
	b.Resources.ColorSpace[name] = cs
	b.resName[key] = name
	return name
}

func (b *Builder) getPatternName(pat color.Pattern) pdf.Name {
	key := resKey{"P", pat}
	if name, ok := b.resName[key]; ok {
		return name
	}
	if b.Resources.Pattern == nil {
		b.Resources.Pattern = make(map[pdf.Name]color.Pattern)
	}
	name := allocateName("P", b.Resources.Pattern)
	b.Resources.Pattern[name] = pat
	b.resName[key] = name
	return name
}

// DrawShading paints the given shading, subject to the current clipping path.
// The current colour in the graphics state is neither used nor altered.
//
// This implements the PDF graphics operator "sh".
func (b *Builder) DrawShading(shading graphics.Shading) {
	if b.Err != nil {
		return
	}

	key := resKey{"S", shading}
	name, ok := b.resName[key]
	if !ok {
		if b.Resources.Shading == nil {
			b.Resources.Shading = make(map[pdf.Name]graphics.Shading)
		}
		name = allocateName("S", b.Resources.Shading)
		b.Resources.Shading[name] = shading
		b.resName[key] = name
	}

	b.emit(content.OpShading, name)
}
