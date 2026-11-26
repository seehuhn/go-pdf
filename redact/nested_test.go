// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"slices"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// A nestPage describes a page of a document built by [nestDoc].
type nestPage struct {
	content string
	res     pdf.Dict
	annots  []pdf.Object

	// resObj, if it is not nil, is used as /Resources in place of res
	resObj pdf.Object

	// extra holds additional entries of the page dictionary
	extra pdf.Dict
}

// A nestBuilder writes the shared objects of a document.
type nestBuilder struct {
	t  testing.TB
	w  *pdf.Writer
	rm *pdf.ResourceManager
}

func (b *nestBuilder) put(obj pdf.Object) pdf.Reference {
	b.t.Helper()
	ref := b.w.Alloc()
	if err := b.w.Put(ref, obj); err != nil {
		b.t.Fatal(err)
	}
	return ref
}

func (b *nestBuilder) stream(dict pdf.Dict, data string) pdf.Reference {
	b.t.Helper()
	ref := b.w.Alloc()
	stm, err := b.w.OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		b.t.Fatal(err)
	}
	if _, err := stm.Write([]byte(data)); err != nil {
		b.t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		b.t.Fatal(err)
	}
	return ref
}

// form writes a form XObject.  Entries of dict override the defaults.
func (b *nestBuilder) form(dict pdf.Dict, data string) pdf.Reference {
	d := pdf.Dict{
		"Type": pdf.Name("XObject"), "Subtype": pdf.Name("Form"),
		"BBox": rect(0, 0, 300, 300),
	}
	for k, v := range dict {
		if v == nil {
			delete(d, k)
			continue
		}
		d[k] = v
	}
	return b.stream(d, data)
}

// helv is a font resource dictionary which a form can use.
func helv() pdf.Dict {
	return pdf.Dict{"Font": pdf.Dict{"F1": pdf.Dict{
		"Type": pdf.Name("Font"), "Subtype": pdf.Name("Type1"),
		"BaseFont": pdf.Name("Helvetica"),
	}}}
}

// nestDoc builds a document with 300x300 pages.  The redaction annotations
// regs apply to the first page.
func nestDoc(t testing.TB, regs []pdf.Dict, setup func(b *nestBuilder) []nestPage) *pdf.Reader {
	t.Helper()
	w, buf := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	return nestDocWith(t, w, buf, regs, setup)
}

// nestDocUnchecked is like [nestDoc], but the writer does not check the
// generated file for unrounded numbers.  Tests use it for documents which
// need numbers beyond the precision of [pdf.Round].
func nestDocUnchecked(t testing.TB, regs []pdf.Dict, setup func(b *nestBuilder) []nestPage) *pdf.Reader {
	t.Helper()
	buf := memfile.New()
	w, err := pdf.NewWriter(buf, pdf.V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}
	return nestDocWith(t, w, buf, regs, setup)
}

func nestDocWith(t testing.TB, w *pdf.Writer, buf *memfile.MemFile, regs []pdf.Dict, setup func(b *nestBuilder) []nestPage) *pdf.Reader {
	t.Helper()
	b := &nestBuilder{t: t, w: w, rm: pdf.NewResourceManager(w)}
	pages := setup(b)

	pagesRef := w.Alloc()
	var kids pdf.Array
	for i, pg := range pages {
		pageRef := w.Alloc()
		kids = append(kids, pageRef)
		cref := b.stream(pdf.Dict{}, pg.content)

		annots := slices.Clone(pg.annots)
		if i == 0 {
			for _, a := range regs {
				a["Type"] = pdf.Name("Annot")
				a["Subtype"] = pdf.Name("Redact")
				a["P"] = pageRef
				annots = append(annots, b.put(a))
			}
		}
		d := pdf.Dict{
			"Type": pdf.Name("Page"), "Parent": pagesRef,
			"MediaBox":  rect(0, 0, 300, 300),
			"Resources": pg.res,
			"Contents":  cref,
		}
		if pg.resObj != nil {
			d["Resources"] = pg.resObj
		}
		maps.Copy(d, pg.extra)
		if len(annots) > 0 {
			d["Annots"] = pdf.Array(annots)
		}
		if err := w.Put(pageRef, d); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Put(pagesRef, pdf.Dict{
		"Type": pdf.Name("Pages"), "Kids": kids, "Count": pdf.Integer(len(pages)),
	}); err != nil {
		t.Fatal(err)
	}
	w.GetMeta().Catalog.Pages = pagesRef
	if err := b.rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	r := openSource(t, buf.Data)
	return r
}

// streamsContaining returns the number of streams of the document which
// contain text.
func streamsContaining(t testing.TB, r *pdf.Reader, text string) int {
	t.Helper()
	n := 0
	for _, data := range fileStreams(t, r) {
		if bytes.Contains(data, []byte(text)) {
			n++
		}
	}
	return n
}

// checkNested redacts the first occurrence of the text secret from the
// first page of the document, which build makes for the given redaction
// annotations.  It checks that the text is gone from all streams of the
// output and that all other glyphs keep their position.
func checkNested(t testing.TB, build func(regs []pdf.Dict) *pdf.Reader, secret string) *pdf.Reader {
	t.Helper()
	in := build(nil)
	before := pageGlyphs(t, in)
	pos := strings.Index(glyphString(before), secret)
	if pos < 0 {
		t.Fatalf("test page does not show %q (got %q)", secret, glyphString(before))
	}
	from, to := pos, pos+len(secret)

	in = build([]pdf.Dict{regionOver(before[from:to])})
	if streamsContaining(t, in, secret) == 0 {
		t.Fatalf("no stream of the input holds %q", secret)
	}
	out := writeAndRead(t, in, nil, "")
	after := pageGlyphs(t, out)

	if strings.Contains(glyphString(after), secret) {
		t.Errorf("extracted text still contains %q", secret)
	}
	if n := streamsContaining(t, out, secret); n > 0 {
		t.Errorf("%d streams of the output still hold %q", n, secret)
	}

	want := append(append([]glyph{}, before[:from]...), before[to:]...)
	if len(after) != len(want) {
		t.Fatalf("got text %q, want %q", glyphString(after), glyphString(want))
	}
	for i, g := range after {
		w := want[i]
		if g.text != w.text || math.Abs(g.x-w.x) > 1e-3 || math.Abs(g.y-w.y) > 1e-3 {
			t.Errorf("glyph %d: got %q at (%g, %g), want %q at (%g, %g)",
				i, g.text, g.x, g.y, w.text, w.x, w.y)
		}
	}
	return out
}

const secretText = "BT /F1 12 Tf 0 10 Td (Hello Secret World) Tj ET"

// singleForm builds a page which paints one form with the content body.
func singleForm(t testing.TB, pageContent string, formDict pdf.Dict, body string) func(regs []pdf.Dict) *pdf.Reader {
	return func(regs []pdf.Dict) *pdf.Reader {
		return nestDoc(t, regs, func(b *nestBuilder) []nestPage {
			d := pdf.Dict{"BBox": rect(0, 0, 250, 50), "Resources": helv()}
			maps.Copy(d, formDict)
			if v, ok := d["Resources"]; ok && v == nil {
				delete(d, "Resources")
			}
			fm := b.form(d, body)
			return []nestPage{{
				content: pageContent,
				res:     pdf.Dict{"XObject": pdf.Dict{"Fm1": fm}, "Font": helv()["Font"]},
			}}
		})
	}
}

func TestFormPartlyOverRegion(t *testing.T) {
	build := singleForm(t, "q 1 0 0 1 20 100 cm /Fm1 Do Q", nil, secretText)
	out := checkNested(t, build, "Secret")
	page := pageAt(t, out, 0)
	for _, n := range doNames(t, out, page) {
		if n == "Fm1" {
			t.Error("the page still paints the original form")
		}
	}
}

func TestFormMatrixAndCTM(t *testing.T) {
	cases := []struct {
		name, page string
		form       pdf.Dict
	}{
		{"scaled matrix", "q 1 0 0 1 20 100 cm /Fm1 Do Q",
			pdf.Dict{"Matrix": pdf.Array{pdf.Number(0.8), pdf.Integer(0), pdf.Integer(0), pdf.Number(1.2), pdf.Integer(5), pdf.Integer(3)}}},
		{"rotated matrix", "q 1 0 0 1 100 50 cm /Fm1 Do Q",
			pdf.Dict{"Matrix": pdf.Array{pdf.Number(0.8), pdf.Number(0.6), pdf.Number(-0.6), pdf.Number(0.8), pdf.Integer(0), pdf.Integer(0)}}},
		{"rotated CTM", "q 0.7 0.7 -0.7 0.7 150 20 cm /Fm1 Do Q", nil},
		{"both", "q 0 1 -1 0 250 20 cm 1.5 0 0 0.8 0 0 cm /Fm1 Do Q",
			pdf.Dict{"Matrix": pdf.Array{pdf.Number(0.8), pdf.Number(0.6), pdf.Number(-0.6), pdf.Number(0.8), pdf.Integer(1), pdf.Integer(2)}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			checkNested(t, singleForm(t, c.page, c.form, secretText), "Secret")
		})
	}
}

func TestFormGroup(t *testing.T) {
	d := pdf.Dict{"Group": pdf.Dict{"S": pdf.Name("Transparency"), "CS": pdf.Name("DeviceRGB")}}
	out := checkNested(t, singleForm(t, "q 1 0 0 1 20 100 cm /Fm1 Do Q", d, secretText), "Secret")
	var found bool
	for _, name := range doNames(t, out, pageAt(t, out, 0)) {
		cur := pdf.CursorAt(pdf.NewExtractor(out), nil)
		stm, err := cur.Stream(xobject(t, out, pageAt(t, out, 0), name))
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := stm.Dict["Group"]; ok {
			found = true
		}
	}
	if !found {
		t.Error("the rewritten form lost its /Group")
	}
}

func TestFormInheritsResources(t *testing.T) {
	build := singleForm(t, "q 1 0 0 1 20 100 cm /Fm1 Do Q", pdf.Dict{"Resources": nil}, secretText)
	out := checkNested(t, build, "Secret")

	// the rewritten form has explicit resources
	page := pageAt(t, out, 0)
	cur := pdf.CursorAt(pdf.NewExtractor(out), nil)
	found := false
	for _, name := range doNames(t, out, page) {
		stm, err := cur.Stream(xobject(t, out, page, name))
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := stm.Dict["Resources"]; ok {
			found = true
		}
	}
	if !found {
		t.Error("the rewritten form has no resources")
	}
}

func TestFormNested(t *testing.T) {
	build := func(regs []pdf.Dict) *pdf.Reader {
		return nestDoc(t, regs, func(b *nestBuilder) []nestPage {
			inner := b.form(pdf.Dict{
				"BBox": rect(0, 0, 250, 50), "Resources": helv(),
				"Matrix": pdf.Array{pdf.Integer(1), pdf.Integer(0), pdf.Integer(0), pdf.Integer(1), pdf.Integer(3), pdf.Integer(4)},
			}, secretText)
			mid := b.form(pdf.Dict{
				"BBox":      rect(0, 0, 280, 280),
				"Resources": pdf.Dict{"XObject": pdf.Dict{"In": inner}},
			}, "q 1.1 0 0 0.9 10 10 cm /In Do Q")
			outer := b.form(pdf.Dict{
				"BBox":      rect(0, 0, 300, 300),
				"Resources": pdf.Dict{"XObject": pdf.Dict{"Mid": mid}},
			}, "q 0.9 0 0 1 5 60 cm /Mid Do Q")
			return []nestPage{{
				content: "/Outer Do",
				res:     pdf.Dict{"XObject": pdf.Dict{"Outer": outer}},
			}}
		})
	}
	checkNested(t, build, "Secret")
}

func TestFormInsideRegionDropped(t *testing.T) {
	in := singleForm(t, "q 1 0 0 1 20 100 cm /Fm1 Do Q", nil, secretText)([]pdf.Dict{
		{"Rect": rect(0, 50, 300, 250)}})
	out := writeAndRead(t, in, nil, "")
	if n := streamsContaining(t, out, "Secret"); n != 0 {
		t.Errorf("%d streams still hold the text of the dropped form", n)
	}
	if got := pageGlyphs(t, out); len(got) != 0 {
		t.Errorf("page still shows %q", glyphString(got))
	}
}

func TestFormOutsideRegionKept(t *testing.T) {
	in := singleForm(t, "q 1 0 0 1 20 100 cm /Fm1 Do Q", nil, secretText)([]pdf.Dict{
		{"Rect": rect(10, 250, 40, 290)}})
	out := writeAndRead(t, in, nil, "")
	page := pageAt(t, out, 0)
	found := false
	for _, n := range doNames(t, out, page) {
		found = found || n == "Fm1"
	}
	if !found {
		t.Error("form outside the regions was changed")
	}
	if streamsContaining(t, out, "Hello Secret World") != 1 {
		t.Error("form outside the regions was rewritten")
	}
}

func TestFormSharedBetweenPages(t *testing.T) {
	build := func(regs []pdf.Dict) *pdf.Reader {
		return nestDoc(t, regs, func(b *nestBuilder) []nestPage {
			fm := b.form(pdf.Dict{"BBox": rect(0, 0, 250, 50), "Resources": helv()}, secretText)
			pg := nestPage{
				content: "q 1 0 0 1 20 100 cm /Fm1 Do Q",
				res:     pdf.Dict{"XObject": pdf.Dict{"Fm1": fm}},
			}
			return []nestPage{pg, pg}
		})
	}
	in := build(nil)
	before := pageGlyphs(t, in)
	pos := strings.Index(glyphString(before), "Secret")
	out := writeAndRead(t, build([]pdf.Dict{regionOver(before[pos : pos+6])}), nil, "")

	page0, page1 := pageAt(t, out, 0), pageAt(t, out, 1)
	for _, n := range doNames(t, out, page1) {
		if n != "Fm1" {
			t.Errorf("second page paints %q", n)
		}
	}
	if n := streamsContaining(t, out, "Hello Secret World"); n != 1 {
		t.Errorf("got %d copies of the original text, want 1", n)
	}
	// the redacted page does not paint the original
	for _, n := range doNames(t, out, page0) {
		if xobject(t, out, page0, n) == xobject(t, out, page1, "Fm1") {
			t.Errorf("redacted page paints the original form")
		}
	}
}

func TestFormSamePlacementOneCopy(t *testing.T) {
	build := singleForm(t, "q 1 0 0 1 20 100 cm /Fm1 Do Q q 1 0 0 1 20 100 cm /Fm1 Do Q", nil, secretText)
	in := build(nil)
	before := pageGlyphs(t, in)
	pos := strings.Index(glyphString(before), "Secret")
	out := writeAndRead(t, build([]pdf.Dict{regionOver(before[pos : pos+6])}), nil, "")
	if n := streamsContaining(t, out, "Secret"); n != 0 {
		t.Errorf("%d streams still hold the text", n)
	}
	page := pageAt(t, out, 0)
	refs := map[pdf.Object]bool{}
	for _, n := range doNames(t, out, page) {
		refs[xobject(t, out, page, n)] = true
	}
	// one copy of the form and the mark
	if len(refs) != 2 {
		t.Errorf("page paints %d different XObjects, want 2", len(refs))
	}
}

func TestFormTwoPlacementsTwoCopies(t *testing.T) {
	build := singleForm(t, "q 1 0 0 1 20 100 cm /Fm1 Do Q q 1 0 0 1 20 200 cm /Fm1 Do Q", nil, secretText)
	in := build(nil)
	before := pageGlyphs(t, in)
	pos := strings.Index(glyphString(before), "Secret")
	out := writeAndRead(t, build([]pdf.Dict{regionOver(before[pos : pos+6])}), nil, "")
	after := pageGlyphs(t, out)
	if got := strings.Count(glyphString(after), "Secret"); got != 1 {
		t.Errorf("got %d copies of the text, want the one outside the region", got)
	}
	page := pageAt(t, out, 0)
	var forms []pdf.Name
	for _, n := range doNames(t, out, page) {
		if n == "Fm1" {
			forms = append(forms, n)
		}
	}
	if len(forms) != 1 {
		t.Errorf("page paints the original form %d times, want once", len(forms))
	}
}

func TestFormAnnotationGetsCopy(t *testing.T) {
	// a kept annotation which shows the form meets no region, and gets an
	// unchanged copy of the form, while the page paints a redacted copy
	build := func(regs []pdf.Dict) *pdf.Reader {
		return nestDoc(t, regs, func(b *nestBuilder) []nestPage {
			fm := b.form(pdf.Dict{"BBox": rect(0, 0, 250, 50), "Resources": helv()}, secretText)
			ap := b.put(pdf.Dict{
				"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Stamp"),
				"Rect": rect(10, 260, 60, 290),
				"AP":   pdf.Dict{"N": fm},
			})
			return []nestPage{{
				content: "q 1 0 0 1 20 100 cm /Fm1 Do Q",
				res:     pdf.Dict{"XObject": pdf.Dict{"Fm1": fm}},
				annots:  []pdf.Object{ap},
			}}
		})
	}
	in := build(nil)
	before := pageGlyphs(t, in)
	pos := strings.Index(glyphString(before), "Secret")
	out := writeAndRead(t, build([]pdf.Dict{regionOver(before[pos : pos+6])}), nil, "")
	if strings.Contains(glyphString(pageGlyphs(t, out)), "Secret") {
		t.Error("page content still shows the text")
	}
	annot := pageAnnot(t, out, func(d pdf.Dict) bool { return d["Subtype"] == pdf.Name("Stamp") })
	stm, err := pdf.CursorAt(pdf.NewExtractor(out), nil).Stream(path(t, out, annot, "AP", "N"))
	if err != nil || stm == nil {
		t.Fatalf("annotation has no normal appearance (%v)", err)
	}
	data, err := pdf.ReadAll(out, nil, stm, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("Secret")) {
		t.Error("appearance lost the text")
	}
}

func TestSoftMaskGroup(t *testing.T) {
	build := func(regs []pdf.Dict) *pdf.Reader {
		return nestDoc(t, regs, func(b *nestBuilder) []nestPage {
			g := b.form(pdf.Dict{
				"BBox": rect(0, 0, 250, 50), "Resources": helv(),
				"Group": pdf.Dict{"S": pdf.Name("Transparency"), "CS": pdf.Name("DeviceGray")},
			}, "1 g "+secretText)
			return []nestPage{{
				content: "q 1 0 0 1 20 100 cm /GS1 gs 0 0 200 100 re f Q",
				res: pdf.Dict{"ExtGState": pdf.Dict{"GS1": pdf.Dict{
					"Type":  pdf.Name("ExtGState"),
					"SMask": pdf.Dict{"S": pdf.Name("Luminosity"), "G": g},
				}}},
			}}
		})
	}
	in := build(nil)
	// the region is where the soft-mask text lies: x from 20, y from 110
	region := pdf.Dict{"Rect": rect(60, 108, 100, 130)}
	_ = in
	out := writeAndRead(t, build([]pdf.Dict{region}), nil, "")
	if n := streamsContaining(t, out, "Secret"); n != 0 {
		t.Errorf("%d streams still hold the soft-mask text", n)
	}
	if n := streamsContaining(t, out, "Hello"); n == 0 {
		t.Error("the soft-mask group lost the text outside the region")
	}

	// the gs operator uses an extended graphics state with the copy
	page := pageAt(t, out, 0)
	cur := pdf.CursorAt(pdf.NewExtractor(out), nil)
	res, err := cur.Dict(page["Resources"])
	if err != nil {
		t.Fatal(err)
	}
	states, err := cur.Dict(res["ExtGState"])
	if err != nil {
		t.Fatal(err)
	}
	var used int
	for _, op := range pageOps(t, out, page) {
		if op.name != content.OpSetExtGState {
			continue
		}
		d, err := cur.Dict(states[op.args[0].(pdf.Name)])
		if err != nil {
			t.Fatal(err)
		}
		sm, err := cur.Dict(d["SMask"])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := cur.Stream(sm["G"]); err != nil {
			t.Fatal(err)
		}
		used++
	}
	if used != 1 {
		t.Errorf("page sets %d extended graphics states, want 1", used)
	}
}

func TestSoftMaskGroupOutside(t *testing.T) {
	build := func(regs []pdf.Dict) *pdf.Reader {
		return nestDoc(t, regs, func(b *nestBuilder) []nestPage {
			g := b.form(pdf.Dict{"BBox": rect(0, 0, 250, 50), "Resources": helv()}, secretText)
			return []nestPage{{
				content: "q 1 0 0 1 20 100 cm /GS1 gs 0 0 200 100 re f Q",
				res: pdf.Dict{"ExtGState": pdf.Dict{"GS1": pdf.Dict{
					"SMask": pdf.Dict{"S": pdf.Name("Luminosity"), "G": g},
				}}},
			}}
		})
	}
	out := writeAndRead(t, build([]pdf.Dict{{"Rect": rect(10, 250, 40, 290)}}), nil, "")
	if n := streamsContaining(t, out, "Hello Secret World"); n != 1 {
		t.Errorf("soft-mask group outside the regions was rewritten")
	}
}

// patternDoc builds a page which fills a rectangle with a tiling pattern
// whose cells show the text "Secret".
func patternDoc(t testing.TB, regs []pdf.Dict, fillRect string, patDict pdf.Dict) *pdf.Reader {
	return nestDoc(t, regs, func(b *nestBuilder) []nestPage {
		d := pdf.Dict{
			"Type": pdf.Name("Pattern"), "PatternType": pdf.Integer(1),
			"PaintType": pdf.Integer(1), "TilingType": pdf.Integer(1),
			"BBox":  rect(0, 0, 40, 20),
			"XStep": pdf.Integer(40), "YStep": pdf.Integer(20),
			"Resources": helv(),
		}
		maps.Copy(d, patDict)
		pat := b.stream(d, "BT /F1 12 Tf 1 4 Td (Secret) Tj ET")
		return []nestPage{{
			content: "/Pattern cs /P1 scn " + fillRect + " f",
			res:     pdf.Dict{"Pattern": pdf.Dict{"P1": pat}, "ColorSpace": pdf.Dict{}},
		}}
	})
}

func TestTilingPattern(t *testing.T) {
	const fill = "0 0 200 200 re"
	// the region is in the cell (1, 1), over the first glyph
	region := pdf.Dict{"Rect": rect(41, 25, 47, 31)}
	in := patternDoc(t, []pdf.Dict{region}, fill, nil)
	out := writeAndRead(t, in, nil, "")

	if n := streamsContaining(t, out, "Secret"); n != 0 {
		t.Errorf("%d streams still hold the pattern text", n)
	}
	if n := streamsContaining(t, out, "ret"); n == 0 {
		t.Error("the pattern lost the glyphs outside the region")
	}
	page := pageAt(t, out, 0)
	cur := pdf.CursorAt(pdf.NewExtractor(out), nil)
	res, err := cur.Dict(page["Resources"])
	if err != nil {
		t.Fatal(err)
	}
	pats, err := cur.Dict(res["Pattern"])
	if err != nil {
		t.Fatal(err)
	}
	if len(pats) != 1 {
		t.Errorf("page has %d patterns, want 1", len(pats))
	}
}

func TestTilingPatternRegionOutsideFill(t *testing.T) {
	// the region does not meet the filled rectangle
	region := pdf.Dict{"Rect": rect(250, 250, 290, 290)}
	in := patternDoc(t, []pdf.Dict{region}, "0 0 100 100 re", nil)
	out := writeAndRead(t, in, nil, "")
	if n := streamsContaining(t, out, "(Secret)"); n != 1 {
		t.Errorf("got %d streams with the pattern text, want 1", n)
	}
}

func TestTilingPatternManyCells(t *testing.T) {
	// a large region meets more cells than the pattern lattice is tested for
	region := pdf.Dict{"Rect": rect(30, 30, 190, 190)}
	in := patternDoc(t, []pdf.Dict{region}, "0 0 200 200 re", nil)
	out := writeAndRead(t, in, nil, "")
	if n := streamsContaining(t, out, "Secret"); n != 0 {
		t.Errorf("%d streams still hold the pattern text", n)
	}
}

func TestTilingPatternMatrix(t *testing.T) {
	m := pdf.Array{pdf.Number(0.5), pdf.Number(0.5), pdf.Number(-0.5), pdf.Number(0.5), pdf.Integer(10), pdf.Integer(20)}
	region := pdf.Dict{"Rect": rect(0, 0, 300, 300)}
	in := patternDoc(t, []pdf.Dict{region}, "0 0 200 200 re", pdf.Dict{"Matrix": m})
	out := writeAndRead(t, in, nil, "")
	if n := streamsContaining(t, out, "Secret"); n != 0 {
		t.Errorf("%d streams still hold the pattern text", n)
	}
}

func TestFormCycle(t *testing.T) {
	build := func(regs []pdf.Dict) *pdf.Reader {
		return nestDoc(t, regs, func(b *nestBuilder) []nestPage {
			a, bb := b.w.Alloc(), b.w.Alloc()
			for _, f := range []struct {
				ref  pdf.Reference
				data string
				res  pdf.Dict
			}{
				{a, secretText + " /B Do", pdf.Dict{
					"Font": helv()["Font"], "XObject": pdf.Dict{"B": bb}}},
				{bb, "/A Do", pdf.Dict{"XObject": pdf.Dict{"A": a}}},
			} {
				stm, err := b.w.OpenStream(f.ref, pdf.Dict{
					"Type": pdf.Name("XObject"), "Subtype": pdf.Name("Form"),
					"BBox": rect(0, 0, 250, 50), "Resources": f.res,
				}, pdf.FilterCompress{})
				if err != nil {
					t.Fatal(err)
				}
				stm.Write([]byte(f.data))
				if err := stm.Close(); err != nil {
					t.Fatal(err)
				}
			}
			return []nestPage{{
				content: "q 1 0 0 1 20 100 cm /A Do Q",
				res:     pdf.Dict{"XObject": pdf.Dict{"A": a}},
			}}
		})
	}
	in := build(nil)
	before := pageGlyphs(t, in)
	pos := strings.Index(glyphString(before), "Secret")
	out := writeAndRead(t, build([]pdf.Dict{regionOver(before[pos : pos+6])}), nil, "")
	if n := streamsContaining(t, out, "Secret"); n != 0 {
		t.Errorf("%d streams still hold the text", n)
	}
	checkStable(t, out)
}

func TestFormCycleOutside(t *testing.T) {
	// a cycle which does not meet a region is copied plainly
	build := func(regs []pdf.Dict) *pdf.Reader {
		return nestDoc(t, regs, func(b *nestBuilder) []nestPage {
			a, bb := b.w.Alloc(), b.w.Alloc()
			for _, f := range []struct {
				ref  pdf.Reference
				data string
				res  pdf.Dict
			}{
				{a, secretText + " /B Do", pdf.Dict{
					"Font": helv()["Font"], "XObject": pdf.Dict{"B": bb}}},
				{bb, "/A Do", pdf.Dict{"XObject": pdf.Dict{"A": a}}},
			} {
				stm, err := b.w.OpenStream(f.ref, pdf.Dict{
					"Type": pdf.Name("XObject"), "Subtype": pdf.Name("Form"),
					"BBox": rect(0, 0, 250, 50), "Resources": f.res,
				}, pdf.FilterCompress{})
				if err != nil {
					t.Fatal(err)
				}
				stm.Write([]byte(f.data))
				if err := stm.Close(); err != nil {
					t.Fatal(err)
				}
			}
			return []nestPage{{
				content: "q 1 0 0 1 20 100 cm /A Do Q",
				res:     pdf.Dict{"XObject": pdf.Dict{"A": a}},
			}}
		})
	}
	out := writeAndRead(t, build([]pdf.Dict{{"Rect": rect(250, 250, 290, 290)}}), nil, "")
	if n := streamsContaining(t, out, "Hello Secret World"); n != 1 {
		t.Errorf("got %d streams with the text, want 1", n)
	}
}

// fanDoc builds a document in which each of the given number of levels of
// forms places the next level fan times at slightly different places.  The
// region meets the text in the lowest level.
func fanDoc(t testing.TB, levels, fan int) *pdf.Reader {
	return nestDoc(t, []pdf.Dict{{"Rect": rect(30, 105, 60, 120)}}, func(b *nestBuilder) []nestPage {
		next := b.form(pdf.Dict{"BBox": rect(0, 0, 100, 50), "Resources": helv()}, secretText)
		for level := range levels {
			var sb strings.Builder
			step := math.Pow(10, -float64(level+1))
			for i := range fan {
				fmt.Fprintf(&sb, "q 1 0 0 1 %.12f 0 cm /Fm Do Q\n", float64(i)*step)
			}
			next = b.form(pdf.Dict{
				"BBox":      rect(0, 0, 100, 50),
				"Resources": pdf.Dict{"XObject": pdf.Dict{"Fm": next}},
			}, sb.String())
		}
		return []nestPage{{
			content: "q 1 0 0 1 20 100 cm /Fm Do Q",
			res:     pdf.Dict{"XObject": pdf.Dict{"Fm": next}},
		}}
	})
}

func TestFormFanOut(t *testing.T) {
	out := writeAndRead(t, fanDoc(t, 3, 3), nil, "")
	if n := streamsContaining(t, out, "Secret"); n != 0 {
		t.Errorf("%d streams still hold the text", n)
	}
}

func TestFormBomb(t *testing.T) {
	// ten levels, each of which places the next one ten times: the work is
	// beyond the budget
	var buf bytes.Buffer
	err := Write(&buf, fanDoc(t, 10, 10), nil)
	if !errors.Is(err, ErrTooComplex) {
		t.Errorf("got error %v, want ErrTooComplex", err)
	}
}

func TestFormDeepNesting(t *testing.T) {
	// a very deep chain of forms is processed and the text is removed
	const depth = 200
	build := func(regs []pdf.Dict) *pdf.Reader {
		return nestDoc(t, regs, func(b *nestBuilder) []nestPage {
			next := b.form(pdf.Dict{"BBox": rect(0, 0, 250, 50), "Resources": helv()}, secretText)
			for range depth {
				next = b.form(pdf.Dict{
					"BBox":      rect(0, 0, 250, 50),
					"Resources": pdf.Dict{"XObject": pdf.Dict{"Fm": next}},
				}, "/Fm Do")
			}
			return []nestPage{{
				content: "q 1 0 0 1 20 100 cm /Fm Do Q",
				res:     pdf.Dict{"XObject": pdf.Dict{"Fm": next}},
			}}
		})
	}
	in := build(nil)
	before := pageGlyphs(t, in)
	pos := strings.Index(glyphString(before), "Secret")
	if pos < 0 {
		t.Fatal("test page shows no text")
	}
	out := writeAndRead(t, build([]pdf.Dict{regionOver(before[pos : pos+6])}), nil, "")
	if n := streamsContaining(t, out, "Secret"); n != 0 {
		t.Errorf("%d streams still hold the text", n)
	}
}

// A tagEntry places a structure element in the parent tree.  The entry for
// key is the element itself if idx is negative, and otherwise the element is
// at position idx of the array for key.
type tagEntry struct{ key, idx int }

// structTree writes a structure tree with one element for each entry.  Every
// element has /ActualText, /Alt and /E.
func (b *nestBuilder) structTree(entries ...tagEntry) {
	b.t.Helper()
	rootRef, docRef := b.w.Alloc(), b.w.Alloc()
	single := map[int]pdf.Reference{}
	arrays := map[int]pdf.Array{}
	var kids pdf.Array
	for _, e := range entries {
		ref := b.put(pdf.Dict{
			"Type": pdf.Name("StructElem"), "S": pdf.Name("P"), "P": docRef,
			"ActualText": pdf.String("at"), "Alt": pdf.String("alt"), "E": pdf.String("exp"),
		})
		kids = append(kids, ref)
		if e.idx < 0 {
			single[e.key] = ref
			continue
		}
		arr := arrays[e.key]
		for len(arr) <= e.idx {
			arr = append(arr, nil)
		}
		arr[e.idx] = ref
		arrays[e.key] = arr
	}
	var keys []int
	for k := range single {
		keys = append(keys, k)
	}
	for k := range arrays {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var nums pdf.Array
	for _, k := range keys {
		nums = append(nums, pdf.Integer(k))
		if ref, ok := single[k]; ok {
			nums = append(nums, ref)
		} else {
			nums = append(nums, arrays[k])
		}
	}
	if err := b.w.Put(docRef, pdf.Dict{
		"Type": pdf.Name("StructElem"), "S": pdf.Name("Document"), "P": rootRef,
		"Alt": pdf.String("docalt"), "K": kids,
	}); err != nil {
		b.t.Fatal(err)
	}
	if err := b.w.Put(rootRef, pdf.Dict{
		"Type": pdf.Name("StructTreeRoot"), "K": docRef,
		"ParentTree": pdf.Dict{"Nums": nums},
	}); err != nil {
		b.t.Fatal(err)
	}
	b.w.GetMeta().Catalog.StructTreeRoot = rootRef
}

// checkTagged checks whether the structure element of an entry in the
// output lost its text entries.
func checkTagged(t *testing.T, out *pdf.Reader, e tagEntry, lost bool) {
	t.Helper()
	d := outElem(t, out, e.key, e.idx)
	if lost {
		for _, k := range []pdf.Name{"ActualText", "Alt", "E", "Phoneme", "T", "AF"} {
			if _, ok := d[k]; ok {
				t.Errorf("element %v keeps /%s", e, k)
			}
		}
		return
	}
	// the entries which structTree sets
	for _, k := range []pdf.Name{"ActualText", "Alt", "E"} {
		if _, ok := d[k]; !ok {
			t.Errorf("element %v lost /%s", e, k)
		}
	}
}

func TestFormLostMCID(t *testing.T) {
	lost, kept := tagEntry{5, 3}, tagEntry{5, 4}
	build := func(regs []pdf.Dict) *pdf.Reader {
		return nestDoc(t, regs, func(b *nestBuilder) []nestPage {
			b.structTree(lost, kept)
			fm := b.form(pdf.Dict{
				"BBox": rect(0, 0, 250, 50), "Resources": helv(),
				"StructParents": pdf.Integer(5),
			}, "/Span <</MCID 3 /ActualText (secretX)>> BDC\n"+secretText+"\nEMC\n"+
				"/Span <</MCID 4>> BDC BT /F1 12 Tf 0 30 Td (Kept) Tj ET EMC")
			return []nestPage{{
				content: "q 1 0 0 1 20 100 cm /Fm1 Do Q",
				res:     pdf.Dict{"XObject": pdf.Dict{"Fm1": fm}},
			}}
		})
	}
	in := build(nil)
	before := pageGlyphs(t, in)
	pos := strings.Index(glyphString(before), "Secret")
	in = build([]pdf.Dict{regionOver(before[pos : pos+6])})
	out := writeAndRead(t, in, nil, "")
	if n := streamsContaining(t, out, "secretX"); n != 0 {
		t.Error("the output holds the ActualText of the lost span")
	}
	checkTagged(t, out, lost, true)
	checkTagged(t, out, kept, false)
}

func TestFormImage(t *testing.T) {
	data := secretRGB(imgW, imgH, func(x, y int) bool {
		return x >= 2 && x <= 4 && y >= 5 && y <= 7
	})
	in := nestDoc(t, []pdf.Dict{{"Rect": rect(127, 126, 143, 144)}}, func(b *nestBuilder) []nestPage {
		img, err := b.rm.Embed(rgbDict(imgW, imgH, data))
		if err != nil {
			t.Fatal(err)
		}
		fm := b.form(pdf.Dict{
			"BBox":      rect(0, 0, 100, 100),
			"Resources": pdf.Dict{"XObject": pdf.Dict{"Im1": img}},
		}, "q 100 0 0 100 0 0 cm /Im1 Do Q")
		return []nestPage{{
			content: "q 1 0 0 1 100 100 cm /Fm1 Do Q",
			res:     pdf.Dict{"XObject": pdf.Dict{"Fm1": fm}},
		}}
	})
	out := writeAndRead(t, in, nil, "")
	checkNoSecret(t, out)
	if n := len(fileStreams(t, out)); n < 3 {
		t.Errorf("only %d streams in the output", n)
	}
}

func TestFormDoesNotChangeUnrelatedResources(t *testing.T) {
	// forms which are not painted stay in the resources
	in := nestDoc(t, []pdf.Dict{{"Rect": rect(0, 250, 40, 290)}}, func(b *nestBuilder) []nestPage {
		fm := b.form(pdf.Dict{"BBox": rect(0, 0, 250, 50), "Resources": helv()}, secretText)
		return []nestPage{{
			content: "q 1 0 0 1 20 100 cm /Fm1 Do Q",
			res:     pdf.Dict{"XObject": pdf.Dict{"Fm1": fm, "Fm2": fm}},
		}}
	})
	out := writeAndRead(t, in, nil, "")
	page := pageAt(t, out, 0)
	if xobject(t, out, page, "Fm2") == nil {
		t.Error("unused form was dropped")
	}
}

func TestFormInheritedTextState(t *testing.T) {
	// the same form at the same place, with different inherited font sizes:
	// only the larger text reaches the region
	page := "/F1 12 Tf q 1 0 0 1 20 100 cm /Fm1 Do Q /F1 24 Tf q 1 0 0 1 20 100 cm /Fm1 Do Q"
	build := singleForm(t, page, pdf.Dict{"Resources": nil}, "BT 0 10 Td (Hello Secret World) Tj ET")
	in := build([]pdf.Dict{{"Rect": rect(170, 108, 180, 125)}})
	out := writeAndRead(t, in, nil, "")
	names := doNames(t, out, pageAt(t, out, 0))
	// the unchanged placement keeps the original, the other one is a copy
	var orig int
	seen := map[pdf.Name]bool{}
	for _, n := range names {
		if n == "Fm1" {
			orig++
		}
		if seen[n] {
			t.Errorf("name %s is used twice", n)
		}
		seen[n] = true
	}
	if orig != 1 || len(names) < 2 {
		t.Errorf("got %d originals among %v, want 1 and a copy", orig, names)
	}
}

func TestInheritedPatternColour(t *testing.T) {
	in := nestDoc(t, []pdf.Dict{{"Rect": rect(41, 25, 47, 31)}}, func(b *nestBuilder) []nestPage {
		pat := b.stream(pdf.Dict{
			"Type": pdf.Name("Pattern"), "PatternType": pdf.Integer(1),
			"PaintType": pdf.Integer(1), "TilingType": pdf.Integer(1),
			"BBox": rect(0, 0, 40, 20), "XStep": pdf.Integer(40), "YStep": pdf.Integer(20),
			"Resources": helv(),
		}, "BT /F1 12 Tf 1 4 Td (Secret) Tj ET")
		fm := b.form(nil, "10 10 180 180 re f")
		return []nestPage{{
			content: "/Pattern cs /P1 scn q /Fm1 Do Q",
			res: pdf.Dict{
				"Pattern": pdf.Dict{"P1": pat},
				"XObject": pdf.Dict{"Fm1": fm},
			},
		}}
	})
	out := writeAndRead(t, in, nil, "")
	if n := streamsContaining(t, out, "Secret"); n != 0 {
		t.Errorf("%d streams still hold the pattern text", n)
	}
}

func TestDroppedFormRecords(t *testing.T) {
	lost := []tagEntry{{7, 2}, {8, 5}, {9, -1}, {11, -1}}
	kept := tagEntry{20, -1}
	in := nestDoc(t, []pdf.Dict{{"Rect": rect(0, 0, 300, 300)}}, func(b *nestBuilder) []nestPage {
		b.structTree(append(slices.Clone(lost), kept)...)
		inner := b.form(pdf.Dict{
			"StructParents": pdf.Integer(8), "StructParent": pdf.Integer(11),
		}, "/Span <</MCID 5>> BDC 0 0 10 10 re f EMC")
		outer := b.form(pdf.Dict{
			"StructParents": pdf.Integer(7), "StructParent": pdf.Integer(9),
			"Resources": pdf.Dict{"XObject": pdf.Dict{"In": inner}},
		}, "/Span <</MCID 2>> BDC 0 0 10 10 re f EMC /In Do")
		return []nestPage{{
			content: "q 1 0 0 1 20 100 cm /Fm1 Do Q",
			res:     pdf.Dict{"XObject": pdf.Dict{"Fm1": outer}},
		}}
	})
	out := writeAndRead(t, in, nil, "")
	for _, e := range lost {
		checkTagged(t, out, e, true)
	}
	checkTagged(t, out, kept, false)
}

func TestDroppedFormRecordsGroupAndPattern(t *testing.T) {
	lost := []tagEntry{{8, 5}, {9, -1}, {11, -1}, {12, -1}}
	kept := tagEntry{20, -1}
	in := nestDoc(t, []pdf.Dict{{"Rect": rect(0, 0, 300, 300)}}, func(b *nestBuilder) []nestPage {
		b.structTree(append(slices.Clone(lost), kept)...)
		group := b.form(pdf.Dict{
			"StructParents": pdf.Integer(8), "StructParent": pdf.Integer(11),
		}, "/Span <</MCID 5>> BDC 0 0 10 10 re f EMC")
		pat := b.stream(pdf.Dict{
			"Type": pdf.Name("Pattern"), "PatternType": pdf.Integer(1),
			"PaintType": pdf.Integer(1), "TilingType": pdf.Integer(1),
			"BBox": rect(0, 0, 10, 10), "XStep": pdf.Integer(10), "YStep": pdf.Integer(10),
			"Resources": pdf.Dict{"XObject": pdf.Dict{"In": b.form(pdf.Dict{
				"StructParent": pdf.Integer(12),
			}, "0 0 5 5 re f")}},
		}, "/In Do")
		outer := b.form(pdf.Dict{
			"StructParents": pdf.Integer(7), "StructParent": pdf.Integer(9),
			"BBox": rect(0, 0, 50, 50),
			"Resources": pdf.Dict{
				"ExtGState": pdf.Dict{"G1": pdf.Dict{"SMask": pdf.Dict{
					"Type": pdf.Name("Mask"), "S": pdf.Name("Luminosity"), "G": group,
				}}},
				"Pattern": pdf.Dict{"P1": pat},
			},
		}, "/G1 gs /Pattern cs /P1 scn 0 0 10 10 re f")
		return []nestPage{{
			content: "q 1 0 0 1 20 100 cm /Fm1 Do Q",
			res:     pdf.Dict{"XObject": pdf.Dict{"Fm1": outer}},
		}}
	})
	out := writeAndRead(t, in, nil, "")
	for _, e := range lost {
		checkTagged(t, out, e, true)
	}
	checkTagged(t, out, kept, false)
}

func TestRewrittenFormStructParent(t *testing.T) {
	lost := tagEntry{4, -1}
	build := func(regs []pdf.Dict) *pdf.Reader {
		return nestDoc(t, regs, func(b *nestBuilder) []nestPage {
			b.structTree(lost)
			fm := b.form(pdf.Dict{
				"BBox": rect(0, 0, 250, 50), "Resources": helv(),
				"StructParent": pdf.Integer(4),
			}, secretText)
			return []nestPage{{
				content: "q 1 0 0 1 20 100 cm /Fm1 Do Q",
				res:     pdf.Dict{"XObject": pdf.Dict{"Fm1": fm}},
			}}
		})
	}
	in := build(nil)
	before := pageGlyphs(t, in)
	pos := strings.Index(glyphString(before), "Secret")
	in = build([]pdf.Dict{regionOver(before[pos : pos+6])})
	checkTagged(t, writeAndRead(t, in, nil, ""), lost, true)
}

func TestFormWithoutBBox(t *testing.T) {
	build := singleForm(t, "q 1 0 0 1 20 100 cm /Fm1 Do Q", pdf.Dict{"BBox": nil}, secretText)

	// the glyph positions come from the same form with a BBox
	ref := singleForm(t, "q 1 0 0 1 20 100 cm /Fm1 Do Q", nil, secretText)(nil)
	before := pageGlyphs(t, ref)
	pos := strings.Index(glyphString(before), "Secret")
	out := writeAndRead(t, build([]pdf.Dict{regionOver(before[pos : pos+6])}), nil, "")
	if n := streamsContaining(t, out, "Secret"); n != 0 {
		t.Errorf("%d streams still hold the text", n)
	}
	if n := streamsContaining(t, out, "Hello"); n == 0 {
		t.Error("the form lost the text outside the region")
	}

	out = writeAndRead(t, build([]pdf.Dict{{"Rect": rect(10, 250, 40, 290)}}), nil, "")
	if n := streamsContaining(t, out, "Hello Secret World"); n != 1 {
		t.Errorf("form without BBox away from the regions was changed")
	}
}

func TestSoftMaskGroupWithoutBBox(t *testing.T) {
	in := nestDoc(t, []pdf.Dict{{"Rect": rect(60, 108, 100, 130)}}, func(b *nestBuilder) []nestPage {
		g := b.form(pdf.Dict{"BBox": nil, "Resources": helv()}, secretText)
		return []nestPage{{
			content: "q 1 0 0 1 20 100 cm /GS1 gs 0 0 200 100 re f Q",
			res: pdf.Dict{"ExtGState": pdf.Dict{"GS1": pdf.Dict{
				"SMask": pdf.Dict{"S": pdf.Name("Luminosity"), "G": g},
			}}},
		}}
	})
	out := writeAndRead(t, in, nil, "")
	if n := streamsContaining(t, out, "Secret"); n != 0 {
		t.Errorf("%d streams still hold the soft-mask text", n)
	}
	if n := streamsContaining(t, out, "Hello"); n == 0 {
		t.Error("the soft-mask group lost all its text")
	}
}

// TestTilingPatternHugeTranslation checks that the text of a pattern is
// removed where it meets a region, when the lattice cells over the region
// have indices near 1e16.
func TestTilingPatternHugeTranslation(t *testing.T) {
	// the lattice cells are far from the origin, where adding one to a
	// float64 counter has no effect
	m := pdf.Array{pdf.Integer(1), pdf.Integer(0), pdf.Integer(0), pdf.Integer(1),
		pdf.Number(1e17), pdf.Number(1e17)}
	region := pdf.Dict{"Rect": rect(0, 0, 300, 300)}
	in := patternDoc(t, []pdf.Dict{region}, "0 0 200 200 re", pdf.Dict{
		"Matrix": m, "XStep": pdf.Integer(10), "YStep": pdf.Integer(10),
	})
	out := writeAndRead(t, in, nil, "")
	if n := streamsContaining(t, out, "Secret"); n != 0 {
		t.Errorf("%d streams still hold the pattern text", n)
	}
}

func TestFormBudgetInsideNestedStream(t *testing.T) {
	// the rewritten form is larger than the budget allows
	// few operators with large operands: the cost is in the bytes written
	body := secretText + "\n" + strings.Repeat("["+strings.Repeat("0 ", 4096)+"] 0 d\n", 2000)
	in := nestDoc(t, []pdf.Dict{{"Rect": rect(30, 105, 60, 120)}}, func(b *nestBuilder) []nestPage {
		fm := b.form(pdf.Dict{"BBox": rect(0, 0, 250, 50), "Resources": helv()}, body)
		return []nestPage{{
			content: "q 1 0 0 1 20 100 cm /Fm Do Q",
			res:     pdf.Dict{"XObject": pdf.Dict{"Fm": fm}},
		}}
	})
	if err := Write(io.Discard, in, nil); !errors.Is(err, ErrTooComplex) {
		t.Errorf("got error %v, want ErrTooComplex", err)
	}
}

// TestFormCacheKeepsWalkers checks that the copy of a form which inherits a
// pattern colour is not reused for the pattern of a different form.
func TestFormCacheKeepsWalkers(t *testing.T) {
	const n = 20
	in := nestDoc(t, []pdf.Dict{{"Rect": rect(41, 25, 47, 31)}}, func(b *nestBuilder) []nestPage {
		shared := b.form(pdf.Dict{"Resources": pdf.Dict{}}, "0 0 200 200 re f")
		xobj := pdf.Dict{}
		var sb strings.Builder
		for i := range n {
			pat := b.stream(pdf.Dict{
				"Type": pdf.Name("Pattern"), "PatternType": pdf.Integer(1),
				"PaintType": pdf.Integer(1), "TilingType": pdf.Integer(1),
				"BBox": rect(0, 0, 40, 20), "XStep": pdf.Integer(40), "YStep": pdf.Integer(20),
				"Resources": helv(),
			}, "BT /F1 12 Tf 1 4 Td (Secret) Tj ET")
			fm := b.form(pdf.Dict{"Resources": pdf.Dict{
				"Pattern": pdf.Dict{"P1": pat},
				"XObject": pdf.Dict{"S": shared},
			}}, "/Pattern cs /P1 scn /S Do")
			name := pdf.Name(fmt.Sprintf("F%d", i))
			xobj[name] = fm
			fmt.Fprintf(&sb, "/%s Do\n", name)
		}
		return []nestPage{{content: sb.String(), res: pdf.Dict{"XObject": xobj}}}
	})
	out := writeAndRead(t, in, nil, "")
	if n := streamsContaining(t, out, "Secret"); n > 0 {
		t.Errorf("%d patterns keep the text under the region", n)
	}
}

// TestFormContentOutsideBBox checks that text outside the /BBox of a form,
// which text extraction finds, is removed where it meets a region.
func TestFormContentOutsideBBox(t *testing.T) {
	build := singleForm(t, "q 1 0 0 1 20 100 cm /Fm1 Do Q", pdf.Dict{"BBox": rect(0, 0, 10, 10)}, secretText)
	checkNested(t, build, "Secret")
}

// TestSoftMaskGroupContentOutsideBBox checks that the text of a soft-mask
// group is removed where it meets a region, even outside the group's /BBox.
func TestSoftMaskGroupContentOutsideBBox(t *testing.T) {
	in := nestDoc(t, []pdf.Dict{{"Rect": rect(60, 108, 100, 130)}}, func(b *nestBuilder) []nestPage {
		g := b.form(pdf.Dict{"BBox": rect(0, 0, 10, 10), "Resources": helv()}, secretText)
		return []nestPage{{
			content: "q 1 0 0 1 20 100 cm /GS1 gs 0 0 200 100 re f Q",
			res: pdf.Dict{"ExtGState": pdf.Dict{"GS1": pdf.Dict{
				"SMask": pdf.Dict{"S": pdf.Name("Luminosity"), "G": g},
			}}},
		}}
	})
	out := writeAndRead(t, in, nil, "")
	if n := streamsContaining(t, out, "Secret"); n != 0 {
		t.Errorf("%d streams still hold the soft-mask text", n)
	}
}

// TestFormLostMCIDWithoutKey checks that every structure element loses its
// text entries if a form whose marked content lost content has no usable
// /StructParents entry.
func TestFormLostMCIDWithoutKey(t *testing.T) {
	other := tagEntry{20, -1}
	cases := []struct {
		name string
		key  pdf.Object
	}{
		{"absent", nil},
		{"negative", pdf.Integer(-3)},
		{"not an integer", pdf.Name("five")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			build := func(regs []pdf.Dict) *pdf.Reader {
				return nestDoc(t, regs, func(b *nestBuilder) []nestPage {
					b.structTree(tagEntry{5, 3}, other)
					d := pdf.Dict{"BBox": rect(0, 0, 250, 50), "Resources": helv()}
					if c.key != nil {
						d["StructParents"] = c.key
					}
					fm := b.form(d, "/Span <</MCID 3>> BDC\n"+secretText+"\nEMC")
					return []nestPage{{
						content: "q 1 0 0 1 20 100 cm /Fm1 Do Q",
						res:     pdf.Dict{"XObject": pdf.Dict{"Fm1": fm}},
					}}
				})
			}
			before := pageGlyphs(t, build(nil))
			pos := strings.Index(glyphString(before), "Secret")
			out := writeAndRead(t, build([]pdf.Dict{regionOver(before[pos : pos+6])}), nil, "")
			checkTagged(t, out, other, true)
		})
	}
}

// TestFormInvalidStructParent checks that every structure element loses its
// text entries if a rewritten form has an unusable /StructParent entry.
func TestFormInvalidStructParent(t *testing.T) {
	other := tagEntry{20, -1}
	build := func(regs []pdf.Dict) *pdf.Reader {
		return nestDoc(t, regs, func(b *nestBuilder) []nestPage {
			b.structTree(other)
			fm := b.form(pdf.Dict{
				"BBox": rect(0, 0, 250, 50), "Resources": helv(),
				"StructParent": pdf.Real(4.5),
			}, secretText)
			return []nestPage{{
				content: "q 1 0 0 1 20 100 cm /Fm1 Do Q",
				res:     pdf.Dict{"XObject": pdf.Dict{"Fm1": fm}},
			}}
		})
	}
	before := pageGlyphs(t, build(nil))
	pos := strings.Index(glyphString(before), "Secret")
	out := writeAndRead(t, build([]pdf.Dict{regionOver(before[pos : pos+6])}), nil, "")
	checkTagged(t, out, other, true)
	checkStable(t, out)
}

// TestFormIndirectStructParent checks that an indirect /StructParent entry
// of a rewritten form is followed.
func TestFormIndirectStructParent(t *testing.T) {
	lost, kept := tagEntry{4, -1}, tagEntry{20, -1}
	build := func(regs []pdf.Dict) *pdf.Reader {
		return nestDoc(t, regs, func(b *nestBuilder) []nestPage {
			b.structTree(lost, kept)
			fm := b.form(pdf.Dict{
				"BBox": rect(0, 0, 250, 50), "Resources": helv(),
				"StructParent": b.put(pdf.Integer(4)),
			}, secretText)
			return []nestPage{{
				content: "q 1 0 0 1 20 100 cm /Fm1 Do Q",
				res:     pdf.Dict{"XObject": pdf.Dict{"Fm1": fm}},
			}}
		})
	}
	before := pageGlyphs(t, build(nil))
	pos := strings.Index(glyphString(before), "Secret")
	out := writeAndRead(t, build([]pdf.Dict{regionOver(before[pos : pos+6])}), nil, "")
	checkTagged(t, out, lost, true)
	checkTagged(t, out, kept, false)
}

// TestFormEndsInTextObject checks that the copy of a form whose content
// ends inside a text object or a marked-content sequence closes them.
func TestFormEndsInTextObject(t *testing.T) {
	build := singleForm(t, "q 1 0 0 1 20 100 cm /Fm1 Do Q", nil,
		"/Span <</MCID 0>> BDC q BT /F1 12 Tf 0 10 Td (Hello Secret World) Tj")
	out := checkNested(t, build, "Secret")
	found := false
	for _, data := range fileStreams(t, out) {
		if !bytes.Contains(data, []byte("Hello")) {
			continue
		}
		found = true
		checkBalanced(t, scanOps(t, data))
	}
	if !found {
		t.Fatal("no copy of the form")
	}
}

// TestTilingPatternExtremeMatrix checks that the text of a pattern is
// removed where it meets a region, when the pattern space is far from page
// space in scale.
func TestTilingPatternExtremeMatrix(t *testing.T) {
	cases := []struct {
		name     string
		m        pdf.Array
		bbox     pdf.Array
		step     pdf.Object
		textSize string
	}{
		{"huge cells",
			pdf.Array{pdf.Number(1e-198), pdf.Integer(0), pdf.Integer(0), pdf.Number(1e-198), pdf.Integer(0), pdf.Integer(0)},
			pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Number(4e199), pdf.Number(2e199)},
			pdf.Number(4e199), "1.2e199"},
		{"tiny cells",
			pdf.Array{pdf.Number(1e198), pdf.Integer(0), pdf.Integer(0), pdf.Number(1e198), pdf.Integer(0), pdf.Integer(0)},
			pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Number(4e-197), pdf.Number(2e-197)},
			pdf.Number(4e-197), "1.2e-197"},
		{"thin",
			pdf.Array{pdf.Integer(1), pdf.Integer(0), pdf.Integer(1), pdf.Number(1e-300), pdf.Integer(0), pdf.Integer(0)},
			rect(0, 0, 40, 20), pdf.Integer(40), "12"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := nestDocUnchecked(t, []pdf.Dict{{"Rect": rect(0, 0, 300, 300)}}, func(b *nestBuilder) []nestPage {
				pat := b.stream(pdf.Dict{
					"Type": pdf.Name("Pattern"), "PatternType": pdf.Integer(1),
					"PaintType": pdf.Integer(1), "TilingType": pdf.Integer(1),
					"BBox": c.bbox, "XStep": c.step, "YStep": c.step,
					"Matrix": c.m, "Resources": helv(),
				}, "BT /F1 "+c.textSize+" Tf (Secret) Tj ET")
				return []nestPage{{
					content: "/Pattern cs /P1 scn 0 0 200 200 re f",
					res:     pdf.Dict{"Pattern": pdf.Dict{"P1": pat}},
				}}
			})
			out := writeAndRead(t, in, nil, "")
			if n := streamsContaining(t, out, "Secret"); n != 0 {
				t.Errorf("%d streams still hold the pattern text", n)
			}
		})
	}
}

func TestPatternInheritsFormState(t *testing.T) {
	// the pattern has no Tf: its text uses the font which the page sets
	// before it paints the form, which holds the pattern
	in := nestDoc(t, []pdf.Dict{{"Rect": rect(1, 1, 20, 8)}}, func(b *nestBuilder) []nestPage {
		pat := b.stream(pdf.Dict{
			"Type": pdf.Name("Pattern"), "PatternType": pdf.Integer(1),
			"PaintType": pdf.Integer(1), "TilingType": pdf.Integer(1),
			"BBox": rect(0, 0, 100, 20), "XStep": pdf.Integer(100), "YStep": pdf.Integer(20),
			"Resources": pdf.Dict{},
		}, "BT 0 0 Td (SECRET) Tj ET")
		fm := b.form(pdf.Dict{
			"Resources": pdf.Dict{"Pattern": pdf.Dict{"P": pat}},
		}, "/Pattern cs /P scn 0 0 300 300 re f")
		return []nestPage{{
			content: "/F1 12 Tf /Fm Do",
			res: pdf.Dict{
				"XObject": pdf.Dict{"Fm": fm},
				"Font":    helv()["Font"],
			},
		}}
	})
	out := writeAndRead(t, in, nil, "")
	if n := streamsContaining(t, out, "SECRET"); n != 0 {
		t.Errorf("%d streams still hold the pattern text", n)
	}
}

func TestFormSingularMatrix(t *testing.T) {
	// the form matrix maps the form onto the point (150, 150), which lies in
	// the region; under the identity, the text would lie outside
	in := nestDoc(t, []pdf.Dict{{"Rect": rect(140, 140, 160, 160)}}, func(b *nestBuilder) []nestPage {
		zero := pdf.Array{
			pdf.Integer(0), pdf.Integer(0), pdf.Integer(0),
			pdf.Integer(0), pdf.Integer(0), pdf.Integer(0),
		}
		fm := b.form(pdf.Dict{"Matrix": zero, "Resources": helv()},
			"BT /F1 12 Tf 50 50 Td (SECRET) Tj ET")
		return []nestPage{{
			content: "q 1 0 0 1 150 150 cm /Fm1 Do Q",
			res:     pdf.Dict{"XObject": pdf.Dict{"Fm1": fm}},
		}}
	})
	out := writeAndRead(t, in, nil, "")
	if n := streamsContaining(t, out, "SECRET"); n != 0 {
		t.Errorf("%d streams still hold the text", n)
	}
}

// streamKeys returns the values of the entry key of the streams of r whose
// /Subtype is subtype.
func streamKeys(t testing.TB, r *pdf.Reader, subtype, key pdf.Name) []pdf.Integer {
	t.Helper()
	var res []pdf.Integer
	for i := uint32(1); i < 200; i++ {
		obj, err := r.Get(pdf.NewReference(i, 0), true)
		if err != nil {
			continue
		}
		stm, ok := obj.(*pdf.Stream)
		if !ok || stm.Dict["Subtype"] != subtype {
			continue
		}
		if k, ok := stm.Dict[key].(pdf.Integer); ok {
			res = append(res, k)
		}
	}
	return res
}

// checkDistinctKeys checks that the keys are distinct, that each has an
// entry in the parent tree of out, and that /ParentTreeNextKey lies beyond
// all of them.  The entry for each key is the element itself if idx is
// negative, and otherwise holds the element at position idx.
func checkDistinctKeys(t *testing.T, out *pdf.Reader, keys []pdf.Integer, idx int) {
	t.Helper()
	if len(keys) != 2 || keys[0] == keys[1] {
		t.Fatalf("got keys %v, want two distinct keys", keys)
	}
	for _, k := range keys {
		outElem(t, out, int(k), idx)
	}
	root, err := pdf.Resolve(out, out.GetMeta().Catalog.StructTreeRoot)
	if err != nil {
		t.Fatal(err)
	}
	next, _ := root.(pdf.Dict)["ParentTreeNextKey"].(pdf.Integer)
	for _, k := range keys {
		if next <= k {
			t.Errorf("/ParentTreeNextKey is %d, key %d", next, k)
		}
	}
}

func TestFormCopyStructParents(t *testing.T) {
	// the redacted copy on the first page and the unchanged form on the
	// second page do not share their parent tree key
	lost, kept := tagEntry{5, 3}, tagEntry{5, 4}
	build := func(regs []pdf.Dict) *pdf.Reader {
		return nestDoc(t, regs, func(b *nestBuilder) []nestPage {
			b.structTree(lost, kept)
			fm := b.form(pdf.Dict{
				"BBox": rect(0, 0, 250, 50), "Resources": helv(),
				"StructParents": pdf.Integer(5),
			}, "/Span <</MCID 3>> BDC\n"+secretText+"\nEMC\n"+
				"/Span <</MCID 4>> BDC BT /F1 12 Tf 0 30 Td (Kept) Tj ET EMC")
			res := pdf.Dict{"XObject": pdf.Dict{"Fm1": fm}}
			return []nestPage{
				{content: "q 1 0 0 1 20 100 cm /Fm1 Do Q", res: res},
				{content: "q 1 0 0 1 20 100 cm /Fm1 Do Q", res: res},
			}
		})
	}
	in := build(nil)
	before := pageGlyphs(t, in)
	pos := strings.Index(glyphString(before), "Secret")
	in = build([]pdf.Dict{regionOver(before[pos : pos+6])})
	out := writeAndRead(t, in, nil, "")

	checkDistinctKeys(t, out, streamKeys(t, out, "Form", "StructParents"), kept.idx)
	checkTagged(t, out, lost, true)
	checkTagged(t, out, kept, false)
}

func TestDroppedFormInheritedProperties(t *testing.T) {
	// the form A has no resources, and the name /MC in it selects a
	// different property list in each of the two forms which paint it
	lost := []tagEntry{{7, 1}, {7, 2}}
	in := nestDoc(t, []pdf.Dict{{"Rect": rect(0, 0, 50, 50)}}, func(b *nestBuilder) []nestPage {
		b.structTree(lost...)
		a := b.form(pdf.Dict{"StructParents": pdf.Integer(7)},
			"/Span /MC BDC 0 0 1 1 re f EMC")
		x := func(mcid int) pdf.Reference {
			return b.form(pdf.Dict{"Resources": pdf.Dict{
				"XObject":    pdf.Dict{"A": a},
				"Properties": pdf.Dict{"MC": pdf.Dict{"MCID": pdf.Integer(mcid)}},
			}}, "/A Do")
		}
		dropped := b.form(pdf.Dict{"Resources": pdf.Dict{
			"XObject": pdf.Dict{"X1": x(1), "X2": x(2)},
		}}, "/X1 Do /X2 Do")
		return []nestPage{{
			content: "/C1 Do",
			res:     pdf.Dict{"XObject": pdf.Dict{"C1": deepChain(b, dropped)}},
		}}
	})
	out := writeAndRead(t, in, nil, "")
	for _, e := range lost {
		checkTagged(t, out, e, true)
	}
}

// TestDroppedFormLongChain checks that the marked content at the end of a
// long chain of forms, which are nested too deeply to be rewritten, is found.
func TestDroppedFormLongChain(t *testing.T) {
	lost := tagEntry{9, 0}
	in := nestDoc(t, []pdf.Dict{{"Rect": rect(0, 0, 50, 50)}}, func(b *nestBuilder) []nestPage {
		b.structTree(lost)
		next := b.form(pdf.Dict{"StructParents": pdf.Integer(9)},
			"/Span <</MCID 0>> BDC 0 0 1 1 re f EMC")
		for range 5000 {
			next = b.form(pdf.Dict{
				"Resources": pdf.Dict{"XObject": pdf.Dict{"N": next}},
			}, "/N Do")
		}
		return []nestPage{{
			content: "/N Do",
			res:     pdf.Dict{"XObject": pdf.Dict{"N": next}},
		}}
	})
	out := writeAndRead(t, in, nil, "")
	checkTagged(t, out, lost, true)
}

// TestFormBrokenStream checks that on a page with regions, a form or
// soft-mask group which cannot be decoded completely is replaced by a copy
// of the decoded part, also where nothing of it meets a region.  A page
// without regions keeps the original.
func TestFormBrokenStream(t *testing.T) {
	const visible = "BT /F1 12 Tf 20 20 Td (VisibleText) Tj ET\n"
	const secret = "BT /F1 12 Tf 20 40 Td (TopSecretText) Tj ET\n"
	cases := []struct {
		name   string
		filter pdf.Name
		data   []byte
		shown  string // the text which the decoded part shows
	}{
		{"truncated", "FlateDecode", truncatedFlate(visible, secret), "VisibleText"},
		{"unknown filter", "NoSuchDecode", []byte(visible + secret), ""},
	}
	for _, c := range cases {
		for _, use := range []string{"direct", "nested", "soft mask"} {
			for _, plainUse := range []bool{false, true} {
				name := fmt.Sprintf("%s/%s/plain=%t", c.name, use, plainUse)
				t.Run(name, func(t *testing.T) {
					in := nestDoc(t, []pdf.Dict{{"Rect": rect(200, 200, 250, 250)}}, func(b *nestBuilder) []nestPage {
						fm := putRawStream(t, b.w, pdf.Dict{
							"Type": pdf.Name("XObject"), "Subtype": pdf.Name("Form"),
							"BBox": rect(0, 0, 300, 300), "Resources": helv(),
							"Filter": c.filter,
						}, c.data)
						first := nestPage{
							content: "/Fm1 Do",
							res:     pdf.Dict{"XObject": pdf.Dict{"Fm1": fm}},
						}
						switch use {
						case "nested":
							first.res = pdf.Dict{"XObject": pdf.Dict{"Fm1": b.form(pdf.Dict{
								"Resources": pdf.Dict{"XObject": pdf.Dict{"In": fm}},
							}, "/In Do")}}
						case "soft mask":
							first = nestPage{
								content: "/GS1 gs 0 0 300 300 re f",
								res: pdf.Dict{"ExtGState": pdf.Dict{"GS1": pdf.Dict{
									"SMask": pdf.Dict{"S": pdf.Name("Luminosity"), "G": fm},
								}}},
							}
						}
						pages := []nestPage{first}
						if plainUse {
							pages = append(pages, nestPage{
								content: "/Fm1 Do",
								res:     pdf.Dict{"XObject": pdf.Dict{"Fm1": fm}},
							})
						}
						return pages
					})
					out, data := writeRaw(t, in)

					want := 0
					if plainUse {
						want = 1
					}
					if n := bytes.Count(data, c.data); n != want {
						t.Errorf("output holds %d copies of the original stream, want %d", n, want)
					}
					if n := streamsContaining(t, out, "TopSecretText"); n != 0 {
						t.Errorf("%d streams hold the undecoded text", n)
					}

					before, _ := pageContents(t, in)
					after, _ := pageContents(t, out)
					if got := glyphString(after[0].glyphs); use != "soft mask" && got != c.shown {
						t.Errorf("page with regions shows %q, want %q", got, c.shown)
					}
					if plainUse {
						if got, want := glyphString(after[1].glyphs), glyphString(before[1].glyphs); got != want {
							t.Errorf("page without regions shows %q, want %q", got, want)
						}
					}
				})
			}
		}
	}
}

// checkNames checks that the names which the content of a page, and of the
// forms, soft-mask groups and patterns it paints, uses for XObjects, extended
// graphics states, patterns and property lists are among the resources.  A
// stream without resources of its own uses those of the stream which paints
// it.
func checkNames(t testing.TB, r *pdf.Reader, page pdf.Dict) {
	t.Helper()
	cur := pdf.CursorAt(pdf.NewExtractor(r), nil)
	dict := func(obj pdf.Object) pdf.Dict {
		t.Helper()
		d, err := pdf.Optional(cur.Dict(obj))
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	lookup := func(res pdf.Dict, category, name pdf.Name) pdf.Object {
		t.Helper()
		obj := dict(res[category])[name]
		if obj == nil {
			t.Errorf("%s resource %s is missing", category, name)
		}
		return obj
	}

	var visit func(data []byte, res pdf.Dict, depth int)
	nested := func(obj pdf.Object, res pdf.Dict, depth int) {
		t.Helper()
		stm, err := pdf.Optional(cur.Stream(obj))
		if err != nil {
			t.Fatal(err)
		}
		if stm == nil || depth > 10 {
			return
		}
		data, err := pdf.ReadAll(r, nil, stm, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		if own, ok := stm.Dict["Resources"]; ok {
			res = dict(own)
		}
		visit(data, res, depth+1)
	}
	group := func(gs pdf.Dict, res pdf.Dict, depth int) {
		t.Helper()
		if sm := dict(gs["SMask"]); sm != nil {
			nested(sm["G"], res, depth)
		}
	}
	visit = func(data []byte, res pdf.Dict, depth int) {
		t.Helper()
		for _, op := range scanOps(t, data) {
			if len(op.args) == 0 {
				continue
			}
			name, _ := op.args[len(op.args)-1].(pdf.Name)
			switch op.name {
			case content.OpXObject:
				nested(lookup(res, "XObject", name), res, depth)
			case content.OpSetExtGState:
				group(dict(lookup(res, "ExtGState", name)), res, depth)
			case content.OpSetFillColorN, content.OpSetStrokeColorN:
				if name == "" {
					continue
				}
				pat := lookup(res, "Pattern", name)
				if stm, _ := pdf.Optional(cur.Stream(pat)); stm != nil {
					nested(pat, res, depth)
				} else {
					group(dict(dict(pat)["ExtGState"]), res, depth)
				}
			case content.OpBeginMarkedContentWithProperties, content.OpMarkedContentPointWithProperties:
				if name != "" && len(op.args) == 2 {
					lookup(res, "Properties", name)
				}
			}
		}
	}
	visit(contentBytes(t, r, page), dict(page["Resources"]), 0)
}

// TestInheritedFormKeepsResources checks that a form without resources of
// its own, which loses no content, still finds the resources it uses after
// the rewrite of the stream which paints it, where that stream does not use
// them itself.
func TestInheritedFormKeepsResources(t *testing.T) {
	const secret = "BT /F1 12 Tf 20 100 Td (Secret) Tj ET"
	cases := []struct {
		name  string
		setup func(b *nestBuilder) nestPage
	}{
		{"image", func(b *nestBuilder) nestPage {
			im, err := b.rm.Embed(rgbDict(2, 2, secretRGB(2, 2, nil)))
			if err != nil {
				t.Fatal(err)
			}
			fm := b.form(nil, "q 50 0 0 50 200 200 cm /Im1 Do Q")
			return nestPage{
				content: "q 100 0 0 100 20 100 cm /Im1 Do Q /Fm Do",
				res:     pdf.Dict{"XObject": pdf.Dict{"Im1": im, "Fm": fm}},
			}
		}},
		{"form", func(b *nestBuilder) nestPage {
			inner := b.form(pdf.Dict{"Resources": helv()}, secret)
			fm := b.form(nil, "q 1 0 0 1 200 0 cm /Inner Do Q")
			return nestPage{
				content: "/Inner Do /Fm Do",
				res:     pdf.Dict{"XObject": pdf.Dict{"Inner": inner, "Fm": fm}},
			}
		}},
		{"soft mask", func(b *nestBuilder) nestPage {
			g := b.form(pdf.Dict{"Resources": helv()}, secret)
			fm := b.form(nil, "q 1 0 0 1 200 0 cm /GS1 gs 0 0 10 10 re f Q")
			return nestPage{
				content: "q /GS1 gs 0 0 300 300 re f Q /Fm Do",
				res: pdf.Dict{
					"XObject": pdf.Dict{"Fm": fm},
					"ExtGState": pdf.Dict{"GS1": pdf.Dict{
						"SMask": pdf.Dict{"S": pdf.Name("Luminosity"), "G": g},
					}},
				},
			}
		}},
		{"property list", func(b *nestBuilder) nestPage {
			fm := b.form(nil, "/P /MC0 BDC 200 200 10 10 re f EMC")
			return nestPage{
				content: "/P /MC0 BDC " + secret + " EMC /Fm Do",
				res: pdf.Dict{
					"Font":       helv()["Font"],
					"XObject":    pdf.Dict{"Fm": fm},
					"Properties": pdf.Dict{"MC0": pdf.Dict{"ActualText": pdf.String("words")}},
				},
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := nestDoc(t, []pdf.Dict{{"Rect": rect(0, 0, 150, 300)}}, func(b *nestBuilder) []nestPage {
				return []nestPage{c.setup(b)}
			})
			out := writeAndRead(t, in, nil, "")
			checkNames(t, out, pageAt(t, out, 0))
			for _, g := range pageGlyphs(t, out) {
				if g.cx < 150 {
					t.Errorf("glyph %q in the region", g.text)
				}
			}
		})
	}
}

// shadingGroup returns the decoded soft-mask group in the graphics state of
// the shading pattern which a page uses under the given name.
func shadingGroup(t testing.TB, r *pdf.Reader, page pdf.Dict, name pdf.Name) []byte {
	t.Helper()
	cur := pdf.CursorAt(pdf.NewExtractor(r), nil)
	res, err := cur.Dict(page["Resources"])
	if err != nil {
		t.Fatal(err)
	}
	pat, err := cur.Dict(path(t, r, res, "Pattern", name))
	if err != nil {
		t.Fatal(err)
	}
	stm, err := cur.Stream(path(t, r, pat, "ExtGState", "SMask", "G"))
	if err != nil || stm == nil {
		t.Fatalf("pattern %s has no soft-mask group: %v", name, err)
	}
	data, err := pdf.ReadAll(r, nil, stm, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// shadingPattern writes a shading pattern with the given matrix, whose
// graphics state has a soft mask with the given group content.
func shadingPattern(b *nestBuilder, m pdf.Array, group string) pdf.Reference {
	g := b.form(pdf.Dict{
		"Resources": helv(),
		"Group":     pdf.Dict{"S": pdf.Name("Transparency"), "CS": pdf.Name("DeviceGray")},
	}, group)
	return b.put(pdf.Dict{
		"PatternType": pdf.Integer(2),
		"Matrix":      m,
		"Shading": pdf.Dict{
			"ShadingType": pdf.Integer(2), "ColorSpace": pdf.Name("DeviceGray"),
			"Coords": rect(0, 0, 300, 0),
			"Function": pdf.Dict{
				"FunctionType": pdf.Integer(2), "Domain": pdf.Array{pdf.Integer(0), pdf.Integer(1)},
				"C0": pdf.Array{pdf.Integer(0)}, "C1": pdf.Array{pdf.Integer(1)}, "N": pdf.Integer(1),
			},
		},
		"ExtGState": pdf.Dict{"SMask": pdf.Dict{"S": pdf.Name("Luminosity"), "G": g}},
	})
}

// TestShadingPatternSoftMask checks that the text of the soft-mask group of a
// shading pattern is removed where it lies under a region in pattern space,
// whichever way the pattern is used, and that a page without regions keeps
// the whole group.
func TestShadingPatternSoftMask(t *testing.T) {
	identity := pdf.Array{pdf.Integer(1), pdf.Integer(0), pdf.Integer(0), pdf.Integer(1), pdf.Integer(0), pdf.Integer(0)}
	shifted := pdf.Array{pdf.Integer(1), pdf.Integer(0), pdf.Integer(0), pdf.Integer(1), pdf.Integer(-200), pdf.Integer(0)}
	cases := []struct {
		name, use string
		matrix    pdf.Array

		// group is the content of the soft-mask group, in pattern space
		group string
	}{
		{"fill", "/Pattern cs /P0 scn 0 0 300 300 re f", identity,
			"BT /F1 12 Tf 20 100 Td (Secret) Tj 180 0 Td (Kept) Tj ET"},
		{"stroke", "/Pattern CS /P0 SCN 0 0 300 300 re S", identity,
			"BT /F1 12 Tf 20 100 Td (Secret) Tj 180 0 Td (Kept) Tj ET"},
		{"text", "/Pattern cs /P0 scn BT /F1 12 Tf 250 250 Td (x) Tj ET", identity,
			"BT /F1 12 Tf 20 100 Td (Secret) Tj 180 0 Td (Kept) Tj ET"},
		{"pattern matrix", "/Pattern cs /P0 scn 0 0 300 300 re f", shifted,
			"BT /F1 12 Tf 220 100 Td (Secret) Tj 180 0 Td (Kept) Tj ET"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := nestDoc(t, []pdf.Dict{{"Rect": rect(0, 0, 150, 300)}}, func(b *nestBuilder) []nestPage {
				pat := shadingPattern(b, c.matrix, c.group)
				res := pdf.Dict{"Pattern": pdf.Dict{"P0": pat}, "Font": helv()["Font"]}
				return []nestPage{{content: c.use, res: res}, {content: c.use, res: res}}
			})
			out := writeAndRead(t, in, nil, "")
			for i := range 2 {
				checkNames(t, out, pageAt(t, out, i))
			}
			data := shadingGroup(t, out, pageAt(t, out, 0), "P0")
			if bytes.Contains(data, []byte("Secret")) {
				t.Error("the soft-mask group keeps the text in the region")
			}
			if !bytes.Contains(data, []byte("Kept")) {
				t.Error("the soft-mask group lost the text outside the region")
			}
			if data := shadingGroup(t, out, pageAt(t, out, 1), "P0"); !bytes.Contains(data, []byte("Secret")) {
				t.Error("the page without regions lost the text of the soft-mask group")
			}
		})
	}
}

// TestReferenceXObject checks that on a page with regions, a reference
// XObject is painted without its reference to the original page, whether or
// not it loses content.
func TestReferenceXObject(t *testing.T) {
	bodies := map[string]string{
		"lost":      "BT /F1 12 Tf 20 100 Td (Secret) Tj 180 0 Td (Kept) Tj ET",
		"unchanged": "BT /F1 12 Tf 200 100 Td (Kept) Tj ET",
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			in := nestDoc(t, []pdf.Dict{{"Rect": rect(0, 0, 150, 300)}}, func(b *nestBuilder) []nestPage {
				ef := b.stream(pdf.Dict{"Type": pdf.Name("EmbeddedFile")}, "EmbeddedOriginal")
				fs := b.put(pdf.Dict{
					"Type": pdf.Name("Filespec"), "F": pdf.String("orig.pdf"),
					"EF": pdf.Dict{"F": ef},
				})
				fm := b.form(pdf.Dict{
					"Resources": helv(),
					"Ref":       pdf.Dict{"F": fs, "Page": pdf.Integer(0)},
				}, body)
				return []nestPage{{content: "/Fm Do", res: pdf.Dict{"XObject": pdf.Dict{"Fm": fm}}}}
			})
			out := writeAndRead(t, in, nil, "")
			if n := streamsContaining(t, out, "EmbeddedOriginal"); n != 0 {
				t.Errorf("%d streams hold the referenced file", n)
			}
			if n := streamsContaining(t, out, "Kept"); n == 0 {
				t.Error("the form lost the text outside the region")
			}
			page := pageAt(t, out, 0)
			cur := pdf.CursorAt(pdf.NewExtractor(out), nil)
			for _, n := range doNames(t, out, page) {
				stm, err := cur.Stream(xobject(t, out, page, n))
				if err != nil {
					t.Fatal(err)
				}
				if _, ok := stm.Dict["Ref"]; ok {
					t.Errorf("XObject %s keeps /Ref", n)
				}
			}
		})
	}
}

// checkMatrices checks that every /Matrix entry of the objects of r holds six
// numbers.
func checkMatrices(t testing.TB, r *pdf.Reader) {
	t.Helper()
	cur := pdf.CursorAt(pdf.NewExtractor(r), nil)
	for i := uint32(1); i < 200; i++ {
		obj, err := r.Get(pdf.NewReference(i, 0), true)
		if err != nil {
			continue
		}
		var d pdf.Dict
		switch o := obj.(type) {
		case *pdf.Stream:
			d = o.Dict
		case pdf.Dict:
			d = o
		}
		if m, ok := d["Matrix"]; ok {
			if _, err := cur.Matrix(m); err != nil {
				t.Errorf("object %d has the malformed matrix %v", i, m)
			}
		}
	}
}

// badMatrices are malformed /Matrix entries, which a reader takes as the
// identity.
var badMatrices = map[string]pdf.Array{
	"seven entries": {pdf.Number(0.5), pdf.Integer(0), pdf.Integer(0), pdf.Number(0.5), pdf.Integer(0), pdf.Integer(0), pdf.Integer(0)},
	"string entry":  {pdf.Number(0.5), pdf.Integer(0), pdf.Integer(0), pdf.Number(0.5), pdf.Integer(0), pdf.String("x")},
}

// TestMalformedMatrix checks that a form, pattern or soft-mask group with a
// malformed /Matrix is redacted as the identity matrix places it, and that
// the rewritten copy has a matrix which places the content in the same way,
// also where the stream loses no content.
func TestMalformedMatrix(t *testing.T) {
	for mname, m := range badMatrices {
		t.Run(mname+"/form", func(t *testing.T) {
			build := singleForm(t, "q 1 0 0 1 20 100 cm /Fm1 Do Q", pdf.Dict{"Matrix": m}, secretText)
			checkMatrices(t, checkNested(t, build, "Secret"))
		})
		t.Run(mname+"/unchanged form", func(t *testing.T) {
			build := singleForm(t, "q 1 0 0 1 20 100 cm /Fm1 Do Q", pdf.Dict{"Matrix": m}, secretText)
			in := build([]pdf.Dict{{"Rect": rect(0, 250, 300, 300)}})
			out := writeAndRead(t, in, nil, "")
			checkMatrices(t, out)
			if got, want := glyphString(pageGlyphs(t, out)), glyphString(pageGlyphs(t, in)); got != want {
				t.Errorf("got text %q, want %q", got, want)
			}
		})
		t.Run(mname+"/pattern", func(t *testing.T) {
			in := patternDoc(t, []pdf.Dict{{"Rect": rect(41, 25, 47, 31)}}, "0 0 200 200 re", pdf.Dict{"Matrix": m})
			out := writeAndRead(t, in, nil, "")
			checkMatrices(t, out)
			if n := streamsContaining(t, out, "Secret"); n != 0 {
				t.Errorf("%d streams still hold the pattern text", n)
			}
		})
		t.Run(mname+"/soft mask", func(t *testing.T) {
			in := nestDoc(t, []pdf.Dict{{"Rect": rect(10, 250, 40, 290)}}, func(b *nestBuilder) []nestPage {
				g := b.form(pdf.Dict{"BBox": rect(0, 0, 250, 50), "Resources": helv(), "Matrix": m}, secretText)
				return []nestPage{{
					content: "q 1 0 0 1 20 100 cm /GS1 gs 0 0 200 100 re f Q",
					res: pdf.Dict{"ExtGState": pdf.Dict{"GS1": pdf.Dict{
						"SMask": pdf.Dict{"S": pdf.Name("Luminosity"), "G": g},
					}}},
				}}
			})
			out := writeAndRead(t, in, nil, "")
			checkMatrices(t, out)
			checkNames(t, out, pageAt(t, out, 0))
		})
		t.Run(mname+"/shading pattern", func(t *testing.T) {
			in := nestDoc(t, []pdf.Dict{{"Rect": rect(10, 250, 40, 290)}}, func(b *nestBuilder) []nestPage {
				pat := shadingPattern(b, m, secretText)
				return []nestPage{{
					content: "/Pattern cs /P0 scn 0 0 300 300 re f",
					res:     pdf.Dict{"Pattern": pdf.Dict{"P0": pat}},
				}}
			})
			out := writeAndRead(t, in, nil, "")
			checkMatrices(t, out)
			checkNames(t, out, pageAt(t, out, 0))
		})
	}
}

// TestTilingPatternInvalidCell checks that a tiling pattern whose cell
// cannot be placed, and which paints under a region, is replaced by a valid
// pattern without content.
func TestTilingPatternInvalidCell(t *testing.T) {
	cases := map[string]pdf.Dict{
		"no BBox":     {"BBox": nil},
		"zero XStep":  {"XStep": pdf.Integer(0)},
		"string step": {"YStep": pdf.String("x")},
	}
	for name, d := range cases {
		t.Run(name, func(t *testing.T) {
			in := patternDoc(t, []pdf.Dict{{"Rect": rect(41, 25, 47, 31)}}, "0 0 200 200 re", d)
			out := writeAndRead(t, in, nil, "")
			if n := streamsContaining(t, out, "Secret"); n != 0 {
				t.Errorf("%d streams still hold the pattern text", n)
			}
			cur := pdf.CursorAt(pdf.NewExtractor(out), nil)
			res := resolveDict(t, out, pageAt(t, out, 0)["Resources"])
			stm, err := cur.Stream(path(t, out, res, "Pattern", "P1"))
			if err != nil || stm == nil {
				t.Fatalf("pattern P1 is missing: %v", err)
			}
			pd := stm.Dict
			if _, err := cur.Rectangle(pd["BBox"]); err != nil || pd["BBox"] == nil {
				t.Errorf("invalid /BBox %v", pd["BBox"])
			}
			for _, key := range []pdf.Name{"XStep", "YStep"} {
				if v, err := cur.Number(pd[key]); err != nil || v == 0 {
					t.Errorf("invalid /%s %v", key, pd[key])
				}
			}
			for key, valid := range map[pdf.Name][]pdf.Integer{
				"PatternType": {1}, "PaintType": {1, 2}, "TilingType": {1, 2, 3},
			} {
				if v, err := cur.Integer(pd[key]); err != nil || !slices.Contains(valid, v) {
					t.Errorf("invalid /%s %v", key, pd[key])
				}
			}
		})
	}
}
