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

// Package varfont builds a synthetic TrueType variable font for unit tests.
package varfont

import (
	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/postscript/funit"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/fvar"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/gvar"
	"seehuhn.de/go/sfnt/hvar"
	"seehuhn.de/go/sfnt/maxp"
	"seehuhn.de/go/sfnt/os2"
	"seehuhn.de/go/sfnt/variation"
)

// glyph IDs of the synthetic font returned by [Glyf].
const (
	GIDNotdef glyph.ID = 0 // a plain box, no variation
	GIDRect   glyph.ID = 1 // rectangle, top +200 and advance +100 at wght=+1
)

// f2 converts a normalized coordinate to F2Dot14.
func f2(x float64) variation.F2Dot14 { return variation.F2Dot14FromFloat(x) }

// simpleGlyph builds an all-on-curve single-contour glyph.
func simpleGlyph(pts ...[2]funit.Int16) *glyf.Glyph {
	contour := make(glyf.Contour, len(pts))
	for i, p := range pts {
		contour[i] = glyf.Point{X: p[0], Y: p[1], OnCurve: true}
	}
	su := &glyf.SimpleUnpacked{Contours: []glyf.Contour{contour}}
	g := su.AsGlyph()
	return &g
}

// Glyf returns a self-contained single-axis TrueType variable font with glyf
// outlines.  The axis is wght (100–900, default 400).  Glyph 1 ("A") rises by
// +200 at the top and gains +100 advance width at wght=+1 (via gvar and HVAR).
func Glyf() *sfnt.Font {
	notdef := simpleGlyph([2]funit.Int16{0, 0}, [2]funit.Int16{500, 0},
		[2]funit.Int16{500, 700}, [2]funit.Int16{0, 700})
	rect := simpleGlyph([2]funit.Int16{100, 0}, [2]funit.Int16{400, 0},
		[2]funit.Int16{400, 700}, [2]funit.Int16{100, 700})

	// filler glyphs so that embedding a single character subsets the font and
	// produces a subset tag.
	filler := func() *glyf.Glyph {
		return simpleGlyph([2]funit.Int16{0, 0}, [2]funit.Int16{300, 0},
			[2]funit.Int16{300, 600}, [2]funit.Int16{0, 600})
	}

	outlines := &glyf.Outlines{
		Glyphs: glyf.Glyphs{notdef, rect, filler(), filler(), filler(), filler()},
		Widths: []funit.Uint16{600, 500, 400, 400, 400, 400},
		Names:  []string{".notdef", "A", "b0", "b1", "b2", "b3"},
		Maxp: &maxp.TTFInfo{
			MaxPoints:   4,
			MaxContours: 1,
		},
	}

	f := &sfnt.Font{
		FamilyName:         "QuireMiniVar",
		Width:              os2.WidthNormal,
		Weight:             os2.WeightNormal,
		IsRegular:          true,
		UnitsPerEm:         1000,
		FontMatrix:         matrix.Scale(0.001, 0.001),
		Ascent:             800,
		Descent:            -200,
		LineGap:            200,
		CapHeight:          700,
		XHeight:            500,
		UnderlinePosition:  -100,
		UnderlineThickness: 50,
		Outlines:           outlines,
	}

	m := cmap.Format4{}
	m['A'] = GIDRect
	f.InstallCMap(m)

	f.Fvar = &fvar.Table{
		Axes: []fvar.Axis{
			{Tag: "wght", Min: 100, Default: 400, Max: 900, Name: "Weight"},
		},
	}

	// gvar: rect top corners rise by +200 at wght=+1 (advance left to HVAR)
	rectBlock := mustEncodeTuples([]variation.TupleVariation{
		{
			Peak: []variation.F2Dot14{f2(1)},
			Deltas: []int32{
				0, 0, 0, 0, 0, 0, 0, 0, // x (4 outline + 4 phantom)
				0, 0, 200, 200, 0, 0, 0, 0, // y (top corners +200)
			},
		},
	})
	f.Gvar = &gvar.Table{
		AxisCount: 1,
		PerGlyph: []gvar.GlyphData{
			{},                // notdef: no variation
			{Data: rectBlock}, // rect
			{}, {}, {}, {},    // fillers: no variation
		},
	}

	// HVAR: rect advance +100 at wght=+1
	f.Hvar = &hvar.Table{
		Store: &variation.ItemVariationStore{
			Regions: []variation.Region{
				{{Start: 0, Peak: f2(1), End: f2(1)}},
			},
			Data: []*variation.ItemVariationData{
				{
					RegionIndexes: []uint16{0},
					Deltas: [][]int32{
						{0},   // inner 0: no variation
						{100}, // inner 1: rect advance +100
					},
				},
			},
		},
		AdvanceMap: &variation.DeltaSetIndexMap{
			// rect -> inner 1 (varies); all others -> inner 0 (no variation)
			Map: []uint32{0, 1, 0, 0, 0, 0},
		},
	}

	f.VariationsPostScriptName = "QuireMiniVar-"

	return f
}

func mustEncodeTuples(tuples []variation.TupleVariation) []byte {
	data, err := variation.EncodeTupleData(tuples, 1, 2, 0, nil)
	if err != nil {
		panic(err)
	}
	return data
}
