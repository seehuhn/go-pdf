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
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/extract"
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

// A converted font holds only the glyphs the page shows, so it is a subset and
// its name must say so.  The conversion sees only the source font program,
// which is usually a subset already, so keeping all of its glyphs is not
// evidence that the output is the complete typeface.
func TestConvertedGlyfKeepsSubsetTag(t *testing.T) {
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
	srcTag := glyfSubsetTag(t, r)
	if srcTag == "" {
		t.Fatal("the source font is not a subset, so there is nothing to carry over")
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

	if got := glyfSubsetTag(t, rr); got == "" {
		t.Error("the converted font claims to be the complete font")
	}
}

// glyfSubsetTag returns the subset tag of the only font on the first page.
func glyfSubsetTag(t *testing.T, r pdf.Getter) string {
	t.Helper()

	switch d := firstPageFontDict(t, r).(type) {
	case *dict.TrueType:
		return d.SubsetTag
	case *dict.CIDFontType2:
		return d.SubsetTag
	default:
		t.Fatalf("unexpected font dictionary type %T", d)
		return ""
	}
}

// firstPageFontDict returns the dictionary of the only font on the first page.
func firstPageFontDict(t *testing.T, r pdf.Getter) dict.Dict {
	t.Helper()

	cur := pdf.NewCursor(r)
	_, page, err := pagetree.GetPage(r, 0)
	if err != nil {
		t.Fatal(err)
	}
	resDict, _ := cur.Dict(page["Resources"])
	fonts, _ := cur.Dict(resDict["Font"])
	if len(fonts) != 1 {
		t.Fatalf("want 1 font, got %d", len(fonts))
	}
	for _, ref := range fonts {
		d, err := extract.Dict(pdf.CursorAt(pdf.NewExtractor(r), nil), ref, false)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	panic("unreachable")
}

// CJK fonts are commonly named in UTF-8.  The converted font carries a program
// which names itself after the /BaseFont entry, so such a name has to reach the
// font program intact: a converter which cannot write the name drops the font
// and the page loses the text it drew.
func TestConvertedGlyfKeepsNonASCIIName(t *testing.T) {
	buf := memfile.New()
	doc, err := document.WriteMultiPage(buf, document.A4, pdf.V1_7,
		&pdf.WriterOptions{HumanReadable: true})
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

	// Rename the font in the finished file.  The replacement is the same
	// length as the original, so the cross-reference table still fits, and a
	// PDF name may hold these bytes even though a PostScript name may not.
	oldName := []byte("GoRegular")
	newName := []byte("宋体体")
	if len(newName) != len(oldName) {
		t.Fatalf("the replacement name is %d bytes, want %d", len(newName), len(oldName))
	}
	if got := bytes.Count(buf.Data, oldName); got != 2 {
		t.Fatalf("the font is named %d times in the file, want 2", got)
	}
	buf.Data = bytes.ReplaceAll(buf.Data, oldName, newName)

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

	// the font survived the conversion, so the page still draws its text,
	// and it is still the font the source named
	glyfSubsetTag(t, rr)
	if got := glyfPostScriptName(t, rr); got != string(newName) {
		t.Errorf("the converted font is named %q, want %q", got, newName)
	}
}

// glyfPostScriptName returns the PostScript name of the only font on the first
// page.
func glyfPostScriptName(t *testing.T, r pdf.Getter) string {
	t.Helper()

	switch d := firstPageFontDict(t, r).(type) {
	case *dict.TrueType:
		return d.PostScriptName
	case *dict.CIDFontType2:
		return d.PostScriptName
	default:
		t.Fatalf("unexpected font dictionary type %T", d)
		return ""
	}
}
