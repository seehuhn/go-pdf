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
	"maps"

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

// Style describes the look of the fallback appearance streams.  It carries no
// state tied to a particular document and can be used for any number of them;
// use [Style.New] to obtain a [Generator] for one document.
type Style struct {
	// NewContentFont chooses the font used to render the text content of
	// annotations, for example for FreeText annotations.  A nil value selects
	// the default, Helvetica.
	//
	// It is called once for each [Generator] and must return a new instance
	// every time, since an instance belongs to the one document its generator
	// serves, as described for [Generator].  A caller which instead wants to
	// name the generator's font elsewhere in its document, for example in a
	// default appearance string, can read [Generator.ContentFont] rather than
	// supplying an instance here.
	NewContentFont func() (font.Layouter, error)
}

// NewStyle returns a Style which uses the default fonts.  This is the zero
// Style; a caller which changes some of the fields can equally well start from
// a Style literal.
func NewStyle() *Style {
	return &Style{}
}

// Generator builds the fallback appearance streams for a single document.
//
// A Generator holds the font instances used in the streams it builds, so that
// the document needs only one copy of each.  These instances allocate character
// codes as text is laid out, which ties a Generator to one document and makes
// it unsafe for concurrent use.  A caller writing the document out must build
// every appearance stream before the fonts are written, since the codes are not
// settled until then.
//
// A caller which only draws the appearances, rather than writing them to a
// file, is under the same one-document restriction: the streams share the
// generator's fonts, so they cannot be mixed with those of another generator.
type Generator struct {
	// ContentFont is the font used to render the text content of annotations,
	// for example for FreeText annotations.  It comes from the Style's
	// NewContentFont.  A caller which needs to name the same font elsewhere in
	// its document, for example in a default appearance string, can read it
	// here rather than making a second instance.
	ContentFont font.Layouter

	// version is the PDF version targeted by the appearance streams.  It is
	// passed through to [builder.New] so that operators the file cannot use
	// are rejected at build time, and decides whether the reset state can be
	// carried in a graphics state dictionary.
	version pdf.Version

	// iconFont is the font used to render symbols inside some of the icons for
	// text annotations.  If this is changed to be different from
	// extended.NimbusRomanBold, the layout of some text icons may need to be
	// adjusted.  Use [Generator.icons] to read it.
	iconFont font.Layouter

	// dingbatsFont is ZapfDingbats, used to draw the on-glyphs of check box and
	// radio button widgets (the marker named by the MK.CA characteristic).
	// Use [Generator.dingbats] to read it.
	dingbatsFont font.Layouter

	// resetGS holds the reset parameters which have no operator of their own,
	// or nil for a file which cannot express them.  One dictionary is shared by
	// every appearance stream, so the file holds a single copy.
	resetGS *extgstate.ExtGState
}

// icons returns the font used for the symbols inside text annotation icons.
//
// Most documents have no annotation which needs it, so it is made on first use
// rather than with the Generator: parsing a font program is far more expensive
// than building the appearance streams that use it.
func (g *Generator) icons() font.Layouter {
	if g.iconFont == nil {
		g.iconFont = font.Must(extended.NimbusRomanBold.New())
	}
	return g.iconFont
}

// dingbats returns the font used for check box and radio button on-glyphs.
// Like [Generator.icons], it is made on first use.
func (g *Generator) dingbats() font.Layouter {
	if g.dingbatsFont == nil {
		g.dingbatsFont = font.Must(standard.ZapfDingbats.New())
	}
	return g.dingbatsFont
}

// reset establishes a known graphics state at the start of an appearance
// stream.  The drawing code below is written on the assumption that these
// values are in force, whatever state a viewer had set up when it invoked the
// stream.
//
// Line cap, join, miter limit and dash pattern have had operators of their own
// since PDF 1.0 and are set that way.  Text knockout has no operator and can
// be set only through a graphics state dictionary, which puts it out of reach
// before PDF 1.4; stroke adjustment travels with it, since its initial value
// is the one wanted here and it need only be restated where a dictionary is
// written anyway.
func (g *Generator) reset(b *builder.Builder) {
	b.SetLineCap(graphics.LineCapButt)
	b.SetLineJoin(graphics.LineJoinMiter)
	b.SetMiterLimit(10)
	b.SetLineDash(nil, 0)

	if g.resetGS != nil {
		b.SetExtGState(g.resetGS)
	}
}

var _ annotation.AppearanceGenerator = (*Generator)(nil)

// New returns a Generator for a PDF file of the given version.  Appearance
// streams are built for that version, so that operators the file cannot use
// are rejected at build time.
//
// An error is returned if the Style's content font cannot be made.
func (s *Style) New(version pdf.Version) (*Generator, error) {
	var contentFont font.Layouter
	var err error
	if s.NewContentFont == nil {
		contentFont, err = standard.Helvetica.New()
	} else {
		contentFont, err = s.NewContentFont()
	}
	if err != nil {
		return nil, err
	}

	g := &Generator{
		ContentFont: contentFont,
		version:     version,
	}
	if version >= pdf.V1_4 {
		g.resetGS = &extgstate.ExtGState{
			Set:              graphics.StateTextKnockout | graphics.StateStrokeAdjustment,
			TextKnockout:     false,
			StrokeAdjustment: false,
		}
	}
	return g, nil
}

// ErrNoFallback is returned by [Generator.AddAppearance] for an annotation
// type it cannot draw.  Callers which walk a document's annotations use it to
// tell a type they are content to skip from a fallback which was attempted and
// failed.
var ErrNoFallback = errors.New("no fallback appearance for this annotation type")

// AddAppearance generates a normal appearance stream for the annotation and
// gives the annotation an appearance dictionary holding it.  A rollover or down
// appearance of its own is content the caller supplied and is carried over,
// while one which repeated the normal appearance repeats the generated one.
// The annotation's Rect and other fields may be adjusted to match the
// generated appearance.
//
// The annotation's previous appearance dictionary is left unchanged: reading a
// file shares one dictionary between every annotation whose AP entry points at
// it, so the annotation receives a copy rather than its neighbours receiving
// the generated appearance.
//
// It returns [ErrNoFallback] if the annotation type has no fallback
// appearance.  Some types are not drawn as a matter of policy and some are
// simply not implemented yet; a caller decides which types it offers here,
// see [seehuhn.de/go/pdf/annotation.ShouldSynthesizeFallback].
func (g *Generator) AddAppearance(a annotation.Annotation) error {
	// TODO(voss): cache appearances where possible

	var normal *form.Form
	var err error
	switch a := a.(type) {
	case *annotation.Text:
		normal, err = g.addTextAppearance(a)
	case *annotation.Link:
		normal, err = g.addLinkAppearance(a)
	case *annotation.FreeText:
		normal, err = g.addFreeTextAppearance(a)
	case *annotation.Line:
		normal, err = g.addLineAppearance(a)
	case *annotation.Square:
		normal, err = g.addSquareAppearance(a)
	case *annotation.Circle:
		normal, err = g.addCircleAppearance(a)
	case *annotation.Polygon:
		normal, err = g.addPolygonAppearance(a)
	case *annotation.PolyLine:
		normal, err = g.addPolyLineAppearance(a)
	case *annotation.Ink:
		normal, err = g.addInkAppearance(a)
	case *annotation.TextMarkup:
		normal, err = g.addTextMarkupAppearance(a)
	case *annotation.Caret:
		normal, err = g.addCaretAppearance(a)
	case *annotation.Stamp:
		normal, err = g.addStampAppearance(a)
	case *annotation.FileAttachment:
		normal, err = g.addFileAttachmentAppearance(a)
	case *annotation.Sound:
		normal, err = g.addSoundAppearance(a)
	case *annotation.Movie:
		normal, err = g.addMovieAppearance(a)
	case *annotation.Screen:
		normal, err = g.addScreenAppearance(a)
	case *annotation.Widget:
		// widgets build their own appearance dictionary (check boxes and radio
		// buttons need an on/off map, not a single normal stream)
		return g.addWidgetAppearance(a)
	default:
		return ErrNoFallback
	}
	if err != nil {
		return err
	}

	c := a.GetCommon()
	c.Appearance = ownAppearance(c.Appearance)
	// only the normal appearance is replaced; a single stream and a per-state
	// map are alternatives for the same entry, so setting one clears the other
	if c.AppearanceState == "" {
		c.Appearance.SetNormal(normal, nil)
	} else {
		// SetNormal needs the current map intact to work out which entries
		// repeat it, so the new state goes into a copy
		byState := maps.Clone(c.Appearance.NormalMap)
		if byState == nil {
			byState = make(map[pdf.Name]*form.Form)
		}
		byState[c.AppearanceState] = normal
		c.Appearance.SetNormal(nil, byState)
	}
	syncAppearanceState(c)

	return nil
}

// ownAppearance returns an appearance dictionary the caller may change, given
// the one an annotation currently holds.  A dictionary read from a file is
// shared with every other annotation whose AP entry points at it, so this
// package works on a copy rather than writing through to its neighbours: it
// never modifies a dictionary it did not create itself.
func ownAppearance(d *appearance.Dict) *appearance.Dict {
	if d == nil {
		return &appearance.Dict{SingleUse: true}
	}
	return d.Clone()
}

// syncAppearanceState brings the annotation's appearance state in line with
// its appearance dictionary, after a generated normal appearance replaced the
// previous one.
//
// The rollover and down appearances survive the replacement, so a dictionary
// can still select by state after a stateless normal appearance went in, and
// then needs a state to select with.  One which no longer selects by state has
// no use for the entry.
func syncAppearanceState(c *annotation.Common) {
	if state := c.Appearance.AnyState(); state == "" {
		c.AppearanceState = ""
	} else if c.AppearanceState == "" {
		c.AppearanceState = state
	}
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
