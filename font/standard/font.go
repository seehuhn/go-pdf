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
	"strings"

	"seehuhn.de/go/geom/rect"

	pstype1 "seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/pdf/font/loader"
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
func (f Font) New() *type1.Instance {
	inst, err := f.newInternal()
	if err != nil {
		panic(err)
	}
	return inst
}

func (f Font) newInternal() (*type1.Instance, error) {
	name := string(f)

	fontData, err := builtin.Open(name, loader.FontTypeType1)
	if err != nil {
		return nil, err // should not happen
	}
	psFont, err := pstype1.Read(fontData)
	if err != nil {
		return nil, err // should not happen
	}
	fontData.Close()

	afmData, err := builtin.Open(name, loader.FontTypeAFM)
	if err != nil {
		return nil, err // should not happen
	}
	metrics, err := afm.Read(afmData)
	if err != nil {
		return nil, err // should not happen
	}
	afmData.Close()

	// Fix up the fonts
	family := strings.SplitN(name, "-", 2)[0]

	psFont.FontName = name
	psFont.FamilyName = family
	psFont.Encoding = restrictGlyphList(f, psFont.Glyphs)
	metrics.FontName = name
	metrics.Encoding = restrictGlyphList(f, metrics.Glyphs)

	// Some of the fonts wrongly have a non-zero bounding box for some of the
	// whitespace glyphs.  We fix this here.
	//
	// Revisit this, once
	// https://github.com/ArtifexSoftware/urw-base35-fonts/issues/48
	// is resolved.
	for _, name := range []string{"space", "uni00A0", "uni2002"} {
		if g, ok := metrics.Glyphs[name]; ok {
			g.BBox = rect.Rect{}
		}
	}

	// Ascent and descent are missing from our .afm files.  We infer values for
	// these from glyph metrics.
	for _, name := range []string{"d", "bracketleft", "bar"} {
		if glyph, ok := metrics.Glyphs[name]; ok {
			y := glyph.BBox.URy
			if y > metrics.Ascent {
				metrics.Ascent = y
			}
		}
	}
	for _, name := range []string{"p", "bracketleft", "bar"} {
		if glyph, ok := metrics.Glyphs[name]; ok {
			y := glyph.BBox.LLy
			if y < metrics.Descent {
				metrics.Descent = y
			}
		}
	}

	// We add the standard ligatures here, just in case.
	if !metrics.IsFixedPitch {
		type lig struct {
			left, right, result string
		}
		var all = []lig{
			{"f", "f", "ff"},
			{"f", "i", "fi"},
			{"f", "l", "fl"},
			{"ff", "i", "ffi"},
			{"ff", "l", "ffl"},
		}
		for _, l := range all {
			_, leftOk := metrics.Glyphs[l.left]
			_, rightOk := metrics.Glyphs[l.right]
			_, resOk := metrics.Glyphs[l.result]
			if leftOk && rightOk && resOk {
				if len(metrics.Glyphs[l.left].Ligatures) == 0 {
					metrics.Glyphs[l.left].Ligatures = make(map[string]string)
				}
				metrics.Glyphs[l.left].Ligatures[l.right] = l.result
			}
		}
	}

	res, err := type1.New(psFont, metrics)
	if err != nil {
		return nil, err
	}

	switch psFont.FamilyName {
	case "Courier", "Times":
		res.IsSerif = true
	case "Helvetica":
		// pass
	case "Symbol", "ZapfDingbats":
		// pass
	default:
		panic("unreachable: " + family)
	}

	return res, nil
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

var builtin = loader.NewFontLoader()
