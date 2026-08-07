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

package extended

import (
	"testing"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
)

// TestNewAll checks that every font this package names can be made.
func TestNewAll(t *testing.T) {
	for _, f := range All {
		if _, err := f.New(); err != nil {
			t.Errorf("%s: %v", fontName[f], err)
		}
	}
}

// Each font here is an extended version of one of the 14 standard fonts, so
// the two describe the same design and must agree about the serif flag their
// font descriptors carry.
func TestSerifMatchesStandard(t *testing.T) {
	equivalent := map[Font]standard.Font{
		D050000L:               standard.ZapfDingbats,
		NimbusMonoPSBold:       standard.CourierBold,
		NimbusMonoPSBoldItalic: standard.CourierBoldOblique,
		NimbusMonoPSItalic:     standard.CourierOblique,
		NimbusMonoPSRegular:    standard.Courier,
		NimbusRomanBold:        standard.TimesBold,
		NimbusRomanBoldItalic:  standard.TimesBoldItalic,
		NimbusRomanItalic:      standard.TimesItalic,
		NimbusRomanRegular:     standard.TimesRoman,
		NimbusSansBold:         standard.HelveticaBold,
		NimbusSansBoldItalic:   standard.HelveticaBoldOblique,
		NimbusSansItalic:       standard.HelveticaOblique,
		NimbusSansRegular:      standard.Helvetica,
		StandardSymbolsPS:      standard.Symbol,
	}
	if len(equivalent) != len(All) {
		t.Fatalf("%d fonts have a standard equivalent, want %d", len(equivalent), len(All))
	}

	for _, f := range All {
		ext := font.Must(f.New())
		std := font.Must(equivalent[f].New())
		if ext.IsSerif != std.IsSerif {
			t.Errorf("%s has IsSerif=%v, but %s has %v",
				fontName[f], ext.IsSerif, equivalent[f], std.IsSerif)
		}
	}
}

// TestNewIndependentEncoders checks that two instances of the same extended
// font allocate character codes independently.  They share the font data, so
// this is what keeps each usable in a document of its own.
func TestNewIndependentEncoders(t *testing.T) {
	first := font.Must(NimbusRomanRegular.New())
	second := font.Must(NimbusRomanRegular.New())

	if first.Simple == second.Simple {
		t.Fatal("the two instances share their encoding state")
	}

	// lay out text with the first instance only
	gid := first.Layout(nil, 10, "A").Seq[0].GID
	if _, ok := first.Encode(gid, "A"); !ok {
		t.Fatal("no code was allocated")
	}
	if first.Simple.CodesRemaining() == second.Simple.CodesRemaining() {
		t.Error("a code allocated in one instance was taken from the other")
	}
}

// TestNewSharesFontData checks that the instances of an extended font draw on
// one copy of the font data, which is what makes [Font.New] cheap after the
// first call.
func TestNewSharesFontData(t *testing.T) {
	first := font.Must(NimbusRomanRegular.New())
	second := font.Must(NimbusRomanRegular.New())

	if first.Font != second.Font {
		t.Error("the font programs are not shared")
	}
	if first.Metrics != second.Metrics {
		t.Error("the metrics are not shared")
	}
	if first.Geometry != second.Geometry {
		t.Error("the geometry is not shared")
	}
}

// TestNewUnknownFont checks that a Font value this package does not name, and
// so has nothing to share, reports the missing data as an error.
func TestNewUnknownFont(t *testing.T) {
	if _, err := Font(len(All)).New(); err == nil {
		t.Error("expected an error for an unknown font")
	}
}
