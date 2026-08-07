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

package bundled

import (
	"testing"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/postscript/afm"
)

// The two halves of a bundled font describe one font, so the metrics must
// cover the glyphs the font program draws.
func TestRead(t *testing.T) {
	psFont, metrics, err := Read("Helvetica")
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"A", "space", "quoteright"} {
		if _, ok := psFont.Glyphs[name]; !ok {
			t.Errorf("the font program draws no %q", name)
		}
		if _, ok := metrics.Glyphs[name]; !ok {
			t.Errorf("the metrics cover no %q", name)
		}
	}
}

// A name this library bundles no font for is an error rather than an empty
// font: the packages built on this one treat it as a broken installation.
func TestReadUnknown(t *testing.T) {
	if _, _, err := Read("NoSuchFont"); err == nil {
		t.Error("a font which is not bundled was read without an error")
	}
}

// Some bundled fonts give a whitespace glyph a bounding box, which would make
// an empty glyph take up room on the page.  Only the box is wrong, so the
// width must survive.
func TestFixUpMetricsClearsWhitespaceBBox(t *testing.T) {
	metrics := &afm.Metrics{
		Glyphs: map[string]*afm.GlyphInfo{
			"space": {WidthX: 278, BBox: rect.Rect{LLx: 10, URx: 20, URy: 100}},
		},
	}
	FixUpMetrics(metrics)

	if got := metrics.Glyphs["space"].BBox; !got.IsZero() {
		t.Errorf("the space glyph covers %v, want nothing", got)
	}
	if got := metrics.Glyphs["space"].WidthX; got != 278 {
		t.Errorf("the space glyph is %v wide, want 278", got)
	}
}

// The bundled .afm files carry no ascent or descent, so both are inferred from
// the glyphs which reach furthest up and down.
func TestFixUpMetricsInfersAscentDescent(t *testing.T) {
	metrics := &afm.Metrics{
		Glyphs: map[string]*afm.GlyphInfo{
			"d": {BBox: rect.Rect{URy: 718}},
			"p": {BBox: rect.Rect{LLy: -207}},
		},
	}
	FixUpMetrics(metrics)

	if metrics.Ascent != 718 {
		t.Errorf("ascent %v, want 718", metrics.Ascent)
	}
	if metrics.Descent != -207 {
		t.Errorf("descent %v, want -207", metrics.Descent)
	}
}

// The inferred values describe the glyphs the font has, which is why a caller
// which restricts the glyph set must do so before the repairs are made.
func TestFixUpMetricsFollowsTheGlyphSet(t *testing.T) {
	metrics := &afm.Metrics{
		Glyphs: map[string]*afm.GlyphInfo{
			"p": {BBox: rect.Rect{LLy: -207}},
		},
	}
	FixUpMetrics(metrics)

	if metrics.Ascent != 0 {
		t.Errorf("ascent %v was inferred from a glyph the font does not have",
			metrics.Ascent)
	}
}

// The bundled .afm files leave out the standard ligatures.  One is added only
// where the font draws all three glyphs it involves: a ligature to a glyph the
// font does not have would replace the text with ".notdef".
func TestFixUpMetricsAddsLigatures(t *testing.T) {
	metrics := &afm.Metrics{
		Glyphs: map[string]*afm.GlyphInfo{
			"f": {}, "i": {}, "l": {}, "fi": {},
		},
	}
	FixUpMetrics(metrics)

	if got := metrics.Glyphs["f"].Ligatures["i"]; got != "fi" {
		t.Errorf("f followed by i gives %q, want %q", got, "fi")
	}
	if got, ok := metrics.Glyphs["f"].Ligatures["l"]; ok {
		t.Errorf("f followed by l gives %q, but the font draws no %q", got, "fl")
	}
}

// A monospaced font draws every glyph at the same width, so a ligature would
// take the place of two glyphs while occupying one.
func TestFixUpMetricsSkipsLigaturesForFixedPitch(t *testing.T) {
	metrics := &afm.Metrics{
		IsFixedPitch: true,
		Glyphs: map[string]*afm.GlyphInfo{
			"f": {}, "i": {}, "fi": {},
		},
	}
	FixUpMetrics(metrics)

	if got := metrics.Glyphs["f"].Ligatures; len(got) != 0 {
		t.Errorf("a monospaced font got the ligatures %v", got)
	}
}
