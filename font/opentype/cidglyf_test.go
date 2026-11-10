// Copyright 2023 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package opentype_test

import (
	"testing"

	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
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
