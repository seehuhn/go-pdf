// Copyright 2023 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package truetype_test

import (
	"math"
	"testing"

	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/internal/cidtest"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestCompositeRoundTrip embeds a TrueType composite (CID) font and reads
// it back, checking the identifying fields.
func TestCompositeRoundTrip(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		t.Run(v.String(), func(t *testing.T) {
			fontData := makefont.TrueType()
			d := embedTrueTypeComposite(t, v, fontData)

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

// TestCompositeDescriptor verifies that the descriptor reports metrics in
// PDF glyph space (1/1000 em) and follows the Ascent-Descent+LineGap
// convention for Leading.
func TestCompositeDescriptor(t *testing.T) {
	fontData := makefont.TrueType()
	d := embedTrueTypeComposite(t, pdf.V2_0, fontData)
	checkDescriptorMetrics(t, d.Descriptor, fontData)
}

// TestCompositeLayout verifies that Layout returns positive advances that
// scale linearly with ptSize.
func TestCompositeLayout(t *testing.T) {
	fontData := makefont.TrueType()
	F, err := truetype.NewComposite(fontData, &truetype.OptionsComposite{CMap: cmap.IdentityH})
	if err != nil {
		t.Fatal(err)
	}

	const ptSize = 12.0
	seq := F.Layout(nil, ptSize, "Hello")
	if len(seq.Seq) == 0 {
		t.Fatal("Layout returned empty glyph sequence")
	}
	for i, g := range seq.Seq {
		if g.Advance <= 0 {
			t.Errorf("glyph %d (GID %d): advance %v, want > 0", i, g.GID, g.Advance)
		}
	}

	seq2 := F.Layout(nil, 2*ptSize, "Hello")
	if got, want := totalAdvance(seq2), 2*totalAdvance(seq); math.Abs(got-want) > 1e-9 {
		t.Errorf("total advance at 2x ptSize: got %v, want %v", got, want)
	}
}

func embedTrueTypeComposite(t *testing.T, v pdf.Version, fontData *sfnt.Font) *dict.CIDFontType2 {
	t.Helper()
	w, _ := memfile.NewPDFWriter(t, v, nil)
	rm := pdf.NewResourceManager(w)

	F, err := truetype.NewComposite(fontData, &truetype.OptionsComposite{CMap: cmap.IdentityH})
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

// TestCompositeVerticalMetrics checks the DW2 and W2 entries written for a
// font with vertical metrics, using a predefined and a UTF-8 vertical CMap.
// One glyph has a vertical advance of zero.
func TestCompositeVerticalMetrics(t *testing.T) {
	for _, c := range []*cmap.File{cmap.IdentityV, cmap.UTF8V} {
		t.Run(c.Name, func(t *testing.T) {
			info := makefont.TrueType()
			cidtest.ZeroAdvance(t, info, 'w')
			F, err := truetype.NewComposite(info, &truetype.OptionsComposite{CMap: c})
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
			F, err := truetype.NewComposite(makefont.TrueType(), &truetype.OptionsComposite{CMap: c})
			if err != nil {
				t.Fatal(err)
			}
			cidtest.CheckHorizontal(t, F, verticalTestText)
		})
	}
}

// verticalTestText mixes glyphs with the two kinds of vertical metrics of
// the makefont fonts.
const verticalTestText = "Hello, vertical world."

// TestCompositeSharedCID checks that two glyphs sharing a CID are written with the
// metrics of the glyph the embedded font has for that CID.
func TestCompositeSharedCID(t *testing.T) {
	for _, wMode := range []font.WritingMode{font.Horizontal, font.Vertical} {
		t.Run(wMode.String(), func(t *testing.T) {
			info := makefont.TrueType()
			f, gidToCID := cidtest.SharedCIDCMap(t, info, wMode)
			F, err := truetype.NewComposite(info, &truetype.OptionsComposite{CMap: f, GIDToCID: gidToCID})
			if err != nil {
				t.Fatal(err)
			}
			cidtest.CheckSharedCID(t, F, info)
		})
	}
}
