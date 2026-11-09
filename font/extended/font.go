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

// Package extended provides the Quire fonts, extended versions of the 14
// standard PDF fonts.
//
// The Quire fonts are derived from the Liberation fonts (version 2.1.5) and the
// Foxit Symbol and Dingbats fonts.  They are renamed subsets, available under
// the licences listed in font/loader/builtin/Licenses.txt.  Quire Sans, Quire
// Serif and Quire Mono stand in for Helvetica, Times and Courier; Quire Mono
// has no serifs, unlike Courier, and its font descriptor says so.  Quire Mono
// has the glyph widths of Courier.  Quire Sans and Quire Serif have the glyph
// widths of Helvetica and Times, except for ¯, ±, ÷, ď, ľ, ť and ∑ and, in
// some faces, µ, ·, ∂, √, ◊, ī and Ľ.  Quire Symbol and Quire Dingbats stand
// in for Symbol and ZapfDingbats.  The fonts are TrueType (Latin faces) or
// OpenType/CFF (Symbol and Dingbats) fonts, and are embedded into PDF files as
// subsets.
//
// # Coverage
//
// The twelve Latin faces cover Basic Latin, Latin-1, Latin Extended-A, the
// Romanian letters with comma below, the Welsh letters ŵ and ŷ, ƒ, the
// spacing accents, Greek, Cyrillic, General Punctuation, currency symbols,
// letterlike symbols, number forms, arrows, mathematical operators, box
// drawing characters and the fi and fl ligature glyphs (U+FB01, U+FB02).
// Quire Symbol and Quire Dingbats cover the glyphs of Symbol and ZapfDingbats.
//
// # Layout Features
//
// All fonts carry hinting.  The layout features differ between the families.
//
//   - Quire Sans: GSUB "liga", "subs" and "sups"; GPOS "kern", "mark" and
//     "mkmk".
//   - Quire Serif: GSUB "liga", "subs" and "sups"; GPOS "kern" and "mark".
//   - Quire Mono: GSUB "subs" and "sups"; GPOS "mark" (Bold, Italic and
//     BoldItalic only; Quire Mono Regular has no GPOS lookups).
//   - The italic faces (Italic and BoldItalic) of Quire Sans and Quire Serif
//     additionally have a GSUB "locl" feature.
//   - Quire Symbol and Quire Dingbats have no layout tables.
//
// The "liga" feature forms the fi and fl ligatures, as the metrics of
// Helvetica and Times-Roman do.  Quire Mono has no ligatures, like Courier.
// The "ccmp" and "dlig" features are declared, but contain no lookups.
//
// If [embed.Options.GsubFeatures] is nil, the default set of the layout engine
// is used: of the GSUB features above only "liga" and "locl" are on by
// default, so subscripts and superscripts must be requested explicitly via
// GsubFeatures.  If [embed.Options.GposFeatures] is nil, "kern", "mark" and
// "mkmk" are on by default.
//
// # Embedding
//
// The fonts are static, so [embed.Options.Variations] must be empty.
// A nil [embed.Options.CMap] gives a simple font; otherwise a composite font
// is written, using that CMap.
package extended

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/embed"
	"seehuhn.de/go/pdf/font/internal/bundled"
	"seehuhn.de/go/pdf/font/standard"
)

// Font identifies the individual fonts.
type Font int

// Constants for the extended fonts.
const (
	QuireSansRegular     Font = iota // extended version of Helvetica
	QuireSansBold                    // extended version of Helvetica-Bold
	QuireSansItalic                  // extended version of Helvetica-Oblique
	QuireSansBoldItalic              // extended version of Helvetica-BoldOblique
	QuireSerifRegular                // extended version of Times-Roman
	QuireSerifBold                   // extended version of Times-Bold
	QuireSerifItalic                 // extended version of Times-Italic
	QuireSerifBoldItalic             // extended version of Times-BoldItalic
	QuireMonoRegular                 // extended version of Courier
	QuireMonoBold                    // extended version of Courier-Bold
	QuireMonoItalic                  // extended version of Courier-Oblique
	QuireMonoBoldItalic              // extended version of Courier-BoldOblique
	QuireSymbol                      // extended version of Symbol
	QuireDingbats                    // extended version of ZapfDingbats
)

// All lists the fonts defined in this package.
var All = []Font{
	QuireSansRegular,
	QuireSansBold,
	QuireSansItalic,
	QuireSansBoldItalic,
	QuireSerifRegular,
	QuireSerifBold,
	QuireSerifItalic,
	QuireSerifBoldItalic,
	QuireMonoRegular,
	QuireMonoBold,
	QuireMonoItalic,
	QuireMonoBoldItalic,
	QuireSymbol,
	QuireDingbats,
}

var postScriptNames = [...]string{
	QuireSansRegular:     "QuireSans-Regular",
	QuireSansBold:        "QuireSans-Bold",
	QuireSansItalic:      "QuireSans-Italic",
	QuireSansBoldItalic:  "QuireSans-BoldItalic",
	QuireSerifRegular:    "QuireSerif-Regular",
	QuireSerifBold:       "QuireSerif-Bold",
	QuireSerifItalic:     "QuireSerif-Italic",
	QuireSerifBoldItalic: "QuireSerif-BoldItalic",
	QuireMonoRegular:     "QuireMono-Regular",
	QuireMonoBold:        "QuireMono-Bold",
	QuireMonoItalic:      "QuireMono-Italic",
	QuireMonoBoldItalic:  "QuireMono-BoldItalic",
	QuireSymbol:          "QuireSymbol",
	QuireDingbats:        "QuireDingbats",
}

var standardFonts = [...]standard.Font{
	QuireSansRegular:     standard.Helvetica,
	QuireSansBold:        standard.HelveticaBold,
	QuireSansItalic:      standard.HelveticaOblique,
	QuireSansBoldItalic:  standard.HelveticaBoldOblique,
	QuireSerifRegular:    standard.TimesRoman,
	QuireSerifBold:       standard.TimesBold,
	QuireSerifItalic:     standard.TimesItalic,
	QuireSerifBoldItalic: standard.TimesBoldItalic,
	QuireMonoRegular:     standard.Courier,
	QuireMonoBold:        standard.CourierBold,
	QuireMonoItalic:      standard.CourierOblique,
	QuireMonoBoldItalic:  standard.CourierBoldOblique,
	QuireSymbol:          standard.Symbol,
	QuireDingbats:        standard.ZapfDingbats,
}

func (f Font) valid() bool {
	return f >= 0 && int(f) < len(postScriptNames)
}

// String returns the PostScript name of the font.
func (f Font) String() string {
	return f.PostScriptName()
}

// GoString returns the Go expression which names the font, for example
// "extended.QuireSansRegular".  This implements [fmt.GoStringer], so that the
// %#v verb prints the name of the constant.
func (f Font) GoString() string {
	if !f.valid() {
		return fmt.Sprintf("extended.Font(%d)", int(f))
	}
	return "extended." + strings.ReplaceAll(postScriptNames[f], "-", "")
}

// PostScriptName returns the PostScript name of the font, for example
// "QuireSans-Regular".
func (f Font) PostScriptName() string {
	if !f.valid() {
		return ""
	}
	return postScriptNames[f]
}

// Standard returns the standard PDF font which f extends.
// It returns the empty string if f is not one of the constants of this package.
func (f Font) Standard() standard.Font {
	if !f.valid() {
		return ""
	}
	return standardFonts[f]
}

// New returns a font instance for f, embedded as described by opt.
// If opt is nil, a simple font with the default layout features is returned.
//
// The bundled font file is parsed at most once per process.  The instance
// belongs to a single document.
//
// An error is returned if f is not one of the constants of this package, if
// opt.Variations is not empty (the fonts are static), or if the font data
// bundled with this package cannot be loaded.  Callers that treat the last
// case as an invariant may wrap the call in [font.Must].
func (f Font) New(opt *embed.Options) (font.Layouter, error) {
	if !f.valid() {
		return nil, errors.New("extended: unknown font")
	}
	info, err := parsed[f]()
	if err != nil {
		return nil, err
	}
	// embedding leaves the parsed font unchanged, so it can be shared
	return embed.OpenTypeFont(info, opt)
}

// parsed holds the parsed font files, read on first use.
var parsed = func() []func() (*sfnt.Font, error) {
	res := make([]func() (*sfnt.Font, error), len(postScriptNames))
	for i, name := range postScriptNames {
		res[i] = sync.OnceValues(func() (*sfnt.Font, error) {
			return bundled.ReadSfnt(name)
		})
	}
	return res
}()
