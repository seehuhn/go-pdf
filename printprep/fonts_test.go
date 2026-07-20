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

package printprep

import (
	"bytes"
	"io"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/pagetree"
)

func TestGlyfToIdentityH(t *testing.T) {
	// source: a page showing text in the (TrueType/glyf) Go font
	buf := memfile.New()
	doc, err := document.WriteMultiPage(buf, document.A4, pdf.V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}
	F, err := gofont.Regular.NewSimple(nil)
	if err != nil {
		t.Fatal(err)
	}
	p := doc.AddPage()
	p.TextBegin()
	p.TextSetFont(F, 12)
	p.TextFirstLine(72, 700)
	p.TextShow("Hello")
	p.TextEnd()
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if err := doc.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := pdf.NewReader(buf, int64(len(buf.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := Write(&out, r, nil); err != nil {
		t.Fatal(err)
	}
	res := out.Bytes()
	rr, err := pdf.NewReader(bytes.NewReader(res), int64(len(res)), nil)
	if err != nil {
		t.Fatal(err)
	}
	cur := pdf.NewCursor(rr)

	// the converted font must be a composite Type0 font, Identity-H encoded,
	// with a CIDFontType2 descendant
	_, page, err := pagetree.GetPage(rr, 0)
	if err != nil {
		t.Fatal(err)
	}
	resDict, _ := cur.Dict(page["Resources"])
	fonts, _ := cur.Dict(resDict["Font"])
	if len(fonts) != 1 {
		t.Fatalf("want 1 font, got %d", len(fonts))
	}
	var fontDict pdf.Dict
	for _, ref := range fonts {
		fontDict, _ = cur.Dict(ref)
	}
	if got := fontDict["Subtype"]; got != pdf.Name("Type0") {
		t.Errorf("font Subtype = %v, want Type0", got)
	}
	if got := fontDict["Encoding"]; got != pdf.Name("Identity-H") {
		t.Errorf("font Encoding = %v, want Identity-H", got)
	}
	descs, _ := cur.Array(fontDict["DescendantFonts"])
	if len(descs) != 1 {
		t.Fatalf("want 1 descendant font, got %d", len(descs))
	}
	desc, _ := cur.Dict(descs[0])
	if got := desc["Subtype"]; got != pdf.Name("CIDFontType2") {
		t.Errorf("descendant Subtype = %v, want CIDFontType2", got)
	}

	// the shown text must be re-encoded to two-byte, non-notdef codes
	codes := firstShownString(t, rr, page["Contents"])
	if len(codes) != 2*len("Hello") {
		t.Errorf("re-encoded string is %d bytes, want %d", len(codes), 2*len("Hello"))
	}
	for i := 0; i+1 < len(codes); i += 2 {
		if codes[i] == 0 && codes[i+1] == 0 {
			t.Errorf("glyph %d re-encoded to .notdef", i/2)
		}
	}
}

// TestLowVersionGlyfPreserved checks that a source below PDF 1.3 does not lose
// its text: converting an embedded glyf font to a composite CIDFontType2 needs
// PDF 1.3, so the output version is floored there and the font survives rather
// than being silently dropped.
func TestLowVersionGlyfPreserved(t *testing.T) {
	// source: a PDF 1.2 page showing text in the (TrueType/glyf) Go font
	buf := memfile.New()
	doc, err := document.WriteMultiPage(buf, document.A4, pdf.V1_2, nil)
	if err != nil {
		t.Fatal(err)
	}
	F, err := gofont.Regular.NewSimple(nil)
	if err != nil {
		t.Fatal(err)
	}
	p := doc.AddPage()
	p.TextBegin()
	p.TextSetFont(F, 12)
	p.TextFirstLine(72, 700)
	p.TextShow("Hello")
	p.TextEnd()
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if err := doc.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := pdf.NewReader(buf, int64(len(buf.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := Write(&out, r, nil); err != nil {
		t.Fatal(err)
	}
	res := out.Bytes()
	rr, err := pdf.NewReader(bytes.NewReader(res), int64(len(res)), nil)
	if err != nil {
		t.Fatal(err)
	}

	// the output version was raised to at least 1.3
	if v := rr.GetMeta().Version; v < pdf.V1_3 {
		t.Errorf("output version = %v, want >= 1.3", v)
	}

	// the converted font survived: a Type0 font is present in the page resources
	_, page, err := pagetree.GetPage(rr, 0)
	if err != nil {
		t.Fatal(err)
	}
	cur := pdf.NewCursor(rr)
	resDict, _ := cur.Dict(page["Resources"])
	fonts, _ := cur.Dict(resDict["Font"])
	if len(fonts) != 1 {
		t.Fatalf("want 1 font (text preserved), got %d", len(fonts))
	}
	for _, ref := range fonts {
		fontDict, _ := cur.Dict(ref)
		if got := fontDict["Subtype"]; got != pdf.Name("Type0") {
			t.Errorf("font Subtype = %v, want Type0", got)
		}
	}
}

// firstShownString returns the operand of the first text-showing operator in
// the given content stream.
func firstShownString(t *testing.T, r pdf.Getter, contents pdf.Object) []byte {
	t.Helper()
	rc, err := pdf.NewCursor(r).StreamReader(contents)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatal(err)
	}
	open := func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil }
	for name, args := range content.NewScanner(open).NewIter().All() {
		switch name {
		case content.OpTextShow, content.OpTextShowMoveNextLine:
			if len(args) >= 1 {
				if s, ok := args[len(args)-1].(pdf.String); ok {
					return []byte(s)
				}
			}
		case content.OpTextShowArray:
			if len(args) == 1 {
				if arr, ok := args[0].(pdf.Array); ok {
					var b []byte
					for _, el := range arr {
						if s, ok := el.(pdf.String); ok {
							b = append(b, []byte(s)...)
						}
					}
					return b
				}
			}
		}
	}
	t.Fatal("no text-showing operator found")
	return nil
}
