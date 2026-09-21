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

package standard

import (
	"slices"
	"strings"
)

// ByName returns the standard font a font name refers to.
//
// The name may be one of the fourteen PostScript names, one of the short
// names conventionally used in default-appearance strings (Helv, HeBo, TiRo,
// Cour, CourB, Symb, ZaDb), or one of the names other producers use for the
// same faces: Arial, TimesNewRoman and CourierNew stand for Helvetica, Times
// and Courier, an MT, PS or PSMT suffix on the family is ignored, and a style
// may follow the family after a comma or a hyphen, as in "Arial,Bold" or
// "TimesNewRomanPS-BoldItalicMT".  Spaces are ignored throughout; case is
// significant.
//
// The second return value is false if the name matches no standard font.
func ByName(name string) (Font, bool) {
	name = strings.ReplaceAll(name, " ", "")
	if f := Font(name); slices.Contains(allStandardFonts, f) {
		return f, true
	}
	if f, ok := shortNames[name]; ok {
		return f, true
	}

	family, style, ok := strings.Cut(name, ",")
	if !ok {
		family, style, _ = strings.Cut(name, "-")
	}
	for _, suffix := range []string{"PSMT", "MT", "PS"} {
		if s, ok := strings.CutSuffix(family, suffix); ok {
			family = s
			break
		}
	}
	style = strings.TrimSuffix(style, "MT")

	faces, ok := families[family]
	if !ok {
		return "", false
	}
	f, ok := faces[style]
	return f, ok
}

// shortNames are the abbreviations conventionally used to name the standard
// fonts in default-appearance strings.
var shortNames = map[string]Font{
	"Helv":  Helvetica,
	"HeBo":  HelveticaBold,
	"TiRo":  TimesRoman,
	"Cour":  Courier,
	"CourB": CourierBold,
	"Symb":  Symbol,
	"ZaDb":  ZapfDingbats,
}

// families maps a family name to its faces by style suffix.
var families = map[string]map[string]Font{
	"Helvetica":     helveticaFaces,
	"Arial":         helveticaFaces,
	"Times":         timesFaces,
	"TimesNewRoman": timesFaces,
	"Courier":       courierFaces,
	"CourierNew":    courierFaces,
	"Symbol":        singleFace(Symbol),
	"ZapfDingbats":  singleFace(ZapfDingbats),
}

var helveticaFaces = map[string]Font{
	"":            Helvetica,
	"Bold":        HelveticaBold,
	"Italic":      HelveticaOblique,
	"Oblique":     HelveticaOblique,
	"BoldItalic":  HelveticaBoldOblique,
	"BoldOblique": HelveticaBoldOblique,
}

var timesFaces = map[string]Font{
	"":            TimesRoman,
	"Roman":       TimesRoman,
	"Bold":        TimesBold,
	"Italic":      TimesItalic,
	"Oblique":     TimesItalic,
	"BoldItalic":  TimesBoldItalic,
	"BoldOblique": TimesBoldItalic,
}

var courierFaces = map[string]Font{
	"":            Courier,
	"Bold":        CourierBold,
	"Italic":      CourierOblique,
	"Oblique":     CourierOblique,
	"BoldItalic":  CourierBoldOblique,
	"BoldOblique": CourierBoldOblique,
}

// singleFace returns the face table of a family whose only face is f,
// whatever style the name claims.
func singleFace(f Font) map[string]Font {
	return map[string]Font{
		"":            f,
		"Bold":        f,
		"Italic":      f,
		"Oblique":     f,
		"BoldItalic":  f,
		"BoldOblique": f,
	}
}
