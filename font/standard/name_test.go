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

// TestPlacesCoverEveryFont checks that every one of the fourteen fonts has a
// place in a family, since [Font.Face] and [Font.Style] answer from that
// table and a font missing from it would have no faces at all.
func TestPlacesCoverEveryFont(t *testing.T) {
	for _, f := range All {
		if _, ok := places[f]; !ok {
			t.Errorf("%s has no place", f)
		}
	}
}

// TestStyleAndRegularFace pins the style each font name spells out, and the
// regular face its family is named by.
func TestStyleAndRegularFace(t *testing.T) {
	cases := []struct {
		font    Font
		regular Font
		bold    bool
		italic  bool
	}{
		{Courier, Courier, false, false},
		{CourierBold, Courier, true, false},
		{CourierOblique, Courier, false, true},
		{CourierBoldOblique, Courier, true, true},
		{Helvetica, Helvetica, false, false},
		{HelveticaBold, Helvetica, true, false},
		{HelveticaOblique, Helvetica, false, true},
		{HelveticaBoldOblique, Helvetica, true, true},
		{TimesRoman, TimesRoman, false, false},
		{TimesBold, TimesRoman, true, false},
		{TimesItalic, TimesRoman, false, true},
		{TimesBoldItalic, TimesRoman, true, true},
		{Symbol, Symbol, false, false},
		{ZapfDingbats, ZapfDingbats, false, false},
	}
	for _, c := range cases {
		t.Run(string(c.font), func(t *testing.T) {
			if got := c.font.Face(false, false); got != c.regular {
				t.Errorf("regular face = %q, want %q", got, c.regular)
			}
			bold, italic := c.font.Style()
			if bold != c.bold || italic != c.italic {
				t.Errorf("style = (%v, %v), want (%v, %v)", bold, italic, c.bold, c.italic)
			}
		})
	}
}

// TestFaceRoundTrip checks the identity a caller combining two sources of
// style relies on: asking a font for the style it already has gives it back.
func TestFaceRoundTrip(t *testing.T) {
	for _, f := range All {
		if got := f.Face(f.Style()); got != f {
			t.Errorf("%s.Face(%s.Style()) = %q", f, f, got)
		}
	}
}

func TestFace(t *testing.T) {
	cases := []struct {
		font   Font
		bold   bool
		italic bool
		want   Font
	}{
		// a style added to the regular face, one taken away again, and one
		// exchanged for the other
		{Helvetica, true, true, HelveticaBoldOblique},
		{HelveticaBoldOblique, false, false, Helvetica},
		{HelveticaBold, false, true, HelveticaOblique},
		{HelveticaOblique, true, false, HelveticaBold},
		// Times names its italic face Italic where Helvetica says Oblique
		{TimesRoman, false, true, TimesItalic},
		{TimesItalic, true, true, TimesBoldItalic},
		{Courier, false, true, CourierOblique},
		// a family with a single face has nothing else to give
		{Symbol, true, true, Symbol},
		{ZapfDingbats, true, false, ZapfDingbats},
	}
	for _, c := range cases {
		t.Run(string(c.font), func(t *testing.T) {
			if got := c.font.Face(c.bold, c.italic); got != c.want {
				t.Errorf("Face(%v, %v) = %q, want %q", c.bold, c.italic, got, c.want)
			}
		})
	}
}

func TestFacePanicsOnUnknownFont(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("no panic for a font outside the fourteen")
		}
	}()
	Font("Arial").Face(false, false)
}
