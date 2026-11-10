// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package dict_test

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/type1glyphs"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/fonttypes"
	"seehuhn.de/go/pdf/page"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/reader"
)

func TestType1Embedding(t *testing.T) {
	const (
		testText    = "Helllo"
		uniqueChars = 4 // 'H', 'e', 'l', 'o'
	)

	buf := &bytes.Buffer{}

	// step 1: write a complete PDF document

	doc, err := document.WriteSinglePage(buf, document.A5r, pdf.V2_0, nil)
	if err != nil {
		t.Fatal(err)
	}

	F := fonttypes.Type1WithMetrics()

	fontRefObj, err := doc.RM.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	fontRef := fontRefObj.(pdf.Reference)

	doc.TextSetFont(F, 12.0)
	doc.TextBegin()
	doc.TextFirstLine(50, 200)
	doc.TextShow(testText)
	doc.TextEnd()

	err = doc.Close()
	if err != nil {
		t.Fatal(err)
	}

	// step 2: read the PDF document, find the font dict, and check
	r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()), nil)
	if err != nil {
		t.Fatal(err)
	}

	fontDict, err := pdf.NewCursor(r).DictTyped(fontRef, "Font")
	if err != nil {
		t.Fatal(err)
	}

	_, err = pdf.NewCursor(r).Name(fontDict["Encoding"])
	if err != nil {
		t.Fatal(err)
	}

	if _, present := fontDict["ToUnicode"]; present {
		t.Error("unexpected ToUnicode entry in font dictionary")
	}

	// step 3: extract the font file and check
	x := pdf.NewExtractor(r)
	parsedDict, err := extract.Dict(pdf.CursorAt(x, nil), fontDict, false)
	if err != nil {
		t.Fatal(err)
	}
	fontInfo, ok := parsedDict.FontInfo().(*dict.FontInfoSimple)
	if !ok {
		t.Fatal("expected FontInfoSimple type")
	}

	if fontInfo.FontFile == nil {
		t.Fatal("expected embedded font file")
	}
	if fontInfo.FontFile.Type != glyphdata.Type1 {
		t.Fatal("expected Type1 font")
	}

	// Extract font data using FromStream helper
	t1Font, err := type1glyphs.FromStream(fontInfo.FontFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(t1Font.Glyphs) != uniqueChars+1 { // +1 for the .notdef glyph
		t.Errorf("expected %d unique glyphs, got %d", uniqueChars+1, len(t1Font.Glyphs))
	}

	// step 4: extract the text from the PDF document and check
	_, pageDict, err := pagetree.GetPage(r, 0)
	if err != nil {
		t.Fatal(err)
	}
	pg, err := pdf.CursorAt(x, nil).Decode(pageDict, page.Decode)
	if err != nil {
		t.Fatal(err)
	}
	rd := reader.New(x)
	allText := ""
	rd.Character = func(c font.Code) error {
		allText += c.Text
		return nil
	}
	err = rd.ProcessPage(pg)
	if err != nil {
		t.Fatal(err)
	}

	if allText != testText {
		t.Errorf("expected text 'Hello', got %q", allText)
	}

	err = r.Close()
	if err != nil {
		t.Error(err)
	}
}

// TestType1BuiltinText checks that the text of codes left to the built-in
// encoding is found for a standard font which is not embedded, and only there.
func TestType1BuiltinText(t *testing.T) {
	builtinAt := func(code byte) encoding.Simple {
		return func(c byte) string {
			if c == code {
				return encoding.UseBuiltin
			}
			return ""
		}
	}
	cases := []struct {
		name     string
		psName   string
		embedded bool
		want     string
	}{
		{"standard", "Symbol", false, "α"},
		{"embedded", "Symbol", true, ""},
		{"not standard", "Foo", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := &dict.Type1{
				PostScriptName: tc.psName,
				Descriptor:     &font.Descriptor{IsSymbolic: true},
				Encoding:       builtinAt(0x61),
			}
			d.Width[0x61] = 631
			if tc.embedded {
				d.FontFile = &glyphdata.Stream{Type: glyphdata.Type1}
			}

			for code := range d.MakeFont().Codes(pdf.String("a")) {
				if code.Text != tc.want {
					t.Errorf("got text %q, want %q", code.Text, tc.want)
				}
			}

			w, ok := d.GlyphWidth("α")
			if ok != (tc.want != "") || (ok && w != 0.631) {
				t.Errorf("GlyphWidth: got %g, %t", w, ok)
			}
		})
	}
}
