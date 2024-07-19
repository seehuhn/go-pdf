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

// Package extended provides extended versions of the 14 standard PDF fonts.
package extended

import (
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/loader"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/funit"
	pstype1 "seehuhn.de/go/postscript/type1"
)

// Font identifies the individual fonts.
type Font int

// Constants for the extended fonts.
const (
	D050000L               Font = iota // extended version of ZapfDingbats
	NimbusMonoPSBold                   // extended version of Courier-Bold
	NimbusMonoPSBoldItalic             // extended version of Courier-BoldOblique
	NimbusMonoPSItalic                 // extended version of Courier-Oblique
	NimbusMonoPSRegular                // extended version of Courier
	NimbusRomanBold                    // extended version of Times-Bold
	NimbusRomanBoldItalic              // extended version of Times-BoldItalic
	NimbusRomanItalic                  // extended version of Times-Italic
	NimbusRomanRegular                 // extended version of Times-Roman
	NimbusSansBold                     // extended version of Helvetica-Bold
	NimbusSansBoldItalic               // extended version of Helvetica-BoldOblique
	NimbusSansItalic                   // extended version of Helvetica-Oblique
	NimbusSansRegular                  // extended version of Helvetica
	StandardSymbolsPS                  // extended version of Symbol
)

// New returns a new font instance for the given font and options.
func (f Font) New(opt *font.Options) (*type1.Instance, error) {
	name := fontName[f]

	fontData, err := builtin.Open(name, loader.FontTypeType1)
	if err != nil {
		panic("invalid extended font ID")
	}
	psFont, err := pstype1.Read(fontData)
	if err != nil {
		panic("built-in extended font corrupted???")
	}
	fontData.Close()

	afmData, err := builtin.Open(name, loader.FontTypeAFM)
	if err != nil {
		panic("built-in extended font metrics missing???")
	}
	metrics, err := afm.Read(afmData)
	if err != nil {
		panic("built-in extended font metrics corrupted???")
	}
	afmData.Close()

	// Some of the fonts wrongly have a non-zero bounding box for some of the
	// whitespace glyphs.  We fix this here.
	//
	// Revisit this, once
	// https://github.com/ArtifexSoftware/urw-base35-fonts/issues/48
	// is resolved.
	for _, name := range []string{"space", "uni00A0", "uni2002"} {
		if g, ok := metrics.Glyphs[name]; ok {
			g.BBox = funit.Rect16{}
		}
	}

	// Some metrics missing from our .afm files.  We infer values for
	// these from other metrics.
	for _, name := range []string{"d", "bracketleft", "bar"} {
		if glyph, ok := metrics.Glyphs[name]; ok {
			y := float64(glyph.BBox.URy)
			if y > metrics.Ascent {
				metrics.Ascent = y
			}
		}
	}
	for _, name := range []string{"p", "bracketleft", "bar"} {
		if glyph, ok := metrics.Glyphs[name]; ok {
			y := float64(glyph.BBox.LLy)
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

	return type1.New(psFont, metrics, opt)
}

var fontName = map[Font]string{
	D050000L:               "D050000L",
	NimbusMonoPSBold:       "NimbusMonoPS-Bold",
	NimbusMonoPSBoldItalic: "NimbusMonoPS-BoldItalic",
	NimbusMonoPSItalic:     "NimbusMonoPS-Italic",
	NimbusMonoPSRegular:    "NimbusMonoPS-Regular",
	NimbusRomanBold:        "NimbusRoman-Bold",
	NimbusRomanBoldItalic:  "NimbusRoman-BoldItalic",
	NimbusRomanItalic:      "NimbusRoman-Italic",
	NimbusRomanRegular:     "NimbusRoman-Regular",
	NimbusSansBold:         "NimbusSans-Bold",
	NimbusSansBoldItalic:   "NimbusSans-BoldItalic",
	NimbusSansItalic:       "NimbusSans-Italic",
	NimbusSansRegular:      "NimbusSans-Regular",
	StandardSymbolsPS:      "StandardSymbolsPS",
}

// All lists the 14 standard PDF fonts defined in this package.
var All = allExtendedFonts

var allExtendedFonts = []Font{
	D050000L,
	NimbusMonoPSBold,
	NimbusMonoPSBoldItalic,
	NimbusMonoPSItalic,
	NimbusMonoPSRegular,
	NimbusRomanBold,
	NimbusRomanBoldItalic,
	NimbusRomanItalic,
	NimbusRomanRegular,
	NimbusSansBold,
	NimbusSansBoldItalic,
	NimbusSansItalic,
	NimbusSansRegular,
	StandardSymbolsPS,
}

var builtin = loader.NewFontLoader()
