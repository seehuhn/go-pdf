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

package fontdesc

import (
	"testing"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/postscript/afm"
)

func testMetrics() *afm.Metrics {
	return &afm.Metrics{
		FamilyName:   "Test",
		Weight:       "Bold",
		CapHeight:    700,
		XHeight:      500,
		Ascent:       800,
		Descent:      -200,
		StdHW:        76.4,
		StdVW:        88,
		ItalicAngle:  -12.34,
		IsFixedPitch: true,
		Glyphs: map[string]*afm.GlyphInfo{
			"H": {WidthX: 700, BBox: rect.Rect{LLx: -100, LLy: -210, URx: 900, URy: 650}},
			"y": {WidthX: 500, BBox: rect.Rect{LLy: -210, URy: 790}},
			"x": {WidthX: 500, BBox: rect.Rect{URy: 450}},
		},
	}
}

func TestFromAFM(t *testing.T) {
	d := FromAFM(testMetrics())
	if d.StemV != 88 || d.StemH != 76 {
		t.Errorf("wrong stems: StemV %g, StemH %g", d.StemV, d.StemH)
	}
	if d.Ascent != 800 || d.Descent != -200 || d.CapHeight != 700 || d.XHeight != 500 {
		t.Errorf("wrong heights: %g %g %g %g", d.Ascent, d.Descent, d.CapHeight, d.XHeight)
	}
	if d.FontFamily != "Test" || !d.IsFixedPitch || !d.IsItalic || d.ItalicAngle != -12.3 {
		t.Errorf("wrong style: %+v", d)
	}
	if d.FontWeight == 0 {
		t.Error("missing font weight")
	}

	m := testMetrics()
	m.StdHW, m.StdVW, m.ItalicAngle = 0, 0, 0
	d = FromAFM(m)
	if d.StemV != 0 || d.StemH != 0 || d.IsItalic {
		t.Errorf("unexpected stems or italic: %+v", d)
	}
}

func TestAFMHeights(t *testing.T) {
	ascent, descent, capHeight, xHeight := AFMHeights(testMetrics())
	if ascent != 800 || descent != -200 || capHeight != 700 || xHeight != 500 {
		t.Errorf("recorded heights not used: %g %g %g %g", ascent, descent, capHeight, xHeight)
	}

	// omitted heights are taken from the bounding boxes
	m := testMetrics()
	m.Ascent, m.Descent, m.CapHeight, m.XHeight = 0, 0, 0, 0
	ascent, descent, capHeight, xHeight = AFMHeights(m)
	if ascent != 790 || descent != -210 || capHeight != 650 || xHeight != 450 {
		t.Errorf("wrong fallbacks: %g %g %g %g", ascent, descent, capHeight, xHeight)
	}

	// without the glyphs the heights are 0
	m.Glyphs = nil
	_, _, capHeight, xHeight = AFMHeights(m)
	if capHeight != 0 || xHeight != 0 {
		t.Errorf("heights without glyphs: %g %g", capHeight, xHeight)
	}
}
