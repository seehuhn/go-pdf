package converter

import (
	"bytes"
	"fmt"
	"iter"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/reader"
)

type mockGetter struct {
	pdf.Getter
	failRef  pdf.Reference
	pagesRef pdf.Reference
}

func (m *mockGetter) Get(ref pdf.Reference, resolve bool) (pdf.Native, error) {
	if ref == m.failRef {
		return nil, fmt.Errorf("mock error at ref %v", ref)
	}

	switch ref {
	case 100: // Root Pages with 2 pages
		return pdf.Dict{
			"Type":  pdf.Name("Pages"),
			"Count": pdf.Integer(2),
			"Kids":  pdf.Array{pdf.Reference(101), pdf.Reference(102)},
		}, nil
	case 101: // Normal Page
		return pdf.Dict{
			"Type":     pdf.Name("Page"),
			"MediaBox": pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(612), pdf.Integer(792)},
		}, nil
	case 102: // Page 2 (MediaBox component fails GetNumber)
		return pdf.Dict{
			"Type":     pdf.Name("Page"),
			"MediaBox": pdf.Array{pdf.Boolean(true), pdf.Integer(0), pdf.Integer(612), pdf.Integer(792)},
		}, nil
	case 20: // Page with malformed Contents
		return pdf.Dict{
			"Type":     pdf.Name("Page"),
			"MediaBox": pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(612), pdf.Integer(792)},
			"Contents": pdf.Reference(21),
		}, nil
	}
	return nil, fmt.Errorf("unknown ref %v", ref)
}

func (m *mockGetter) GetMeta() *pdf.MetaInfo {
	return &pdf.MetaInfo{
		Version: pdf.V1_7,
		Catalog: &pdf.Catalog{
			Pages: m.pagesRef,
		},
	}
}

type mockFont struct {
	name string
}

func (m *mockFont) PostScriptName() string                       { return m.name }
func (m *mockFont) WritingMode() font.WritingMode                { return font.Horizontal }
func (m *mockFont) Codec() *charcode.Codec                       { return nil }
func (m *mockFont) Codes(s pdf.String) iter.Seq[*font.Code]      { return nil }
func (m *mockFont) FontInfo() any                                { return nil }
func (m *mockFont) AsPDF(opt pdf.OutputOptions) pdf.Native       { return nil }
func (m *mockFont) Embed(e *pdf.EmbedHelper) (pdf.Native, error) { return nil, nil }

type errorWriter struct {
	failAtByte int
	written    int
}

func (e *errorWriter) Write(p []byte) (n int, err error) {
	for i := range p {
		if e.written >= e.failAtByte {
			return i, fmt.Errorf("write error at byte %d", e.written)
		}
		e.written++
	}
	return len(p), nil
}

func TestEdgeCases(t *testing.T) {
	// 1. SetupCallbacks with nil currentPage
	c := NewConverter(nil)
	if c.Reader != nil && c.Reader.Character != nil {
		c.Reader.Character(0, "test", 10)
	}

	// 2. ConvertPage errors
	_, err := c.ConvertPage(1, pdf.Name("not-a-dict"))
	if err == nil {
		t.Error("expected error for non-dict page object")
	}

	badPage1 := pdf.Dict{
		"Type":     pdf.Name("Page"),
		"MediaBox": pdf.Name("not-an-array"),
	}
	_, err = c.ConvertPage(1, badPage1)
	if err == nil {
		t.Error("expected error for malformed MediaBox")
	}

	// 3. Coalesce early returns
	p0 := &Page{Fragments: nil}
	p0.Coalesce()
	p1 := &Page{Fragments: []*Fragment{{XMin: 0}}}
	p1.Coalesce()

	// 4. Sort branches
	p2 := &Page{
		Fragments: []*Fragment{
			{XMin: 20, YMin: 10, YMax: 20},
			{XMin: 10, YMin: 10, YMax: 20},
			{XMin: 10, YMin: 10, YMax: 20}, // identical
			{XMin: 5, YMin: 50, YMax: 60},
		},
	}
	p2.Sort()

	p4 := &Page{
		Fragments: []*Fragment{
			{XMin: 10, YMin: 50, YMax: 60},
			{XMin: 10, YMin: 10, YMax: 20},
		},
	}
	p4.Sort()

	// 5. ConvertDocument errors
	r1 := reader.New(&mockGetter{failRef: 100, pagesRef: 100}, nil)
	c1 := NewConverter(nil)
	c1.Reader = r1
	_, err = c1.ConvertDocument()
	if err == nil {
		t.Error("expected error in NumPages")
	}

	r2 := reader.New(&mockGetter{failRef: 102, pagesRef: 100}, nil)
	c2 := NewConverter(nil)
	c2.Reader = r2
	_, err = c2.ConvertDocument()
	if err == nil {
		t.Error("expected error in GetPage (second page)")
	}

	// MediaBox error branches
	for i := 0; i < 4; i++ {
		mb := pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(0), pdf.Integer(0)}
		mb[i] = pdf.Boolean(true)
		mg := &mockGetter{}
		r := reader.New(mg, nil)
		c_mb := NewConverter(nil)
		c_mb.Reader = r
		pageDict := pdf.Dict{"Type": pdf.Name("Page"), "MediaBox": mb}
		_, err = c_mb.ConvertPage(1, pageDict)
		if err == nil {
			t.Errorf("expected error for malformed MediaBox component %d", i)
		}
	}

	// ParsePage error branch
	mg_pp := &mockGetter{failRef: 21}
	pageDict20, _ := mg_pp.Get(pdf.Reference(20), true)
	r_pp := reader.New(mg_pp, nil)
	c_pp := NewConverter(nil)
	c_pp.Reader = r_pp
	_, err = c_pp.ConvertPage(1, pageDict20)
	if err == nil {
		t.Error("expected error in ParsePage")
	}

	// ConvertPage error during ConvertDocument (hits line 156)
	r_cd := reader.New(&mockGetter{pagesRef: 100}, nil)
	c_cd := NewConverter(nil)
	c_cd.Reader = r_cd
	_, err = c_cd.ConvertDocument()
	if err == nil {
		t.Error("expected error in ConvertDocument from second page failure")
	}

	// Empty MediaBox
	badPage3 := pdf.Dict{
		"Type":     pdf.Name("Page"),
		"MediaBox": pdf.Array{},
	}
	c_empty := NewConverter(nil)
	_, err = c_empty.ConvertPage(1, badPage3)
	if err == nil {
		t.Error("expected error for empty MediaBox")
	}
}

func TestWriteCSSErrors(t *testing.T) {
	ft := &FontTracker{}
	// Add diverse fonts to hit style branches
	fonts := []string{"Arial-Bold", "Arial-Italic", "Arial-Oblique", "Times-BoldItalic"}
	for _, f := range fonts {
		ft.Fonts = append(ft.Fonts, &mockFont{name: f})
		ft.Sizes = append(ft.Sizes, 12.0)
	}

	// Loop over byte offsets to hit every possible failure point
	for i := 0; i < 1000; i++ {
		ew := &errorWriter{failAtByte: i}
		ft.WriteCSS(ew)
	}
}

func TestFontTrackerSpecial(t *testing.T) {
	ft := &FontTracker{}
	ft.Fonts = append(ft.Fonts, &mockFont{name: "Arial-BoldItalic"})
	ft.Sizes = append(ft.Sizes, 12.0)

	var buf bytes.Buffer
	ft.WriteCSS(&buf)
	css := buf.String()
	if !strings.Contains(css, "font-weight: bold;") {
		t.Error("CSS missing bold style")
	}
	if !strings.Contains(css, "font-style: italic;") {
		t.Error("CSS missing italic style")
	}
}

func TestFontTrackerOblique(t *testing.T) {
	ft := &FontTracker{}
	ft.Fonts = append(ft.Fonts, &mockFont{name: "MyFont-Oblique"})
	ft.Sizes = append(ft.Sizes, 14.0)

	var buf bytes.Buffer
	ft.WriteCSS(&buf)
	css := buf.String()
	if !strings.Contains(css, "font-style: italic;") {
		t.Error("CSS missing italic (oblique) style")
	}
}
