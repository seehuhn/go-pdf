// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cff_test

import (
	"testing"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/internal/cidtest"
	"seehuhn.de/go/pdf/internal/debug/makefont"
)

// verticalTestText mixes glyphs with the two kinds of vertical metrics of
// the makefont fonts.
const verticalTestText = "Hello, vertical world."

// TestCompositeVerticalMetrics checks the DW2 and W2 entries written for a
// font with vertical metrics, using a predefined and a UTF-8 vertical CMap.
// One glyph has a vertical advance of zero.
func TestCompositeVerticalMetrics(t *testing.T) {
	for _, c := range []*cmap.File{cmap.IdentityV, cmap.UTF8V} {
		t.Run(c.Name, func(t *testing.T) {
			info := makefont.OpenType()
			cidtest.ZeroAdvance(t, info, 'w')
			F, err := cff.NewComposite(info, &cff.OptionsComposite{CMap: c})
			if err != nil {
				t.Fatal(err)
			}
			cidtest.CheckVertical(t, F, info, verticalTestText)
		})
	}
}

// TestCompositeHorizontalMetrics checks that horizontal writing leaves
// out DW2 and W2, even though the font has vertical metrics.
func TestCompositeHorizontalMetrics(t *testing.T) {
	for _, c := range []*cmap.File{cmap.IdentityH, cmap.UTF8H} {
		t.Run(c.Name, func(t *testing.T) {
			F, err := cff.NewComposite(makefont.OpenType(), &cff.OptionsComposite{CMap: c})
			if err != nil {
				t.Fatal(err)
			}
			cidtest.CheckHorizontal(t, F, verticalTestText)
		})
	}
}

// TestCompositeNotdef checks that the font as written and the font read back agree on
// every field of [font.Code], for a CMap with notdef mappings to CIDs other
// than 0, and that every code has the metrics of the CID it selects.
func TestCompositeNotdef(t *testing.T) {
	for _, wMode := range []font.WritingMode{font.Horizontal, font.Vertical} {
		t.Run(wMode.String(), func(t *testing.T) {
			info := makefont.OpenType()
			f, gidToCID := cidtest.NotdefCMap(t, info, wMode)
			F, err := cff.NewComposite(info, &cff.OptionsComposite{CMap: f, GIDToCID: gidToCID})
			if err != nil {
				t.Fatal(err)
			}
			cidtest.CheckNotdef(t, F)
		})
	}
}

// TestCompositeSharedCID checks that two glyphs sharing a CID are written with the
// metrics of the glyph the embedded font has for that CID.
func TestCompositeSharedCID(t *testing.T) {
	for _, wMode := range []font.WritingMode{font.Horizontal, font.Vertical} {
		t.Run(wMode.String(), func(t *testing.T) {
			info := makefont.OpenType()
			f, gidToCID := cidtest.SharedCIDCMap(t, info, wMode)
			F, err := cff.NewComposite(info, &cff.OptionsComposite{CMap: f, GIDToCID: gidToCID})
			if err != nil {
				t.Fatal(err)
			}
			cidtest.CheckSharedCID(t, F, info)
		})
	}
}

// TestCompositeNotdefGlyph checks that, with a CMap whose collection implies a
// glyph for CID 0, CID 0 is still glyph 0 of the font.
func TestCompositeNotdefGlyph(t *testing.T) {
	info := makefont.OpenType()
	f, err := cmap.Predefined("Adobe-Japan1-7")
	if err != nil {
		t.Fatal(err)
	}
	F, err := cff.NewComposite(info, &cff.OptionsComposite{CMap: f})
	if err != nil {
		t.Fatal(err)
	}
	cidtest.CheckNotdefGlyph(t, F, info, "Hello")
}

// TestCompositeBadGIDToCID checks that a GIDToCID which does not map CID 0
// and glyph 0 to each other is rejected.
func TestCompositeBadGIDToCID(t *testing.T) {
	info := makefont.OpenType()
	f, gidToCID := cidtest.NotdefCMap(t, info, font.Horizontal)
	if _, err := cff.NewComposite(info, &cff.OptionsComposite{CMap: f, GIDToCID: gidToCID}); err != nil {
		t.Fatal(err)
	}

	for _, g2c := range []map[glyph.ID]cid.CID{{5: 0}, {0: 3}} {
		bad := cmap.NewGIDToCIDFromMap(gidToCID.ROS(), g2c)
		_, err := cff.NewComposite(info, &cff.OptionsComposite{CMap: f, GIDToCID: bad})
		if err == nil {
			t.Errorf("table %v accepted", g2c)
		}
	}
}
