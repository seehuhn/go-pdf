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

	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/extended"
	"seehuhn.de/go/pdf/graphics/color"
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
	iconFont font.Layouter
	cache    map[key]*appearance.Dict
}

type key struct {
	name string
	bg   color.Color
}

func NewStyle() *Style {
	iconFont := extended.NimbusRomanBold.New()
	return &Style{
		iconFont: iconFont,
		cache:    make(map[key]*appearance.Dict),
	}
}

func (s *Style) AddAppearance(a annotation.Annotation) error {
	switch a := a.(type) {
	case *annotation.Text:
		s.addTextAppearance(a)
	case *annotation.FreeText:
		s.addFreeTextAppearance(a)
	default:
		return fmt.Errorf("unsupported annotation type: %T", a)
	}
	return nil
}
