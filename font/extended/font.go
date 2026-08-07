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
	"seehuhn.de/go/pdf/font/internal/bundled"
	"seehuhn.de/go/pdf/font/type1"
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

// New returns a new font instance for the given extended font.
//
// The font data bundled with this package is immutable, so each font is read
// and parsed at most once per process and the instances handed out are clones
// sharing that data; see [type1.Instance.Clone].  An instance allocates
// character codes of its own and so belongs to a single document, but the data
// behind it does not: a caller must leave the shared data alone, since a change
// to it reaches every other instance of the same font.
//
// An error is returned if the font data bundled with this package cannot be
// loaded; this indicates a broken installation and should not happen for any
// of the predefined [Font] constants.  Callers that treat this as an
// invariant may wrap the call in [font.Must].
func (f Font) New() (*type1.Instance, error) {
	return shared.Get(f)
}

// shared holds the instance each extended font is cloned from, read on first
// use.
var shared = bundled.New(allExtendedFonts, Font.read)

// read builds a font instance from the bundled font data.
func (f Font) read() (*type1.Instance, error) {
	psFont, metrics, err := bundled.Read(fontName[f])
	if err != nil {
		return nil, err
	}

	bundled.FixUpMetrics(metrics)

	res, err := type1.New(psFont, metrics)
	if err != nil {
		return nil, err
	}

	res.IsSerif = isSerif[f]

	return res, nil
}

// isSerif records the fonts with serifs, for the descriptor flag of the same
// name.  The Nimbus Mono PS designs follow Courier in having slab serifs.
var isSerif = map[Font]bool{
	NimbusMonoPSBold:       true,
	NimbusMonoPSBoldItalic: true,
	NimbusMonoPSItalic:     true,
	NimbusMonoPSRegular:    true,
	NimbusRomanBold:        true,
	NimbusRomanBoldItalic:  true,
	NimbusRomanItalic:      true,
	NimbusRomanRegular:     true,
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
