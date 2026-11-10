// Copyright 2023 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package opentype_test

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestSimpleGlyfDescriptor verifies that the PDF font descriptor of an
// OpenType+glyf simple font reports metrics in PDF glyph space (1/1000 em)
// and follows the Ascent-Descent+LineGap convention for Leading.  Earlier
// revisions of makeDict scaled some fields incorrectly and computed Leading
// as just LineGap; this test pins the formula.
func TestSimpleGlyfDescriptor(t *testing.T) {
	fontData := makefont.TrueType()

	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	F, err := opentype.NewSimple(fontData, nil)
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
	d, ok := dictObj.(*dict.TrueType)
	if !ok {
		t.Fatalf("wrong font dictionary type: %T", dictObj)
	}
	checkDescriptorMetrics(t, d.Descriptor, fontData)
}
