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
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/extended"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content/builder"
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
//  - MK: "appearance characteristics dictionary" for Widget; for Screen its
//    Icon is used as the appearance-generation input by addScreenAppearance
//  - Sy: "symbol" for Caret

// Style controls the visual appearance of fallback annotation streams.
// The ContentFont field may be replaced after construction to use a
// different font for text content.
type Style struct {
	// ContentFont is the font used to render the text content of annotations,
	// for example for FreeText annotations.
	ContentFont font.Layouter

	// version is the PDF version targeted by appearance streams built via
	// this Style.  It is passed through to [builder.New] so that
	// version-restricted operators (e.g. `gs` on pre-1.2) are rejected at
	// build time.
	version pdf.Version

	// iconFont is the font used to render symbols inside some of the icons for
	// text annotations.  If this is changed to be different from
	// extended.NimbusRomanBold, the layout of some text icons may need to be
	// adjusted.
	iconFont font.Layouter

	// dingbatsFont is ZapfDingbats, used to draw the on-glyphs of check box and
	// radio button widgets (the marker named by the MK.CA characteristic).
	dingbatsFont font.Layouter

	// reset is used to set a default graphics state at the beginning of each
	// appearance stream.
	reset *extgstate.ExtGState
}

var _ annotation.AppearanceGenerator = (*Style)(nil)

// NewStyle returns a new Style with default fonts and graphics state.
// Appearance streams are built for the given PDF version, so that
// version-restricted operators are rejected at build time.
func NewStyle(version pdf.Version) *Style {
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
		version:      version,
		iconFont:     font.Must(extended.NimbusRomanBold.New()),
		dingbatsFont: font.Must(standard.ZapfDingbats.New()),
		ContentFont:  font.Must(standard.Helvetica.New()),
		reset:        reset,
	}
}

// ErrNoFallback is returned by [Style.AddAppearance] for an annotation type it
// cannot draw.  Callers which walk a document's annotations use it to tell a
// type they are content to skip from a fallback which was attempted and
// failed.
var ErrNoFallback = errors.New("no fallback appearance for this annotation type")

// AddAppearance generates a normal appearance stream for the annotation
// and sets it in the annotation's appearance dictionary. Any existing
// rollover and down appearances are cleared. The annotation's Rect and
// other fields may be adjusted to match the generated appearance.
//
// It returns [ErrNoFallback] if the annotation type has no fallback
// appearance.  Some types are not drawn as a matter of policy and some are
// simply not implemented yet; a caller decides which types it offers here,
// see [seehuhn.de/go/pdf/annotation.ShouldSynthesizeFallback].
func (s *Style) AddAppearance(a annotation.Annotation) error {
	// TODO(voss): cache appearances where possible

	var normal *form.Form
	var err error
	switch a := a.(type) {
	case *annotation.Text:
		normal, err = s.addTextAppearance(a)
	case *annotation.Link:
		normal, err = s.addLinkAppearance(a)
	case *annotation.FreeText:
		normal, err = s.addFreeTextAppearance(a)
	case *annotation.Line:
		normal, err = s.addLineAppearance(a)
	case *annotation.Square:
		normal, err = s.addSquareAppearance(a)
	case *annotation.Circle:
		normal, err = s.addCircleAppearance(a)
	case *annotation.Polygon:
		normal, err = s.addPolygonAppearance(a)
	case *annotation.PolyLine:
		normal, err = s.addPolyLineAppearance(a)
	case *annotation.Ink:
		normal, err = s.addInkAppearance(a)
	case *annotation.TextMarkup:
		normal, err = s.addTextMarkupAppearance(a)
	case *annotation.Caret:
		normal, err = s.addCaretAppearance(a)
	case *annotation.Stamp:
		normal, err = s.addStampAppearance(a)
	case *annotation.FileAttachment:
		normal, err = s.addFileAttachmentAppearance(a)
	case *annotation.Sound:
		normal, err = s.addSoundAppearance(a)
	case *annotation.Movie:
		normal, err = s.addMovieAppearance(a)
	case *annotation.Screen:
		normal, err = s.addScreenAppearance(a)
	case *annotation.Widget:
		// widgets build their own appearance dictionary (check boxes and radio
		// buttons need an on/off map, not a single normal stream)
		return s.addWidgetAppearance(a)
	default:
		return ErrNoFallback
	}
	if err != nil {
		return err
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
		if c.Appearance.RollOverMap != nil {
			delete(c.Appearance.RollOverMap, c.AppearanceState)
		}
		if c.Appearance.DownMap != nil {
			delete(c.Appearance.DownMap, c.AppearanceState)
		}
	}

	return nil
}

// harvest finalizes the builder into a form with the given bounding box.  It
// returns an error if the content stream cannot be built, for example because
// it uses operators unavailable in the target PDF version.
func harvest(b *builder.Builder, bbox pdf.Rectangle) (*form.Form, error) {
	ops, err := b.Harvest()
	if err != nil {
		return nil, err
	}
	return &form.Form{
		Content: ops,
		Res:     b.Resources,
		BBox:    bbox,
	}, nil
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
