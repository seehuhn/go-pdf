// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package fallback

import (
	"math"
	"strings"
	"unicode"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/extgstate"
	"seehuhn.de/go/pdf/graphics/form"
)

func (s *Style) addStampAppearance(a *annotation.Stamp) *form.Form {
	col := a.Color
	if col == nil {
		col = color.DeviceRGB{0.75, 0, 0}
	}

	label := stampLabel(a.Icon)

	rect := a.Rect
	w := rect.Dx()
	h := rect.Dy()
	if w <= 0 || h <= 0 {
		return &form.Form{
			Content: nil,
			Res:     &content.Resources{},
			BBox:    rect,
		}
	}

	b := builder.New(content.Form, nil)

	b.SetExtGState(s.reset)
	if a.StrokingTransparency != 0 || a.NonStrokingTransparency != 0 {
		gs := &extgstate.ExtGState{
			Set:         graphics.StateStrokeAlpha | graphics.StateFillAlpha,
			StrokeAlpha: 1 - a.StrokingTransparency,
			FillAlpha:   1 - a.NonStrokingTransparency,
			SingleUse:   true,
		}
		b.SetExtGState(gs)
	}

	cx := (rect.LLx + rect.URx) / 2
	cy := (rect.LLy + rect.URy) / 2

	b.PushGraphicsState()

	// slight rotation around rect center
	angle := 6.0 * math.Pi / 180
	sinA := math.Sin(angle)
	m := matrix.Translate(-cx, -cy)
	m = m.Mul(matrix.Rotate(angle))
	m = m.Mul(matrix.Translate(cx, cy))
	b.Transform(m)

	lw := min(w, h) * 0.03
	if lw < 0.5 {
		lw = 0.5
	}
	b.SetLineWidth(lw)
	b.SetStrokeColor(col)
	b.SetFillColor(col)

	// shrink the drawing area so that the rotated content (including
	// stroke width) fits within the non-rotated BBox
	rotMarginX := h/2*sinA + lw/2
	rotMarginY := w/2*sinA + lw/2

	// cosmetic padding inside the rotation margin
	cosmetic := min(w, h) * 0.04
	padX := rotMarginX + cosmetic
	padY := rotMarginY + cosmetic

	// outer rounded rectangle
	r := min(w-2*padX, h-2*padY) * 0.12
	roundedRect(b, rect.LLx+padX, rect.LLy+padY, w-2*padX, h-2*padY, r)
	b.Stroke()

	// inner rounded rectangle (double-border stamp look)
	inset := lw * 2.5
	innerPadX := padX + inset
	innerPadY := padY + inset
	ri := max(0, r-inset)
	roundedRect(b, rect.LLx+innerPadX, rect.LLy+innerPadY, w-2*innerPadX, h-2*innerPadY, ri)
	b.Stroke()

	// measure and draw text
	textPadX := innerPadX + lw*2
	seq := s.ContentFont.Layout(nil, 1.0, label)
	textW := seq.TotalWidth()
	if textW > 0 {
		availW := w - 2*textPadX
		availH := h - 2*innerPadY - 2*lw
		fontSize := availW / textW
		geom := s.ContentFont.GetGeometry()
		textH := (geom.Ascent - geom.Descent) * fontSize
		if textH > availH*0.8 {
			fontSize = availH * 0.8 / (geom.Ascent - geom.Descent)
		}
		if fontSize < 1 {
			fontSize = 1
		}

		// center text, shifted slightly below geometric center
		drop := fontSize * 0.08
		b.TextBegin()
		b.TextSetFont(s.ContentFont, fontSize)
		b.TextSetHorizontalScaling(1)
		ty := cy - (geom.Ascent+geom.Descent)*fontSize/2 - drop
		b.TextFirstLine(rect.LLx+textPadX, ty)
		b.TextShowAligned(label, availW, 0.5)
		b.TextEnd()
	}

	b.PopGraphicsState()

	return &form.Form{
		Content: b.Stream,
		Res:     b.Resources,
		BBox:    rect,
	}
}

// stampLabel converts a StampIcon name to display text.
// It splits camelCase names and uppercases the result.
func stampLabel(icon annotation.StampIcon) string {
	name := string(icon)
	if name == "" {
		return "DRAFT"
	}

	var parts []string
	start := 0
	for i := 1; i < len(name); i++ {
		if unicode.IsUpper(rune(name[i])) {
			parts = append(parts, name[start:i])
			start = i
		}
	}
	parts = append(parts, name[start:])
	return strings.ToUpper(strings.Join(parts, " "))
}

// roundedRect appends a rounded rectangle subpath to the builder
func roundedRect(b *builder.Builder, x, y, w, h, r float64) {
	r = min(r, w/2, h/2)
	b.MoveTo(x+r, y)
	b.LineTo(x+w-r, y)
	b.LineToArc(x+w-r, y+r, r, -math.Pi/2, 0)
	b.LineTo(x+w, y+h-r)
	b.LineToArc(x+w-r, y+h-r, r, 0, math.Pi/2)
	b.LineTo(x+r, y+h)
	b.LineToArc(x+r, y+h-r, r, math.Pi/2, math.Pi)
	b.LineTo(x, y+r)
	b.LineToArc(x+r, y+r, r, math.Pi, 3*math.Pi/2)
	b.ClosePath()
}
