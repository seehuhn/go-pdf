// Copyright 2023 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package opentype_test

import (
	"testing"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/internal/cidtest"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestCompositeGlyfRoundTrip embeds an OpenType+glyf composite font and
// reads it back, checking the identifying fields.
func TestCompositeGlyfRoundTrip(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		t.Run(v.String(), func(t *testing.T) {
			fontData := makefont.TrueType()
			d := embedCompositeGlyf(t, v, fontData)

			if d.PostScriptName != fontData.FontName {
				t.Errorf("PostScriptName: got %q, want %q",
					d.PostScriptName, fontData.FontName)
			}
			if len(d.SubsetTag) != 6 {
				t.Errorf("SubsetTag: got %q, want 6 uppercase letters", d.SubsetTag)
			}
		})
	}
}

// TestCompositeGlyfDescriptor verifies that the descriptor of a composite
// OpenType+glyf font reports metrics in PDF glyph space (1/1000 em) and
// follows the Ascent-Descent+LineGap convention for Leading.
func TestCompositeGlyfDescriptor(t *testing.T) {
	fontData := makefont.TrueType()
	fd := embedCompositeGlyf(t, pdf.V2_0, fontData).Descriptor
	checkDescriptorMetrics(t, fd, fontData)
}

func embedCompositeGlyf(t *testing.T, v pdf.Version, fontData *sfnt.Font) *dict.CIDFontType2 {
	t.Helper()
	w, _ := memfile.NewPDFWriter(t, v, nil)
	rm := pdf.NewResourceManager(w)

	F, err := opentype.NewComposite(fontData, &opentype.OptionsComposite{CMap: cmap.IdentityH})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := rm.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	gg := F.Layout(nil, 12, "Hello")
	for _, g := range gg.Seq {
		_, _ = F.Encode(g.GID, g.Text)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	dictObj, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := dictObj.(*dict.CIDFontType2)
	if !ok {
		t.Fatalf("wrong font dictionary type: %T", dictObj)
	}
	return d
}

// TestCompositeGlyfVerticalMetrics checks the DW2 and W2 entries written for a
// font with vertical metrics, using a predefined and a UTF-8 vertical CMap.
// One glyph has a vertical advance of zero.
func TestCompositeGlyfVerticalMetrics(t *testing.T) {
	for _, c := range []*cmap.File{cmap.IdentityV, cmap.UTF8V} {
		t.Run(c.Name, func(t *testing.T) {
			info := makefont.TrueType()
			cidtest.ZeroAdvance(t, info, 'w')
			F, err := opentype.NewComposite(info, &opentype.OptionsComposite{CMap: c})
			if err != nil {
				t.Fatal(err)
			}
			cidtest.CheckVertical(t, F.(*opentype.CompositeGlyf), info, verticalTestText)
		})
	}
}

// TestCompositeGlyfHorizontalMetrics checks that horizontal writing leaves
// out DW2 and W2, even though the font has vertical metrics.
func TestCompositeGlyfHorizontalMetrics(t *testing.T) {
	for _, c := range []*cmap.File{cmap.IdentityH, cmap.UTF8H} {
		t.Run(c.Name, func(t *testing.T) {
			F, err := opentype.NewComposite(makefont.TrueType(), &opentype.OptionsComposite{CMap: c})
			if err != nil {
				t.Fatal(err)
			}
			cidtest.CheckHorizontal(t, F.(*opentype.CompositeGlyf), verticalTestText)
		})
	}
}

// verticalTestText mixes glyphs with the two kinds of vertical metrics of
// the makefont fonts.
const verticalTestText = "Hello, vertical world."

// TestCompositeGlyfNotdef checks that the font as written and the font read back agree on
// every field of [font.Code], for a CMap with notdef mappings to CIDs other
// than 0, and that every code has the metrics of the CID it selects.
func TestCompositeGlyfNotdef(t *testing.T) {
	for _, wMode := range []font.WritingMode{font.Horizontal, font.Vertical} {
		t.Run(wMode.String(), func(t *testing.T) {
			info := makefont.TrueType()
			f, gidToCID := cidtest.NotdefCMap(t, info, wMode)
			F, err := opentype.NewComposite(info, &opentype.OptionsComposite{CMap: f, GIDToCID: gidToCID})
			if err != nil {
				t.Fatal(err)
			}
			cidtest.CheckNotdef(t, F.(*opentype.CompositeGlyf))
		})
	}
}

// TestCompositeGlyfSharedCID checks that two glyphs sharing a CID are written with the
// metrics of the glyph the embedded font has for that CID.
func TestCompositeGlyfSharedCID(t *testing.T) {
	for _, wMode := range []font.WritingMode{font.Horizontal, font.Vertical} {
		t.Run(wMode.String(), func(t *testing.T) {
			info := makefont.TrueType()
			f, gidToCID := cidtest.SharedCIDCMap(t, info, wMode)
			F, err := opentype.NewComposite(info, &opentype.OptionsComposite{CMap: f, GIDToCID: gidToCID})
			if err != nil {
				t.Fatal(err)
			}
			cidtest.CheckSharedCID(t, F.(*opentype.CompositeGlyf), info)
		})
	}
}

// TestCompositeNotdefGlyphGlyf checks that, with a CMap whose collection implies a
// glyph for CID 0, CID 0 is still glyph 0 of the font.
func TestCompositeNotdefGlyphGlyf(t *testing.T) {
	info := makefont.TrueType()
	f, err := cmap.Predefined("Adobe-Japan1-7")
	if err != nil {
		t.Fatal(err)
	}
	F, err := opentype.NewComposite(info, &opentype.OptionsComposite{CMap: f})
	if err != nil {
		t.Fatal(err)
	}
	cidtest.CheckNotdefGlyph(t, F.(*opentype.CompositeGlyf), info, "Hello")
}

// TestCompositeBadGIDToCIDGlyf checks that a GIDToCID which does not map CID 0
// and glyph 0 to each other is rejected.
func TestCompositeBadGIDToCIDGlyf(t *testing.T) {
	info := makefont.TrueType()
	f, gidToCID := cidtest.NotdefCMap(t, info, font.Horizontal)
	if _, err := opentype.NewComposite(info, &opentype.OptionsComposite{CMap: f, GIDToCID: gidToCID}); err != nil {
		t.Fatal(err)
	}

	for _, g2c := range []map[glyph.ID]cid.CID{{5: 0}, {0: 3}} {
		bad := cmap.NewGIDToCIDFromMap(gidToCID.ROS(), g2c)
		_, err := opentype.NewComposite(info, &opentype.OptionsComposite{CMap: f, GIDToCID: bad})
		if err == nil {
			t.Errorf("table %v accepted", g2c)
		}
	}
}
