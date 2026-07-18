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

package varfont

import (
	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/fvar"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/hvar"
	"seehuhn.de/go/sfnt/os2"
	"seehuhn.de/go/sfnt/variation"
)

// glyph IDs of the synthetic CFF2 fonts returned by [CFF2] and [StaticCFF2].
const (
	CFF2GIDBox     glyph.ID = 1 // box glyph ("A"), widens at wght=+1 in CFF2
	CFF2GIDFillerB glyph.ID = 2 // filler glyph ("B"), same FD as the other fillers
)

// cff2Box returns a rectangular CFF2 glyph from (0,0) to (w,h).  No argument
// carries a variation delta; used both for the static fixture and for the
// non-varying glyphs of the variable fixture.
func cff2Box(w, h float64) *cff.GlyphCFF2 {
	b := func(v float64) cff.Blend { return cff.Blend{Default: v} }
	return &cff.GlyphCFF2{Cmds: []cff.GlyphOpCFF2{
		{Op: cff.OpMoveTo, Args: []cff.Blend{b(0), b(0)}},
		{Op: cff.OpLineTo, Args: []cff.Blend{b(w), b(0)}},
		{Op: cff.OpLineTo, Args: []cff.Blend{b(w), b(h)}},
		{Op: cff.OpLineTo, Args: []cff.Blend{b(0), b(h)}},
	}}
}

// cff2VarBox returns a rectangular CFF2 glyph from (0,0) to (w,h), whose
// right edge widens by dw at the wght=+1 extreme.
func cff2VarBox(w, h, dw float64) *cff.GlyphCFF2 {
	b := func(v float64) cff.Blend { return cff.Blend{Default: v} }
	bv := func(v, d float64) cff.Blend { return cff.Blend{Default: v, Deltas: []float64{d}} }
	return &cff.GlyphCFF2{Cmds: []cff.GlyphOpCFF2{
		{Op: cff.OpMoveTo, Args: []cff.Blend{b(0), b(0)}},
		{Op: cff.OpLineTo, Args: []cff.Blend{bv(w, dw), b(0)}},
		{Op: cff.OpLineTo, Args: []cff.Blend{bv(w, dw), b(h)}},
		{Op: cff.OpLineTo, Args: []cff.Blend{b(0), b(h)}},
	}}
}

// cff2FontShell builds the non-outline parts shared by [CFF2] and
// [StaticCFF2]: family metrics, a cmap mapping "A" to [CFF2GIDBox] and "B" to
// [CFF2GIDFillerB], and two FDs (glyphs 0-1 in FD 0, glyphs 2-5 in FD 1) so
// that a composite embedding touching both letters is CID-keyed rather than
// collapsing to a simple font, while a composite or simple embedding using
// only "A" subsets down to a single FD.
func cff2FontShell(familyName string, glyphs []*cff.GlyphCFF2, widths []float64) *sfnt.Font {
	o := &cff.OutlinesCFF2{
		Glyphs:  glyphs,
		Widths:  widths,
		Private: []*cff.PrivateCFF2{{}, {}},
		FDSelect: func(gid glyph.ID) int {
			if gid <= CFF2GIDBox {
				return 0
			}
			return 1
		},
	}

	f := &sfnt.Font{
		FamilyName:         familyName,
		Width:              os2.WidthNormal,
		Weight:             os2.WeightNormal,
		UnitsPerEm:         1000,
		Ascent:             700,
		Descent:            -300,
		LineGap:            100,
		CapHeight:          700,
		XHeight:            500,
		UnderlinePosition:  -100,
		UnderlineThickness: 50,
		FontMatrix:         matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
		Outlines:           o,
	}

	m := cmap.Format4{}
	m['A'] = CFF2GIDBox
	m['B'] = CFF2GIDFillerB
	f.InstallCMap(m)

	return f
}

// CFF2 returns a self-contained single-axis variable CFF2 font: a wght axis,
// a box glyph ("A") whose right edge widens from 500 to 600 at the +1 end
// (via a CFF2 blend) with a matching HVAR advance-width delta (550 -> 600),
// plus filler glyphs so that embedding a subset of the characters produces a
// subset tag.  The shape mirrors go-render's buildVarCFF2 and go-sfnt's
// makeVarCFF2Font fixtures.
func CFF2() *sfnt.Font {
	notdef := &cff.GlyphCFF2{Cmds: []cff.GlyphOpCFF2{
		{Op: cff.OpMoveTo, Args: []cff.Blend{{Default: 0}, {Default: 0}}},
	}}
	box := cff2VarBox(500, 700, 100)
	fillerB := cff2Box(300, 600)
	filler := func() *cff.GlyphCFF2 { return cff2Box(300, 600) }

	glyphs := []*cff.GlyphCFF2{notdef, box, fillerB, filler(), filler(), filler()}
	widths := []float64{600, 550, 400, 400, 400, 400}

	f := cff2FontShell("QuireMiniCFF2Var", glyphs, widths)
	o := f.Outlines.(*cff.OutlinesCFF2)

	peak := variation.Region{{Start: 0, Peak: f2(1), End: f2(1)}}
	o.VarStore = &variation.ItemVariationStore{
		Regions: []variation.Region{peak},
		Data: []*variation.ItemVariationData{
			{RegionIndexes: []uint16{0}, Deltas: [][]int32{}},
		},
	}

	f.Fvar = &fvar.Table{
		Axes: []fvar.Axis{{Tag: "wght", Min: 100, Default: 400, Max: 900, Name: "Weight"}},
	}
	// advance of the box glyph 550 -> 600 at the +1 end; all other glyphs static
	f.Hvar = &hvar.Table{
		Store: &variation.ItemVariationStore{
			Regions: []variation.Region{peak},
			Data: []*variation.ItemVariationData{
				{RegionIndexes: []uint16{0}, Deltas: [][]int32{{0}, {50}}},
			},
		},
		AdvanceMap: &variation.DeltaSetIndexMap{Map: []uint32{0, 1, 0, 0, 0, 0}},
	}
	f.VariationsPostScriptName = "QuireMiniCFF2Var-"

	return f
}

// StaticCFF2 is the non-variable counterpart of [CFF2]: the same glyph
// shapes at the coordinates [CFF2] uses at its default instance (box right
// edge 500, advance 550), with no variation tables.
func StaticCFF2() *sfnt.Font {
	notdef := &cff.GlyphCFF2{Cmds: []cff.GlyphOpCFF2{
		{Op: cff.OpMoveTo, Args: []cff.Blend{{Default: 0}, {Default: 0}}},
	}}
	box := cff2Box(500, 700)
	filler := func() *cff.GlyphCFF2 { return cff2Box(300, 600) }

	glyphs := []*cff.GlyphCFF2{notdef, box, filler(), filler(), filler(), filler()}
	widths := []float64{600, 550, 400, 400, 400, 400}

	return cff2FontShell("QuireMiniCFF2Static", glyphs, widths)
}
