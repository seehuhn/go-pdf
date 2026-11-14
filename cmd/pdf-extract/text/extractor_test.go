// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package text

import (
	"bytes"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/page"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/property"
)

func TestTextExtractorBasic(t *testing.T) {
	// create test PDF with simple text
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	F := font.Must(standard.Helvetica.New())

	pageTree := pagetree.NewWriter(w, rm)

	b := builder.New(content.Page, nil, pdf.V2_0)
	b.TextBegin()
	b.TextFirstLine(100, 700)
	b.TextSetFont(F, 12)
	b.TextShow("Hello World")
	b.TextEnd()

	if b.Err != nil {
		t.Fatal(b.Err)
	}

	p := &page.Page{
		MediaBox:  &pdf.Rectangle{LLx: 0, LLy: 0, URx: 595, URy: 842},
		Resources: b.Resources,
		Contents:  []page.Segment{&content.Operators{Ops: b.Stream}},
	}
	err := pageTree.AppendPage(p)
	if err != nil {
		t.Fatal(err)
	}

	treeRef, err := pageTree.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	w.GetMeta().Catalog.Pages = treeRef
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	// extract text
	var buf bytes.Buffer
	extractor := New(w, &buf)
	_, pageDict, err := pagetree.GetPage(w, 0)
	if err != nil {
		t.Fatal(err)
	}

	err = extractor.ExtractPage(pageDict)
	if err != nil {
		t.Fatal(err)
	}

	extracted := buf.String()
	if !strings.Contains(extracted, "Hello World") {
		t.Errorf("extracted text %q does not contain \"Hello World\"", extracted)
	}
}

func TestTextExtractorActualTextIndirect(t *testing.T) {
	// create test PDF with ActualText via resource reference (not inline)
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	F := font.Must(standard.Helvetica.New())

	pageTree := pagetree.NewWriter(w, rm)

	b := builder.New(content.Page, nil, pdf.V2_0)

	b.TextBegin()
	b.TextFirstLine(100, 700)
	b.TextSetFont(F, 12)
	b.TextShow("the ")

	// ActualText via resource reference (SingleUse=false, Inline=false)
	b.MarkedContentStart(&graphics.MarkedContent{
		Tag: "Span",
		Properties: &property.ActualText{
			Text:      pdf.Text("replaced"),
			SingleUse: false,
		},
		Inline: false,
	})
	b.TextShow("original")
	b.MarkedContentEnd()

	b.TextShow(" text")
	b.TextEnd()

	if b.Err != nil {
		t.Fatal(b.Err)
	}

	p := &page.Page{
		MediaBox:  &pdf.Rectangle{LLx: 0, LLy: 0, URx: 595, URy: 842},
		Resources: b.Resources,
		Contents:  []page.Segment{&content.Operators{Ops: b.Stream}},
	}
	err := pageTree.AppendPage(p)
	if err != nil {
		t.Fatal(err)
	}

	treeRef, err := pageTree.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	w.GetMeta().Catalog.Pages = treeRef
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	extractor := New(w, &buf)

	_, pageDict, err := pagetree.GetPage(w, 0)
	if err != nil {
		t.Fatal(err)
	}

	err = extractor.ExtractPage(pageDict)
	if err != nil {
		t.Fatal(err)
	}

	extracted := strings.TrimSpace(buf.String())
	if !strings.Contains(extracted, "replaced") {
		t.Errorf("extracted text %q does not contain \"replaced\"", extracted)
	}
	if strings.Contains(extracted, "original") {
		t.Errorf("extracted text %q contains \"original\"", extracted)
	}
}

func TestTextExtractorActualText(t *testing.T) {
	// create test PDF with ActualText
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	F := font.Must(standard.Helvetica.New())

	pageTree := pagetree.NewWriter(w, rm)

	b := builder.New(content.Page, nil, pdf.V2_0)

	// The string "the original text" is rendered as three text objects so
	// that the MarkedContent / ActualText boundary lies outside the BT/ET
	// pair.  The positions are chosen so that the three pieces flow on a
	// single line, matching how a PDF generator would lay them out.
	const (
		startX         = 100.0
		baseline       = 700.0
		theWidth       = 20.016 // Helvetica "the " at 12pt
		originalWidth  = 38.676 // Helvetica "original" at 12pt
		afterTheX      = startX + theWidth
		afterOriginalX = afterTheX + originalWidth
	)

	// normal text
	b.TextBegin()
	b.TextFirstLine(startX, baseline)
	b.TextSetFont(F, 12)
	b.TextShow("the ")
	b.TextEnd()

	// text with ActualText
	actualText := &property.ActualText{
		Text:      pdf.Text("replaced"),
		SingleUse: true,
	}
	mc := &graphics.MarkedContent{
		Tag:        "Span",
		Properties: actualText,
		Inline:     true,
	}
	b.MarkedContentStart(mc)
	b.TextBegin()
	b.TextFirstLine(afterTheX, baseline)
	b.TextSetFont(F, 12)
	b.TextShow("original")
	b.TextEnd()
	b.MarkedContentEnd()

	// more normal text
	b.TextBegin()
	b.TextFirstLine(afterOriginalX, baseline)
	b.TextSetFont(F, 12)
	b.TextShow(" text")
	b.TextEnd()

	if b.Err != nil {
		t.Fatal(b.Err)
	}

	p := &page.Page{
		MediaBox:  &pdf.Rectangle{LLx: 0, LLy: 0, URx: 595, URy: 842},
		Resources: b.Resources,
		Contents:  []page.Segment{&content.Operators{Ops: b.Stream}},
	}
	err := pageTree.AppendPage(p)
	if err != nil {
		t.Fatal(err)
	}

	treeRef, err := pageTree.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	w.GetMeta().Catalog.Pages = treeRef
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	extractor := New(w, &buf)

	_, pageDict, err := pagetree.GetPage(w, 0)
	if err != nil {
		t.Fatal(err)
	}

	err = extractor.ExtractPage(pageDict)
	if err != nil {
		t.Fatal(err)
	}

	extracted := strings.TrimSpace(buf.String())
	const want = "the replaced text"
	if extracted != want {
		t.Errorf("got %q, want %q", extracted, want)
	}
}

// TestTextExtractorNoExplicitTextMatrix checks that text shown without any
// BT/ET or Td/Tm operator, so that the text matrix was never explicitly
// set, extracts without panicking.
func TestTextExtractorNoExplicitTextMatrix(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	F := font.Must(standard.Helvetica.New())

	pageTree := pagetree.NewWriter(w, rm)

	// Build the operators by hand: a /Tf followed by a bare /Tj at page
	// level (no BT/ET, no Td/Tm) leaves the text-matrix "set" flag clear,
	// while the reader still fires the character callback because the
	// font is known.
	b := builder.New(content.Page, nil, pdf.V2_0)
	if err := b.RegisterFont("F1", F); err != nil {
		t.Fatal(err)
	}
	ops := &content.Operators{Ops: []content.Operator{
		{Name: content.OpTextSetFont, Args: []pdf.Object{pdf.Name("F1"), pdf.Integer(12)}},
		{Name: content.OpTextShow, Args: []pdf.Object{pdf.String("hello")}},
	}}

	p := &page.Page{
		MediaBox:  &pdf.Rectangle{LLx: 0, LLy: 0, URx: 595, URy: 842},
		Resources: b.Resources,
		Contents:  []page.Segment{ops},
	}
	if err := pageTree.AppendPage(p); err != nil {
		t.Fatal(err)
	}
	treeRef, err := pageTree.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	w.GetMeta().Catalog.Pages = treeRef
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	extractor := New(w, &buf)
	_, pageDict, err := pagetree.GetPage(w, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := extractor.ExtractPage(pageDict); err != nil {
		t.Fatal(err)
	}
}

// TestTextExtractorVerticalXRange checks that the x range applies to a
// glyph's horizontal origin, its left edge, in vertical writing mode as in
// horizontal writing mode.  There the text position is the vertical origin,
// to the right of it by the position vector.
func TestTextExtractorVerticalXRange(t *testing.T) {
	F, err := gofont.Regular.NewComposite(&truetype.OptionsComposite{CMap: cmap.IdentityV})
	if err != nil {
		t.Fatal(err)
	}
	gg := F.Layout(nil, 20, "H")
	code, ok := F.Encode(gg.Seq[0].GID, gg.Seq[0].Text)
	if !ok {
		t.Fatal("cannot encode H")
	}
	s := F.Codec().AppendCode(nil, code)
	var vx float64
	for c := range F.Codes(s) {
		vx = c.VerticalOriginX * 20
	}

	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	pageTree := pagetree.NewWriter(w, rm)
	b := builder.New(content.Page, nil, pdf.V2_0)
	b.TextBegin()
	b.TextFirstLine(100, 700)
	b.TextSetFont(F, 20)
	b.TextShowRaw(s)
	b.TextEnd()
	if b.Err != nil {
		t.Fatal(b.Err)
	}
	err = pageTree.AppendPage(&page.Page{
		MediaBox:  &pdf.Rectangle{URx: 595, URy: 842},
		Resources: b.Resources,
		Contents:  []page.Segment{&content.Operators{Ops: b.Stream}},
	})
	if err != nil {
		t.Fatal(err)
	}
	treeRef, err := pageTree.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	w.GetMeta().Catalog.Pages = treeRef
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		min  float64
		want string
	}{
		{100 - vx - 0.5, "H"},
		{100 - vx + 0.5, ""},
	} {
		var buf bytes.Buffer
		extractor := New(w, &buf)
		extractor.XRangeMin = tc.min
		_, pageDict, err := pagetree.GetPage(w, 0)
		if err != nil {
			t.Fatal(err)
		}
		if err := extractor.ExtractPage(pageDict); err != nil {
			t.Fatal(err)
		}
		if got := strings.TrimSpace(buf.String()); got != tc.want {
			t.Errorf("XRangeMin %g: extracted %q, want %q", tc.min, got, tc.want)
		}
	}
}
