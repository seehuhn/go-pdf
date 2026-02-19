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

package fallback

import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/extended"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/extgstate"
	"seehuhn.de/go/pdf/graphics/form"
)

// The following fields are ignored when an annotation has an appearance
// stream.
//  - C: `Common.Color`
//  - Border: `Common.Border`
//  - IC: fill color for Circle, Line, Polygon, Polyline, Redact, Square
//  - BS: border style for Circle, FreeText, Ink, Line, Link, Polygon, Polyline, Square, Widget
//  - BE: border effects for Circle, FreeText, Polygon, Square
//  - H: horizontal shift for Watermark
//  - DA: default appearance for FreeText, Redact
//  - Q: alignment for FreeText, Redact
//  - DS: default style for FreeText
//  - LE: line ending style for FreeText, Line, Polyline
//  - LL: "leader lines" for Line
//  - LLE: "leader line extension" for Line
//  - MK: "appearance characteristics dictionary" for Screen, Widget
//  - Sy: "symbol" for Caret

type Style struct {
	// ContentFont is the font used to render the text content of annotations,
	// for example for FreeText annotations.
	ContentFont font.Layouter

	// iconFont is the font used to render symbols inside some of the icons for
	// text annotations.  If this is changed to be different from
	// extended.NimbusRomanBold, the layout of some text icons may need to be
	// adjusted.
	iconFont font.Layouter

	// reset is used to set a default graphics state at the beginning of each
	// appearance stream.
	reset *extgstate.ExtGState
}

func NewStyle() *Style {
	reset := &extgstate.ExtGState{
		Set: graphics.StateTextKnockout |
			graphics.StateLineCap |
			graphics.StateLineJoin |
			graphics.StateMiterLimit |
			graphics.StateLineDash |
			graphics.StateStrokeAdjustment,
		TextKnockout:     false,
		LineCap:          graphics.LineCapButt,
		LineJoin:         graphics.LineJoinMiter,
		MiterLimit:       10,
		DashPattern:      nil,
		DashPhase:        0,
		StrokeAdjustment: false,
	}

	// Allocate fonts once here, to make sure that at most one instance of each
	// font is embedded in an output file.
	return &Style{
		iconFont:    extended.NimbusRomanBold.New(),
		ContentFont: standard.Helvetica.New(),
		reset:       reset,
	}
}

func (s *Style) AddAppearance(a annotation.Annotation) error {
	// TODO(voss): cache appearances where possible

	var normal *form.Form
	switch a := a.(type) {
	case *annotation.Text:
		normal = s.addTextAppearance(a)
	case *annotation.Link:
		normal = s.addLinkAppearance(a)
	case *annotation.FreeText:
		normal = s.addFreeTextAppearance(a)
	case *annotation.Line:
		normal = s.addLineAppearance(a)
	case *annotation.Square:
		normal = s.addSquareAppearance(a)
	case *annotation.Circle:
		normal = s.addCircleAppearance(a)
	case *annotation.Polygon:
		normal = s.addPolygonAppearance(a)
	default:
		return fmt.Errorf("unsupported annotation type: %T", a)
	}

	c := a.GetCommon()
	if c.Appearance == nil {
		c.Appearance = &appearance.Dict{
			SingleUse: true,
		}
	}
	if c.AppearanceState == "" {
		c.Appearance.Normal = normal
		c.Appearance.NormalMap = nil
		c.Appearance.RollOver = nil
		c.Appearance.RollOverMap = nil
		c.Appearance.Down = nil
		c.Appearance.DownMap = nil
	} else {
		c.Appearance.Normal = nil
		if c.Appearance.NormalMap == nil {
			c.Appearance.NormalMap = make(map[pdf.Name]*form.Form)
		}
		c.Appearance.NormalMap[c.AppearanceState] = normal
		if c.Appearance.RollOver != nil {
			delete(c.Appearance.RollOverMap, c.AppearanceState)
		}
		if c.Appearance.Down != nil {
			delete(c.Appearance.DownMap, c.AppearanceState)
		}
	}

	return nil
}

// getBorderLineWidth returns the line width from BorderStyle, Border, or default
func getBorderLineWidth(border *annotation.Border, borderStyle *annotation.BorderStyle) float64 {
	if borderStyle != nil {
		return borderStyle.Width
	}
	if border != nil {
		return border.Width
	}
	return 1.0 // default line width
}

// getBorderDashPattern returns the dash pattern from BorderStyle or Border
func getBorderDashPattern(border *annotation.Border, borderStyle *annotation.BorderStyle) []float64 {
	if borderStyle != nil && len(borderStyle.DashArray) > 0 {
		return borderStyle.DashArray
	}
	if border != nil && len(border.DashArray) > 0 {
		return border.DashArray
	}
	return nil
}

// applyMargins adjusts a rectangle by applying margins (RD array)
func applyMargins(rect pdf.Rectangle, margin []float64) pdf.Rectangle {
	// apply margins (RD array) if specified
	if len(margin) == 4 {
		// RD format: [left, bottom, right, top]
		rect.LLx += margin[0] // left margin
		rect.LLy += margin[1] // bottom margin
		rect.URx -= margin[2] // right margin
		rect.URy -= margin[3] // top margin
	}
	return rect
}
