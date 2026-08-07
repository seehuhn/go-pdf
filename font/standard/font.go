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
package standard

import (
	"slices"
	"strings"

	"seehuhn.de/go/pdf/font/internal/bundled"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/postscript/afm"
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

func (f Font) String() string {
	return string(f)
}

func (f Font) PostScriptName() string {
	return string(f)
}

// New returns a new font instance for the given standard font.
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

// read builds a font instance from the bundled font data.
func (f Font) read() (*type1.Instance, error) {
	name := string(f)

	psFont, metrics, err := bundled.Read(name)
	if err != nil {
		return nil, err
	}

	// fix up the fonts
	family, _, _ := strings.Cut(name, "-")
	psFont.FontName = name
	psFont.FamilyName = family
	psFont.Encoding = restrictGlyphList(f, psFont.Glyphs)
	metrics.FontName = name
	metrics.Encoding = restrictGlyphList(f, metrics.Glyphs)
	metrics.Kern = restrictKern(metrics.Kern, metrics.Glyphs)

	bundled.FixUpMetrics(metrics)

	res, err := type1.New(psFont, metrics)
	if err != nil {
		return nil, err
	}

	res.IsSerif = isSerif[f]

	return res, nil
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
func restrictGlyphList[T any](f Font, glyphs map[string]T) []string {
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
