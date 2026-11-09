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

// Package standard provides access to the 14 standard PDF fonts.
//
// The fonts are metrics-only: they are built from Adobe's Core14 AFM data and
// are never embedded in PDF files.  The outlines needed for rendering come
// from the font loader, see [seehuhn.de/go/pdf/font/loader].
package standard

import (
	"slices"
	"strings"

	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/pdf/font/internal/bundled"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/sfnt/os2"
)

// Font identifies the individual fonts.
type Font string

// Constants for the 14 standard PDF fonts.
const (
	Courier              Font = "Courier"
	CourierBold          Font = "Courier-Bold"
	CourierBoldOblique   Font = "Courier-BoldOblique"
	CourierOblique       Font = "Courier-Oblique"
	Helvetica            Font = "Helvetica"
	HelveticaBold        Font = "Helvetica-Bold"
	HelveticaBoldOblique Font = "Helvetica-BoldOblique"
	HelveticaOblique     Font = "Helvetica-Oblique"
	TimesRoman           Font = "Times-Roman"
	TimesBold            Font = "Times-Bold"
	TimesBoldItalic      Font = "Times-BoldItalic"
	TimesItalic          Font = "Times-Italic"
	Symbol               Font = "Symbol"
	ZapfDingbats         Font = "ZapfDingbats"
)

// String returns the PostScript name of the font.
func (f Font) String() string {
	return string(f)
}

// PostScriptName returns the PostScript name of the font, as used in the
// BaseFont entry of the font dictionary.
func (f Font) PostScriptName() string {
	return string(f)
}

// New returns a new font instance for the given standard font.
//
// The instance is built from Adobe's metrics for the font and has no font
// program: a PDF file which uses it relies on the viewer to supply the glyphs.
//
// The font data bundled with this package is immutable, so each of the 14
// fonts is read and parsed at most once per process and the instances handed
// out are clones sharing that data; see [type1.Instance.Clone].  An instance
// allocates character codes of its own and so belongs to a single document, but
// the data behind it does not: a caller must leave the shared data alone, since
// a change to it reaches every other instance of the same font.
//
// An error is returned if the font data bundled with this package cannot be
// loaded; this indicates a broken installation and should not happen for any
// of the predefined [Font] constants.  Callers that treat this as an
// invariant may wrap the call in [font.Must].
func (f Font) New() (*type1.Instance, error) {
	return shared.Get(f)
}

// shared holds the instance each standard font is cloned from, read on first
// use.
var shared = bundled.New(allStandardFonts, Font.read)

// read builds a font instance from the bundled metrics.  The instance has no
// font program: the viewer supplies the glyphs.
func (f Font) read() (*type1.Instance, error) {
	metrics, err := f.data()
	if err != nil {
		return nil, err
	}

	res, err := type1.New(nil, metrics)
	if err != nil {
		return nil, err
	}

	res.Descriptor.IsSerif = isSerif[f]

	// regular faces are 400, not the 500 implied by "Medium" in the metrics
	if strings.Contains(string(f), "Bold") {
		res.Descriptor.FontWeight = os2.WeightBold
	} else {
		res.Descriptor.FontWeight = os2.WeightNormal
	}

	return res, nil
}

// data reads the metrics from the bundled font data, restricted to the
// character set the specification guarantees.
func (f Font) data() (*afm.Metrics, error) {
	name := string(f)

	metrics, err := bundled.ReadAFM(name)
	if err != nil {
		return nil, err
	}

	metrics.FontName = name
	metrics.Encoding = f.restrictGlyphList(metrics.Glyphs)
	metrics.Kern = restrictKern(metrics.Kern, metrics.Glyphs)

	if g := metrics.Glyphs["Euro"]; g != nil && g.BBox.IsZero() {
		g.BBox = euroBBox[f]
	}

	nd := notdef[f]
	metrics.Glyphs[".notdef"] = &nd

	// no fi/fl ligatures: they would merge two fixed-pitch cells
	if metrics.IsFixedPitch {
		for _, g := range metrics.Glyphs {
			g.Ligatures = nil
		}
	}

	return metrics, nil
}

// euroBBox gives the Euro glyph bounding boxes for the fonts whose metrics
// list the Euro with an empty box.  Without a box the glyph would count as
// blank.  The boxes are those of the Euro in the corresponding Quire font,
// which is used to draw the glyph and has the same advance width, rounded
// outwards to integers.
var euroBBox = map[Font]rect.Rect{
	Courier:              {LLx: 44, LLy: -10, URx: 549, URy: 669},
	CourierBold:          {LLx: 34, LLy: -10, URx: 576, URy: 669},
	CourierBoldOblique:   {LLx: 26, LLy: -10, URx: 633, URy: 669},
	CourierOblique:       {LLx: 36, LLy: -10, URx: 609, URy: 669},
	Helvetica:            {LLx: 7, LLy: -10, URx: 542, URy: 699},
	HelveticaBold:        {LLx: 4, LLy: -10, URx: 539, URy: 699},
	HelveticaBoldOblique: {LLx: 13, LLy: -9, URx: 593, URy: 699},
	HelveticaOblique:     {LLx: 9, LLy: -9, URx: 600, URy: 699},
	TimesRoman:           {LLx: 15, LLy: -10, URx: 485, URy: 663},
	TimesBold:            {LLx: 12, LLy: -10, URx: 489, URy: 663},
	TimesBoldItalic:      {LLx: -6, LLy: -10, URx: 527, URy: 663},
	TimesItalic:          {LLx: 11, LLy: -10, URx: 540, URy: 663},
}

// notdef gives the metrics of the .notdef glyph, which Adobe's metrics omit.
// The values are those of the .notdef glyphs in the bundled Quire fonts,
// which stand in for the standard fonts, with the box rounded outwards.
var notdef = map[Font]afm.GlyphInfo{
	Courier:              {WidthX: 600, BBox: rect.Rect{LLx: 50, LLy: 0, URx: 551, URy: 659}},
	CourierBold:          {WidthX: 600, BBox: rect.Rect{LLx: 50, LLy: 0, URx: 551, URy: 659}},
	CourierBoldOblique:   {WidthX: 600, BBox: rect.Rect{LLx: 50, LLy: 0, URx: 551, URy: 659}},
	CourierOblique:       {WidthX: 600, BBox: rect.Rect{LLx: 50, LLy: 0, URx: 551, URy: 659}},
	Helvetica:            {WidthX: 600, BBox: rect.Rect{LLx: 50, LLy: 0, URx: 551, URy: 688}},
	HelveticaBold:        {WidthX: 600, BBox: rect.Rect{LLx: 50, LLy: 0, URx: 551, URy: 688}},
	HelveticaBoldOblique: {WidthX: 600, BBox: rect.Rect{LLx: 50, LLy: 0, URx: 551, URy: 688}},
	HelveticaOblique:     {WidthX: 600, BBox: rect.Rect{LLx: 50, LLy: 0, URx: 551, URy: 688}},
	TimesRoman:           {WidthX: 600, BBox: rect.Rect{LLx: 50, LLy: 0, URx: 551, URy: 655}},
	TimesBold:            {WidthX: 600, BBox: rect.Rect{LLx: 50, LLy: 0, URx: 551, URy: 655}},
	TimesBoldItalic:      {WidthX: 600, BBox: rect.Rect{LLx: 50, LLy: 0, URx: 551, URy: 655}},
	TimesItalic:          {WidthX: 600, BBox: rect.Rect{LLx: 50, LLy: 0, URx: 551, URy: 655}},
	Symbol:               {WidthX: 600, BBox: rect.Rect{LLx: 50, LLy: 0, URx: 550, URy: 700}},
	ZapfDingbats:         {WidthX: 600, BBox: rect.Rect{LLx: 50, LLy: 0, URx: 550, URy: 700}},
}

// isSerif records the fonts with serifs, for the descriptor flag of the same
// name.  The Courier designs have slab serifs.
var isSerif = map[Font]bool{
	Courier:            true,
	CourierBold:        true,
	CourierBoldOblique: true,
	CourierOblique:     true,
	TimesBold:          true,
	TimesBoldItalic:    true,
	TimesItalic:        true,
	TimesRoman:         true,
}

// Restrict the font to the character set guaranteed by the spec,
// and return the corresponding encoding.
func (f Font) restrictGlyphList[T any](glyphs map[string]T) []string {
	var charset map[string]bool
	var encoding []string
	switch f {
	case Symbol:
		charset = pdfenc.Symbol.Has
		encoding = pdfenc.Symbol.Encoding[:]
	case ZapfDingbats:
		charset = pdfenc.ZapfDingbats.Has
		encoding = pdfenc.ZapfDingbats.Encoding[:]
	default:
		charset = pdfenc.StandardLatin.Has
		encoding = pdfenc.Standard.Encoding[:]
	}
	for key := range glyphs {
		if !charset[key] && key != ".notdef" {
			delete(glyphs, key)
		}
	}
	return encoding
}

// restrictKern drops the kern pairs which name a glyph the restricted font no
// longer has, so that the metrics stay consistent with their own glyph list.
func restrictKern(kern []afm.KernPair, glyphs map[string]*afm.GlyphInfo) []afm.KernPair {
	return slices.DeleteFunc(kern, func(k afm.KernPair) bool {
		_, leftOK := glyphs[k.Left]
		_, rightOK := glyphs[k.Right]
		return !leftOK || !rightOK
	})
}

// All lists the 14 standard PDF fonts defined in this package.
var All = allStandardFonts

var allStandardFonts = []Font{
	Courier,
	CourierBold,
	CourierBoldOblique,
	CourierOblique,
	Helvetica,
	HelveticaBold,
	HelveticaBoldOblique,
	HelveticaOblique,
	TimesRoman,
	TimesBold,
	TimesBoldItalic,
	TimesItalic,
	Symbol,
	ZapfDingbats,
}
