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

import "testing"

func TestByName(t *testing.T) {
	cases := []struct {
		name string
		want Font
	}{
		// the fourteen PostScript names
		{"Courier", Courier},
		{"Courier-Bold", CourierBold},
		{"Courier-BoldOblique", CourierBoldOblique},
		{"Courier-Oblique", CourierOblique},
		{"Helvetica", Helvetica},
		{"Helvetica-Bold", HelveticaBold},
		{"Helvetica-BoldOblique", HelveticaBoldOblique},
		{"Helvetica-Oblique", HelveticaOblique},
		{"Times-Roman", TimesRoman},
		{"Times-Bold", TimesBold},
		{"Times-BoldItalic", TimesBoldItalic},
		{"Times-Italic", TimesItalic},
		{"Symbol", Symbol},
		{"ZapfDingbats", ZapfDingbats},

		// short names from default-appearance strings
		{"Helv", Helvetica},
		{"HeBo", HelveticaBold},
		{"TiRo", TimesRoman},
		{"Cour", Courier},
		{"CourB", CourierBold},
		{"Symb", Symbol},
		{"ZaDb", ZapfDingbats},

		// names other producers use, hyphen and MT/PS/PSMT spellings
		{"Arial", Helvetica},
		{"ArialMT", Helvetica},
		{"Arial-Bold", HelveticaBold},
		{"Arial-BoldMT", HelveticaBold},
		{"Arial-Italic", HelveticaOblique},
		{"Arial-ItalicMT", HelveticaOblique},
		{"Arial-BoldItalic", HelveticaBoldOblique},
		{"Arial-BoldItalicMT", HelveticaBoldOblique},
		{"Helvetica-Italic", HelveticaOblique},
		{"Helvetica-BoldItalic", HelveticaBoldOblique},
		{"CourierNew", Courier},
		{"CourierNewPSMT", Courier},
		{"CourierNew-Bold", CourierBold},
		{"CourierNewPS-BoldMT", CourierBold},
		{"CourierNew-Italic", CourierOblique},
		{"CourierNewPS-ItalicMT", CourierOblique},
		{"CourierNew-BoldItalic", CourierBoldOblique},
		{"CourierNewPS-BoldItalicMT", CourierBoldOblique},
		{"TimesNewRoman", TimesRoman},
		{"TimesNewRomanPS", TimesRoman},
		{"TimesNewRomanPSMT", TimesRoman},
		{"TimesNewRoman-Bold", TimesBold},
		{"TimesNewRomanPS-Bold", TimesBold},
		{"TimesNewRomanPS-BoldMT", TimesBold},
		{"TimesNewRoman-Italic", TimesItalic},
		{"TimesNewRomanPS-Italic", TimesItalic},
		{"TimesNewRomanPS-ItalicMT", TimesItalic},
		{"TimesNewRoman-BoldItalic", TimesBoldItalic},
		{"TimesNewRomanPS-BoldItalic", TimesBoldItalic},
		{"TimesNewRomanPS-BoldItalicMT", TimesBoldItalic},

		// comma spellings
		{"Arial,Bold", HelveticaBold},
		{"Arial,Italic", HelveticaOblique},
		{"Arial,BoldItalic", HelveticaBoldOblique},
		{"Helvetica,Bold", HelveticaBold},
		{"Helvetica,Italic", HelveticaOblique},
		{"Helvetica,BoldItalic", HelveticaBoldOblique},
		{"Courier,Bold", CourierBold},
		{"Courier,Italic", CourierOblique},
		{"Courier,BoldItalic", CourierBoldOblique},
		{"CourierNew,Bold", CourierBold},
		{"CourierNew,Italic", CourierOblique},
		{"CourierNew,BoldItalic", CourierBoldOblique},
		{"TimesNewRoman,Bold", TimesBold},
		{"TimesNewRoman,Italic", TimesItalic},
		{"TimesNewRoman,BoldItalic", TimesBoldItalic},
		{"Symbol,Bold", Symbol},
		{"Symbol,Italic", Symbol},
		{"Symbol,BoldItalic", Symbol},
		{"SymbolMT", Symbol},
		{"SymbolMT,Bold", Symbol},
		{"SymbolMT,Italic", Symbol},
		{"SymbolMT,BoldItalic", Symbol},

		// spaces are ignored
		{"Times New Roman,Bold", TimesBold},
		{"Courier New", Courier},
		{"Zapf Dingbats", ZapfDingbats},
		{"Times-Roman ", TimesRoman},
		{"He lv", Helvetica},

		// case is significant
		{"helvetica", ""},
		{"HELV", ""},

		// no match
		{"", ""},
		{"Georgia", ""},
		{"MyriadPro-Regular", ""},
		{"Helvetica-Narrow", ""},
		{"Arial-Black", ""},
		{"ArialUnicodeMS", ""},
		{"HeBoX", ""},
		{"/Helvetica", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := ByName(c.name)
			if ok != (c.want != "") {
				t.Fatalf("ByName(%q) ok = %v, want %v", c.name, ok, c.want != "")
			}
			if got != c.want {
				t.Errorf("ByName(%q) = %q, want %q", c.name, got, c.want)
			}
		})
	}
}
