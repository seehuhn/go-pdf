// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"bytes"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/page"
	"seehuhn.de/go/pdf/reader"
)

// glyph is a character as shown on the page.
type glyph struct {
	text string
	code font.Code

	// x and y give the glyph origin, and cx and cy the centre of the
	// glyph box, in page space
	x, y, cx, cy float64
}

// textSource builds a document with a single page.  The page has the
// content, the font /F1 (Helvetica) and the given annotations.
func textSource(t testing.TB, contentData string, annots ...pdf.Dict) *pdf.Reader {
	t.Helper()
	return textSourceOpt(t, contentData, nil, annots...)
}

// srcOpts adds resources to the page built by [textSourceOpt].
type srcOpts struct {
	// fonts are added to the font resources, under the given names
	fonts map[pdf.Name]font.Instance

	// rawFonts are font dictionaries added to the font resources
	rawFonts map[pdf.Name]pdf.Dict

	// gsFont, if set, adds the extended graphics state /G1, which sets the
	// font (one of fonts) and the font size gsSize
	gsFont pdf.Name
	gsSize float64

	// gsHide leaves gsFont out of the font resources
	gsHide bool

	// props are property lists, added as indirect objects to the
	// properties resources
	props map[pdf.Name]pdf.Dict
}

// textSourceOpt is like [textSource], with additional resources.
func textSourceOpt(t testing.TB, contentData string, o *srcOpts, annots ...pdf.Dict) *pdf.Reader {
	t.Helper()
	if o == nil {
		o = &srcOpts{}
	}
	w, buf := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	pagesRef, pageRef := w.Alloc(), w.Alloc()
	fontRef := w.Alloc()
	if err := w.Put(fontRef, pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": pdf.Name("Helvetica"),
	}); err != nil {
		t.Fatal(err)
	}
	fontRes := pdf.Dict{"F1": fontRef}
	for name, d := range o.rawFonts {
		ref := w.Alloc()
		if err := w.Put(ref, d); err != nil {
			t.Fatal(err)
		}
		fontRes[name] = ref
	}
	rm := pdf.NewResourceManager(w)
	for name, f := range o.fonts {
		ref, err := rm.Embed(f)
		if err != nil {
			t.Fatal(err)
		}
		fontRes[name] = ref
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	resources := pdf.Dict{"Font": fontRes}
	if len(o.props) > 0 {
		propRes := pdf.Dict{}
		for name, d := range o.props {
			ref := w.Alloc()
			if err := w.Put(ref, d); err != nil {
				t.Fatal(err)
			}
			propRes[name] = ref
		}
		resources["Properties"] = propRes
	}
	if o.gsFont != "" {
		resources["ExtGState"] = pdf.Dict{"G1": pdf.Dict{
			"Type": pdf.Name("ExtGState"),
			"Font": pdf.Array{fontRes[o.gsFont], pdf.Number(o.gsSize)},
		}}
		if o.gsHide {
			delete(fontRes, o.gsFont)
		}
	}

	contentRef := w.Alloc()
	stm, err := w.OpenStream(contentRef, pdf.Dict{}, pdf.FilterCompress{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stm.Write([]byte(contentData)); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}

	var annotArray pdf.Array
	for _, a := range annots {
		a["Type"] = pdf.Name("Annot")
		a["Subtype"] = pdf.Name("Redact")
		a["P"] = pageRef
		ref := w.Alloc()
		if err := w.Put(ref, a); err != nil {
			t.Fatal(err)
		}
		annotArray = append(annotArray, ref)
	}

	if err := w.Put(pagesRef, pdf.Dict{
		"Type": pdf.Name("Pages"), "Count": pdf.Integer(1), "Kids": pdf.Array{pageRef},
	}); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(pageRef, pdf.Dict{
		"Type": pdf.Name("Page"), "Parent": pagesRef,
		"MediaBox":  rect(0, 0, 300, 300),
		"Resources": resources,
		"Contents":  contentRef,
		"Annots":    annotArray,
	}); err != nil {
		t.Fatal(err)
	}
	w.GetMeta().Catalog.Pages = pagesRef
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	r := openSource(t, buf.Data)
	return r
}

// pageGlyphs returns the characters shown on the first page, including
// invisible ones.
func pageGlyphs(t testing.TB, r *pdf.Reader) []glyph {
	t.Helper()
	x := pdf.NewExtractor(r)
	refs, _ := pageDicts(t, r)
	pg, err := pdf.CursorAt(x, nil).Decode(refs[0], page.Decode)
	if err != nil {
		t.Fatal(err)
	}

	var res []glyph
	active := map[*form.Form]bool{}
	var install func(rd *reader.Reader)
	install = func(rd *reader.Reader) {
		rd.Character = func(c font.Code) error {
			gs := rd.State.GState
			m := gs.GlyphMatrix(&c).Mul(gs.CTM)
			ctr := m.Apply(vec2(c.Width/2, 0.3))
			res = append(res, glyph{
				text: c.Text, code: c,
				x: m[4], y: m[5], cx: ctr.X, cy: ctr.Y,
			})
			return nil
		}
		rd.XObject = func(obj graphics.XObject, ctm matrix.Matrix) error {
			f, ok := obj.(*form.Form)
			if !ok || active[f] {
				return nil
			}
			active[f] = true
			defer delete(active, f)
			fres := f.Res
			if fres == nil {
				fres = rd.State.Resources
			}
			sub := reader.New(x)
			sub.State = content.NewState(content.Form, fres)
			m := f.Matrix
			if m.IsZero() {
				m = matrix.Identity
			}
			sub.State.GState.CTM = m.Mul(ctm)
			install(sub)
			return sub.ProcessIter(f.Content.NewIter())
		}
	}
	rd := reader.New(x)
	install(rd)
	if err := rd.ProcessPage(pg); err != nil {
		t.Fatal(err)
	}
	return res
}

func glyphString(gs []glyph) string {
	var sb strings.Builder
	for _, g := range gs {
		sb.WriteString(g.text)
	}
	return sb.String()
}

// regionOver returns a redaction annotation which covers the centres of the
// given glyphs, and not much more.
func regionOver(gs []glyph) pdf.Dict {
	const margin = 0.3
	llx, lly := math.Inf(1), math.Inf(1)
	urx, ury := math.Inf(-1), math.Inf(-1)
	for _, g := range gs {
		llx, lly = math.Min(llx, g.cx), math.Min(lly, g.cy)
		urx, ury = math.Max(urx, g.cx), math.Max(ury, g.cy)
	}
	return pdf.Dict{"Rect": pdf.Array{
		pdf.Number(pdf.Round(llx-margin, 3)), pdf.Number(pdf.Round(lly-margin, 3)),
		pdf.Number(pdf.Round(urx+margin, 3)), pdf.Number(pdf.Round(ury+margin, 3)),
	}}
}

// checkTextRemoval redacts the first occurrence of the text secret from a
// page with the given content, and checks the contract of text removal.
func checkTextRemoval(t testing.TB, contentData, secret string) {
	t.Helper()
	checkTextRemovalOpt(t, contentData, nil, secret, []byte(secret))
}

// checkTextRemovalOpt is like [checkTextRemoval], for a page with additional
// resources.  The codes secretCodes must not occur in any string of the
// rewritten content.
func checkTextRemovalOpt(t testing.TB, contentData string, o *srcOpts, secret string, secretCodes []byte) {
	t.Helper()
	in := textSourceOpt(t, contentData, o)
	before := pageGlyphs(t, in)
	pos := strings.Index(glyphString(before), secret)
	if pos < 0 {
		t.Fatalf("test page does not show %q (got %q)", secret, glyphString(before))
	}
	from, to := pos, pos+len(secret)

	in = textSourceOpt(t, contentData, o, regionOver(before[from:to]))
	out := writeAndRead(t, in, nil, "")
	after := pageGlyphs(t, out)

	// the secret is gone from the extracted text
	if strings.Contains(glyphString(after), secret) {
		t.Errorf("extracted text still contains %q", secret)
	}
	// ... and from the content
	_, dicts := pageDicts(t, out)
	if bytes.Contains(contentBytes(t, out, dicts[0]), secretCodes) {
		t.Errorf("content still contains %q", secret)
	}
	for _, op := range pageOps(t, out, dicts[0]) {
		for _, arg := range flattenStrings(op.args) {
			if bytes.Contains(arg, secretCodes) {
				t.Errorf("operator %s still shows %q", op.name, secret)
			}
		}
	}

	// all other glyphs are where they were
	want := append(append([]glyph{}, before[:from]...), before[to:]...)
	if len(after) != len(want) {
		t.Fatalf("got text %q, want %q", glyphString(after), glyphString(want))
	}
	for i, g := range after {
		w := want[i]
		if g.text != w.text || g.code.CID != w.code.CID ||
			math.Abs(g.x-w.x) > 1e-3 || math.Abs(g.y-w.y) > 1e-3 {
			t.Errorf("glyph %d: got %q at (%g, %g), want %q at (%g, %g)",
				i, g.text, g.x, g.y, w.text, w.x, w.y)
		}
	}
}

// flattenStrings returns the strings among the operands, including those in
// arrays.
func flattenStrings(args []pdf.Object) [][]byte {
	var res [][]byte
	for _, a := range args {
		switch a := a.(type) {
		case pdf.String:
			res = append(res, []byte(a))
		case pdf.Array:
			res = append(res, flattenStrings(a)...)
		}
	}
	return res
}

func TestRemoveText(t *testing.T) {
	cases := []struct {
		name, content, secret string
	}{
		{"Tj", "BT /F1 12 Tf 50 100 Td (Hello Secret World) Tj ET", "Secret"},
		{"Tj at start", "BT /F1 12 Tf 50 100 Td (Secret World) Tj ET", "Secret"},
		{"Tj at end", "BT /F1 12 Tf 50 100 Td (Hello Secret) Tj ET", "Secret"},
		{"Tj only", "BT /F1 12 Tf 50 100 Td (Secret) Tj (After) Tj ET", "Secret"},
		{"TJ", "BT /F1 12 Tf 50 100 Td [(Hel) -120 (lo Se) 80 (cret) 20 (World)] TJ ET", "Secret"},
		{"TJ number inside", "BT /F1 12 Tf 50 100 Td [(Hello Se) 300 (cr) -90.5 (et World)] TJ ET", "Secret"},
		{"TJ string removed", "BT /F1 12 Tf 50 100 Td [(Hello) 50 (Secret) -70 (World)] TJ ET", "Secret"},
		{"TJ two strings", "BT /F1 12 Tf 50 100 Td [(Hello) 50 (Sec) -70 (ret) 20 (World)] TJ ET", "Secret"},
		{"quote", "BT /F1 12 Tf 14 TL 50 200 Td (First) Tj (Second secret) ' (Third) ' ET", "secret"},
		{"quote first", "BT /F1 12 Tf 14 TL 50 200 Td (First) Tj (secret) ' (Third) ' ET", "secret"},
		{"double quote", "BT /F1 12 Tf 14 TL 50 200 Td (First) Tj 3 2 (Another secret word) \" (Third) ' ET", "secret"},
		{"Tc", "BT /F1 12 Tf 50 100 Td 1.5 Tc (Hello Secret World) Tj ET", "Secret"},
		{"Tw", "BT /F1 12 Tf 50 100 Td 7 Tw (Hello Secret World) Tj ET", "Secret"},
		{"Tw inside", "BT /F1 12 Tf 50 100 Td 7 Tw (Hello Top Secret World) Tj ET", "Top Secret"},
		{"Tz", "BT /F1 12 Tf 50 100 Td 150 Tz (Hello Secret World) Tj ET", "Secret"},
		{"Tz TJ", "BT /F1 12 Tf 50 100 Td 60 Tz [(Hello S) 100 (ecret) -50 ( World)] TJ ET", "Secret"},
		{"Ts", "BT /F1 12 Tf 50 100 Td 4 Ts (Hello Secret World) Tj ET", "Secret"},
		{"all spacing", "BT /F1 12 Tf 50 100 Td 1 Tc 5 Tw 120 Tz 3 Ts [(Hello Top) -40 ( Secret ) 90 (World)] TJ ET", "Secret"},
		{"scaled CTM", "q 2 0 0 0.5 10 40 cm BT /F1 12 Tf 20 100 Td 1 Tc (Hello Secret World) Tj ET Q", "Secret"},
		{"rotated CTM", "q 0 1 -1 0 200 20 cm BT /F1 12 Tf 0 50 Td 2 Tw (Hello Secret World) Tj ET Q", "Secret"},
		{"text matrix", "BT /F1 12 Tf 1 0 0.3 1 40 120 Tm (Hello Secret World) Tj ET", "Secret"},
		{"stroked", "0.01 w 1 Tr BT /F1 12 Tf 50 100 Td (Hello Secret World) Tj ET", "Secret"},
		{"two lines", "BT /F1 12 Tf 50 100 Td (Hello) Tj 0 -20 Td (Secret) Tj 0 -20 Td (World) Tj ET", "Secret"},
		{"two objects", "BT /F1 12 Tf 50 100 Td (Hello) Tj ET BT /F1 12 Tf 50 60 Td (Secret) Tj ET", "Secret"},
		{"split strings", "BT /F1 12 Tf 50 100 Td (Hello Sec) Tj (ret World) Tj ET", "Secret"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			checkTextRemoval(t, c.content, c.secret)
		})
	}
}

func TestRemoveTextSeveralRegions(t *testing.T) {
	const data = "BT /F1 12 Tf 50 100 Td [(Alpha) 100 (Beta) -80 (Gamma) 5 (Delta)] TJ ET"
	before := pageGlyphs(t, textSource(t, data))
	text := glyphString(before)
	i, j := strings.Index(text, "Beta"), strings.Index(text, "Delta")

	in := textSource(t, data, regionOver(before[i:i+4]), regionOver(before[j:j+5]))
	after := pageGlyphs(t, writeAndRead(t, in, nil, ""))
	if got, want := glyphString(after), "AlphaGamma"; got != want {
		t.Fatalf("got text %q, want %q", got, want)
	}
	gamma := strings.Index(text, "Gamma")
	for k, g := range after[5:] {
		w := before[gamma+k]
		if math.Abs(g.x-w.x) > 1e-3 || math.Abs(g.y-w.y) > 1e-3 {
			t.Errorf("glyph %q moved", g.text)
		}
	}
}

func TestKeepUntouchedText(t *testing.T) {
	const data = "BT /F1 12 Tf 50 100 Td [(Hello) -30 (World)] TJ (Next) ' 1 2 (More) \" ET"
	// a region which meets no text
	region := pdf.Dict{"Rect": rect(200, 200, 250, 250)}
	in := textSource(t, data, region)
	out := writeAndRead(t, in, nil, "")

	_, inDicts := pageDicts(t, in)
	_, outDicts := pageDicts(t, out)
	isText := func(op opRecord) bool {
		switch op.name {
		case content.OpTextBegin, content.OpTextEnd, content.OpTextSetFont,
			content.OpTextMoveOffset, content.OpTextShow, content.OpTextShowArray,
			content.OpTextShowMoveNextLine, content.OpTextShowMoveNextLineSetSpacing:
			return true
		}
		return false
	}
	var want, got []opRecord
	for _, op := range pageOps(t, in, inDicts[0]) {
		if isText(op) {
			want = append(want, op)
		}
	}
	for _, op := range pageOps(t, out, outDicts[0]) {
		if isText(op) {
			got = append(got, op)
		}
	}
	if diff := cmp.Diff(fmtOps(want), fmtOps(got)); diff != "" {
		t.Errorf("text operators changed (-want +got):\n%s", diff)
	}
}

func fmtOps(ops []opRecord) []string {
	var res []string
	for _, op := range ops {
		var sb strings.Builder
		_ = content.Operator{Name: op.name, Args: op.args}.Format(&sb, 0)
		res = append(res, sb.String())
	}
	return res
}

func TestTouchingRegion(t *testing.T) {
	const data = "BT /F1 12 Tf 50 100 Td (Hello World) Tj ET"
	before := pageGlyphs(t, textSource(t, data))

	// Helvetica has ascent 718 and descent -207
	top := 100 + 0.718*12
	bottom := 100 - 0.207*12
	last := before[len(before)-1]
	cases := []struct {
		name string
		rect pdf.Array
	}{
		{"above", pdf.Array{pdf.Integer(0), pdf.Number(pdf.Round(top, 3)), pdf.Integer(300), pdf.Integer(200)}},
		{"below", pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(300), pdf.Number(pdf.Round(bottom, 3))}},
		{"left", pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Number(pdf.Round(before[0].x, 3)), pdf.Integer(300)}},
		{"right", pdf.Array{pdf.Number(pdf.Round(last.x+last.code.Width*12, 3)), pdf.Integer(0), pdf.Integer(300), pdf.Integer(300)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := textSource(t, data, pdf.Dict{"Rect": c.rect})
			after := pageGlyphs(t, writeAndRead(t, in, nil, ""))
			if got := glyphString(after); got != "Hello World" {
				t.Errorf("got text %q, want everything", got)
			}
		})
	}
}

func TestStrokedTextReach(t *testing.T) {
	// the region lies just above the glyph boxes
	top := 100 + 0.718*12 + 0.5
	region := pdf.Dict{"Rect": pdf.Array{pdf.Integer(0), pdf.Number(pdf.Round(top, 3)), pdf.Integer(300), pdf.Integer(200)}}

	fill := textSource(t, "BT /F1 12 Tf 50 100 Td (Hello) Tj ET", region)
	if got := glyphString(pageGlyphs(t, writeAndRead(t, fill, nil, ""))); got != "Hello" {
		t.Errorf("filled text: got %q, want everything", got)
	}
	stroked := textSource(t, "2 w 1 Tr BT /F1 12 Tf 50 100 Td (Hello) Tj ET", region)
	if got := glyphString(pageGlyphs(t, writeAndRead(t, stroked, nil, ""))); got != "" {
		t.Errorf("stroked text: got %q, want nothing", got)
	}
}

func TestRemoveTextKeepsState(t *testing.T) {
	// the text state after the removed text is the same as before
	const data = "BT /F1 12 Tf 14 TL 50 200 Td 1 Tc 2 Tw (Secret) ' (Kept) Tj 5 7 (Secret) \" (After) Tj ET"
	before := pageGlyphs(t, textSource(t, data))
	text := glyphString(before)
	first := strings.Index(text, "Secret")
	second := strings.LastIndex(text, "Secret")

	in := textSource(t, data, regionOver(before[first:first+6]), regionOver(before[second:second+6]))
	after := pageGlyphs(t, writeAndRead(t, in, nil, ""))
	if got, want := glyphString(after), "KeptAfter"; got != want {
		t.Fatalf("got text %q, want %q", got, want)
	}
	var want []glyph
	want = append(want, before[first+6:second]...)
	want = append(want, before[second+6:]...)
	for i, g := range after {
		if math.Abs(g.x-want[i].x) > 1e-3 || math.Abs(g.y-want[i].y) > 1e-3 {
			t.Errorf("glyph %q is at (%g, %g), want (%g, %g)", g.text, g.x, g.y, want[i].x, want[i].y)
		}
	}
}

func TestRewriteDropsUnusableOperators(t *testing.T) {
	const data = "% comment\nBT /F1 12 Tf 50 100 Td (Hi) Tj ET\n1 2 foo\nQ\n0 g\n"
	in := textSource(t, data, pdf.Dict{"Rect": rect(200, 200, 250, 250)})
	out := writeAndRead(t, in, nil, "")
	_, dicts := pageDicts(t, out)
	for _, op := range pageOps(t, out, dicts[0]) {
		if op.name == "foo" || op.name == content.OpRawContent {
			t.Errorf("operator %q was kept", op.name)
		}
	}
}

func TestTextWithoutFont(t *testing.T) {
	// text shown without a font paints nothing and is not kept
	const data = "BT 50 100 Td (Nothing) Tj ET"
	in := textSource(t, data, pdf.Dict{"Rect": rect(0, 0, 10, 10)})
	out := writeAndRead(t, in, nil, "")
	_, dicts := pageDicts(t, out)
	for _, op := range pageOps(t, out, dicts[0]) {
		if op.name == content.OpTextShow {
			t.Error("text operator without a font was kept")
		}
	}
}

func vec2(x, y float64) vec.Vec2 { return vec.Vec2{X: x, Y: y} }

func TestRemoveInvisibleText(t *testing.T) {
	const layout = "BT /F1 12 Tf 50 100 Td (Hello Secret World) Tj ET"
	const data = "BT /F1 12 Tf 3 Tr 50 100 Td (Hello Secret) Tj 0 Tr ( World) Tj ET"

	before := pageGlyphs(t, textSource(t, layout))
	pos := strings.Index(glyphString(before), "Secret")
	in := textSource(t, data, regionOver(before[pos:pos+6]))
	out := writeAndRead(t, in, nil, "")

	_, dicts := pageDicts(t, out)
	if bytes.Contains(contentBytes(t, out, dicts[0]), []byte("Secret")) {
		t.Error("invisible text was kept")
	}
	after := pageGlyphs(t, out)
	if got := glyphString(after); got != " World" {
		t.Fatalf("got text %q, want %q", got, " World")
	}
	for i, g := range after {
		w := before[pos+6+i]
		if math.Abs(g.x-w.x) > 1e-3 || math.Abs(g.y-w.y) > 1e-3 {
			t.Errorf("glyph %q moved", g.text)
		}
	}
}

func TestRemoveTextDegenerateScaling(t *testing.T) {
	cases := []struct {
		name, content string
	}{
		{"zero scaling", "BT /F1 12 Tf 50 100 Td 0 Tz (Secret) Tj 100 Tz 40 0 Td (After) Tj ET"},
		{"zero font size", "BT 50 100 Td /F1 0 Tf (Secret) Tj /F1 12 Tf 40 0 Td (After) Tj ET"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := pageGlyphs(t, textSource(t, c.content))
			text := glyphString(before)
			if text != "SecretAfter" {
				t.Fatalf("got text %q", text)
			}
			in := textSource(t, c.content, regionOver(before[:6]))
			after := pageGlyphs(t, writeAndRead(t, in, nil, ""))
			if got := glyphString(after); got != "After" {
				t.Fatalf("got text %q, want %q", got, "After")
			}
			for i, g := range after {
				w := before[6+i]
				if math.Abs(g.x-w.x) > 1e-3 || math.Abs(g.y-w.y) > 1e-3 {
					t.Errorf("glyph %q moved", g.text)
				}
			}
		})
	}
}

// operandKinds gives the operands of the operators, following the operator
// summary of the PDF specification: n is a number, N a name, s a string, a
// an array, d a dictionary and p a dictionary or name.  Operators which are
// missing take no operands or are checked separately.
var operandKinds = map[content.OpName]string{
	content.OpSetLineWidth: "n", content.OpSetLineCap: "n", content.OpSetLineJoin: "n",
	content.OpSetMiterLimit: "n", content.OpSetFlatnessTolerance: "n",
	content.OpTextSetCharacterSpacing: "n", content.OpTextSetWordSpacing: "n",
	content.OpTextSetHorizontalScaling: "n", content.OpTextSetLeading: "n",
	content.OpTextSetRenderingMode: "n", content.OpTextSetRise: "n",
	content.OpSetFillGray: "n", content.OpSetStrokeGray: "n",
	content.OpMoveTo: "nn", content.OpLineTo: "nn", content.OpTextMoveOffset: "nn",
	content.OpTextMoveOffsetSetLeading: "nn",
	content.OpSetFillRGB:               "nnn", content.OpSetStrokeRGB: "nnn",
	content.OpSetFillCMYK: "nnnn", content.OpSetStrokeCMYK: "nnnn",
	content.OpCurveToV: "nnnn", content.OpCurveToY: "nnnn", content.OpRectangle: "nnnn",
	content.OpCurveTo: "nnnnnn", content.OpTransform: "nnnnnn", content.OpTextSetMatrix: "nnnnnn",
	content.OpSetRenderingIntent: "N", content.OpSetExtGState: "N", content.OpXObject: "N",
	content.OpShading: "N", content.OpBeginMarkedContent: "N", content.OpMarkedContentPoint: "N",
	content.OpSetFillColorSpace: "N", content.OpSetStrokeColorSpace: "N",
	content.OpTextSetFont: "Nn", content.OpSetLineDash: "an",
	content.OpBeginMarkedContentWithProperties: "Np",
	content.OpMarkedContentPointWithProperties: "Np",
	content.OpTextShow:                         "s", content.OpTextShowMoveNextLine: "s",
	content.OpTextShowMoveNextLineSetSpacing: "nns", content.OpTextShowArray: "a",
}

// checkOperands checks that every operator has the operands which the
// specification gives for it, and that text objects are not nested.
func checkOperands(t testing.TB, ops []opRecord) {
	t.Helper()
	inText := false
	for _, op := range ops {
		switch op.name {
		case content.OpInlineImage, content.OpRawContent,
			content.OpSetFillColor, content.OpSetStrokeColor,
			content.OpSetFillColorN, content.OpSetStrokeColorN:
			continue
		case content.OpTextBegin:
			if inText {
				t.Error("nested BT")
			}
			inText = true
		case content.OpTextEnd:
			if !inText {
				t.Error("unbalanced ET outside a text object")
			}
			inText = false
		}
		kinds := operandKinds[op.name]
		if len(op.args) != len(kinds) {
			t.Errorf("%s has %d operands, want %d", op.name, len(op.args), len(kinds))
			continue
		}
		for i, k := range []byte(kinds) {
			var ok bool
			switch k {
			case 'n':
				_, ok = getNumber(op.args[i])
			case 'N':
				_, ok = op.args[i].(pdf.Name)
			case 's':
				_, ok = op.args[i].(pdf.String)
			case 'a':
				_, ok = op.args[i].(pdf.Array)
			case 'p':
				switch op.args[i].(type) {
				case pdf.Dict, pdf.Name:
					ok = true
				}
			}
			if !ok {
				t.Errorf("operand %d of %s is %v", i, op.name, op.args[i])
			}
		}
		if op.name == content.OpTextShowArray {
			for _, x := range op.args[0].(pdf.Array) {
				_, isNum := getNumber(x)
				if _, isStr := x.(pdf.String); !isStr && !isNum {
					t.Errorf("array of TJ holds %v", x)
				}
			}
		}
	}
}

// TestRewriteUsesInterpretedOperands checks that the rewritten content
// holds only operators as the analysis read them: an operator with surplus
// or malformed operands, or one which is out of place, cannot be read
// differently by a viewer.
func TestRewriteUsesInterpretedOperands(t *testing.T) {
	cases := []struct {
		name, content string
		region        pdf.Array
	}{
		{"surplus Tm", "BT /F1 12 Tf 1 0 0 1 10 10 1 0 0 1 100 150 Tm (Secret) Tj ET", rect(95, 145, 150, 165)},
		{"malformed Tf", "BT /F1 1 Tf /F2 /F1 12 Tf 100 150 Td (Secret) Tj ET", rect(110, 150, 150, 165)},
		{"nested BT", "BT /F1 12 Tf 100 150 Td BT (Secret) Tj ET ET", rect(0, 0, 50, 20)},
		{"surplus Tr", "BT /F1 12 Tf 100 150 Td 1 2 Tr 1.6 Tr 9 Tr (Secret) Tj ET", rect(0, 0, 20, 20)},
		{"junk in TJ", "BT /F1 12 Tf 100 150 Td [(Se) [(Inner)] /N (cret)] TJ ET", rect(0, 0, 20, 20)},
		{"surplus path operands", "1 2 3 4 0 0 m 50 50 l 1 2 0 0 10 10 re f", rect(200, 200, 250, 250)},
		{"stray operators", "Q ET EMC EX 1 2 W BT /F1 12 Tf (Hi) Tj ET", rect(200, 200, 250, 250)},
		{"colour operands", "1 0 0 0.5 rg 0.1 0.2 /X g /Pattern cs /P1 /P2 scn 0.5 /P1 sc 0 0 1 1 re f", rect(200, 200, 250, 250)},
		{"text in path context", "10 10 m BT /F1 12 Tf 100 150 Td (Secret) Tj ET", rect(200, 200, 250, 250)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := textSource(t, c.content, pdf.Dict{"Rect": c.region})
			out := writeAndRead(t, in, nil, "")
			_, dicts := pageDicts(t, out)
			checkOperands(t, pageOps(t, out, dicts[0]))
		})
	}
}

// TestRewriteDropsMalformedDo checks that a Do operator with surplus
// operands paints, in the output, the XObject which the analysis saw.
func TestRewriteDropsMalformedDo(t *testing.T) {
	in := singleForm(t, "q 1 0 0 1 20 100 cm /Nope /Fm1 Do Q", nil, secretText)(
		[]pdf.Dict{{"Rect": rect(0, 0, 300, 300)}})
	out := writeAndRead(t, in, nil, "")
	ops := pageOps(t, out, pageAt(t, out, 0))
	checkOperands(t, ops)
	for _, op := range ops {
		if op.name == content.OpXObject && slices.Contains(op.args, pdf.Object(pdf.Name("Fm1"))) {
			t.Error("the page paints the form which the analysis did not see")
		}
	}
	checkStable(t, out)
}

// TestRewriteDropsMalformedBDC checks that a property list which the
// analysis does not read as the one of a span is not kept.
func TestRewriteDropsMalformedBDC(t *testing.T) {
	const data = "/Span /X <</ActualText (secretAT)>> BDC BT /F1 12 Tf 50 100 Td (Hello) Tj ET EMC"
	out, raw := redactText(t, data, nil, "Hello")
	if bytes.Contains(raw, []byte("secretAT")) {
		t.Error("output holds the ActualText of the span")
	}
	_, dicts := pageDicts(t, out)
	checkOperands(t, pageOps(t, out, dicts[0]))
	checkStable(t, out)
}

// TestRewriteClampedOperands checks that a graphics state parameter which
// the analysis clamps is written with the value which the analysis uses.
func TestRewriteClampedOperands(t *testing.T) {
	const data = "-3 w 0.5 M 7 J 1.6 j 9 Tr 0 0 10 10 re S"
	in := textSource(t, data, pdf.Dict{"Rect": rect(200, 200, 250, 250)})
	out := writeAndRead(t, in, nil, "")
	_, dicts := pageDicts(t, out)
	ops := pageOps(t, out, dicts[0])
	checkOperands(t, ops)
	checkStable(t, out)

	// stroked returns the graphics state of the first stroke.  If check is
	// set, it also checks that every operand of the operators in question
	// is the value which the state stores.
	stroked := func(ops []opRecord, check bool) *graphics.State {
		t.Helper()
		state := content.NewState(content.Page, nil)
		for _, op := range ops {
			_ = state.ApplyStateChanges(op.name, op.args)
			gs := state.GState
			var stored float64
			switch op.name {
			case content.OpStroke:
				return gs.Clone()
			case content.OpSetLineWidth:
				stored = gs.LineWidth
			case content.OpSetMiterLimit:
				stored = gs.MiterLimit
			case content.OpSetLineCap:
				stored = float64(gs.LineCap)
			case content.OpSetLineJoin:
				stored = float64(gs.LineJoin)
			case content.OpTextSetRenderingMode:
				stored = float64(gs.TextRenderingMode)
			default:
				continue
			}
			if v, _ := getNumber(op.args[0]); check && v != stored {
				t.Errorf("%s %v sets the value %g", op.name, op.args, stored)
			}
		}
		t.Fatal("no stroke")
		return nil
	}
	want := stroked(scanOps(t, []byte(data)), false)
	got := stroked(ops, true)
	if got.LineWidth != want.LineWidth || got.MiterLimit != want.MiterLimit ||
		got.LineCap != want.LineCap || got.LineJoin != want.LineJoin ||
		got.TextRenderingMode != want.TextRenderingMode {
		t.Error("the rewritten content sets other parameters than the analysis used")
	}
}

// TestRewriteNesting checks that the rewritten content is properly nested,
// has no q or Q in a text object, and cannot pop the graphics state which
// holds the clip of the regions.
func TestRewriteNesting(t *testing.T) {
	cases := []struct{ name, content string }{
		{"Q underflow", "Q Q 0 0 1 rg 0 0 300 300 re f"},
		{"ET and EMC underflow", "ET EMC 0 0 300 300 re f"},
		{"q in text", "q BT /F1 12 Tf 50 100 Td q (Hello) Tj Q ET Q 0 0 300 300 re f"},
		{"Q in text", "q BT /F1 12 Tf 50 100 Td (Hello) Tj Q ET 0 0 300 300 re f"},
		{"ET across q", "BT /F1 12 Tf 50 100 Td q (Hello) Tj ET Q 0 0 300 300 re f"},
		{"ET across EMC", "BT /F1 12 Tf 50 100 Td /X BMC (Secret) Tj ET EMC 0 0 300 300 re f"},
		{"EMC across q", "/X BMC q EMC Q 0 0 300 300 re f"},
		{"Q across EMC", "q /X BMC 0 0 1 rg Q EMC 0 0 300 300 re f"},
		{"Q across BDC", "q /P <</MCID 0>> BDC BT /F1 12 Tf 50 100 Td (Secret) Tj ET Q EMC"},
		{"rejected BMC", "q 5 BMC Q EMC 0 0 300 300 re f"},
		{"rejected BMC across q", "/X BMC 5 BMC q EMC Q EMC 0 0 300 300 re f"},
		{"rejected BMC closed by Q", "q /X BMC 5 BMC Q EMC EMC 0 0 300 300 re f"},
		{"compatibility section", "BX q EX Q 0 0 300 300 re f"},
		{"mixed", "Q Q ET EMC BX (Secret) foo EX q BT /F1 12 Tf 50 100 Td BT (Secret) Tj Q ET /X BMC q EMC Q 0 0 1 rg 0 0 200 200 re f"},
	}
	region := pdf.Dict{"Rect": rect(40, 90, 160, 120)}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := writeAndRead(t, textSource(t, c.content, region), nil, "")
			_, dicts := pageDicts(t, out)
			ops := pageOps(t, out, dicts[0])
			checkOperands(t, ops)
			checkNesting(t, ops)
			checkStable(t, out)
			if bytes.Contains(contentBytes(t, out, dicts[0]), []byte("Secret")) {
				t.Error("the content shows the text under the region")
			}
		})
	}
}

// checkNesting checks that the content of a page with regions is properly
// nested and has no q or Q in a text object, and that the operators before
// the marks run inside the graphics state which the first operator saves.
func checkNesting(t testing.TB, ops []opRecord) {
	t.Helper()
	depth := checkBalanced(t, ops)
	marks := slices.IndexFunc(ops, func(op opRecord) bool {
		return op.name == content.OpBeginMarkedContentWithProperties && op.args[0] == pdf.Name("Artifact")
	})
	if len(ops) == 0 || ops[0].name != content.OpPushGraphicsState || marks < 2 ||
		ops[marks-1].name != content.OpPopGraphicsState {
		t.Fatal("the content is not wrapped in q and Q")
	}
	inText := false
	for i, op := range ops[:marks-1] {
		if i > 0 && depth[i] == 1 && op.name == content.OpPopGraphicsState {
			t.Errorf("operator %d restores the graphics state of the clip", i)
		}
		switch op.name {
		case content.OpTextBegin:
			inText = true
		case content.OpTextEnd:
			inText = false
		case content.OpPushGraphicsState, content.OpPopGraphicsState:
			if inText {
				t.Errorf("operator %d: %s in a text object", i, op.name)
			}
		}
	}
}

// TestRewriteNestingPDF2 checks that q and Q in a text object, which PDF 2.0
// allows, are kept, and restore the text matrix.
func TestRewriteNestingPDF2(t *testing.T) {
	const data = "BT /F1 12 Tf 50 100 Td q (Hello) Tj Q (World) Tj ET"
	build := func(regs []pdf.Dict) *pdf.Reader {
		w, buf := memfile.NewPDFWriter(t, pdf.V2_0, nil)
		return nestDocWith(t, w, buf, regs, func(b *nestBuilder) []nestPage {
			return []nestPage{{content: data, res: helv()}}
		})
	}
	before := pageGlyphs(t, build(nil))
	out := writeAndRead(t, build([]pdf.Dict{{"Rect": rect(200, 200, 250, 250)}}), nil, "")
	after := pageGlyphs(t, out)
	if len(after) != len(before) {
		t.Fatalf("got %q, want %q", glyphString(after), glyphString(before))
	}
	for i, g := range after {
		if w := before[i]; g.text != w.text || math.Abs(g.x-w.x) > 1e-3 || math.Abs(g.y-w.y) > 1e-3 {
			t.Errorf("glyph %d: got %q at (%g, %g), want %q at (%g, %g)", i, g.text, g.x, g.y, w.text, w.x, w.y)
		}
	}
	ops := pageOps(t, out, pageAt(t, out, 0))
	checkBalanced(t, ops)
	n := 0
	for _, op := range ops {
		if op.name == content.OpPushGraphicsState {
			n++
		}
	}
	if n != 2 {
		t.Errorf("got %d q operators, want 2", n)
	}
}

// TestRewriteDeepNesting checks that deeply nested content is rewritten in
// time linear in its length.
func TestRewriteDeepNesting(t *testing.T) {
	const n = 100000
	region := pdf.Dict{"Rect": rect(40, 90, 160, 120)}
	cases := []struct{ name, content string }{
		{"q", strings.Repeat("q\n", n) + strings.Repeat("Q\n", n)},
		{"Q underflow", strings.Repeat("/X BMC\n", n) + strings.Repeat("Q\n", n)},
		{"spans", strings.Repeat("/X BMC\n", n/2) + strings.Repeat("50 100 m 60 100 l S\n", n/2)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := writeAndRead(t, textSource(t, c.content, region), nil, "")
			checkBalanced(t, pageOps(t, out, pageAt(t, out, 0)))
		})
	}
}

// TestRewriteUnknownOperators checks that unknown operators, which can carry
// strings, are removed together with their compatibility sections.
func TestRewriteUnknownOperators(t *testing.T) {
	const data = "BX (SecretA) foo /N [(SecretB)] <</K (SecretC)>> bar EX 0 0 10 10 re f"
	out := writeAndRead(t, textSource(t, data, pdf.Dict{"Rect": rect(200, 200, 250, 250)}), nil, "")
	_, dicts := pageDicts(t, out)
	if bytes.Contains(contentBytes(t, out, dicts[0]), []byte("Secret")) {
		t.Error("the content holds the operands of unknown operators")
	}
	for _, op := range pageOps(t, out, dicts[0]) {
		if op.name == content.OpBeginCompatibility || op.name == content.OpEndCompatibility {
			t.Errorf("the content holds %s", op.name)
		}
	}
	checkStable(t, out)
}

// TestFormImageRedacted checks that an image which a form paints under a
// region is replaced by a redacted copy, which keeps the samples outside the
// region.
func TestFormImageRedacted(t *testing.T) {
	data := secretRGB(imgW, imgH, inRegion)
	in := nestDoc(t, []pdf.Dict{regionDict()}, func(b *nestBuilder) []nestPage {
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

	// the first row of the image lies outside the region
	row := data[:3*imgW]
	found := false
	for _, s := range fileStreams(t, out) {
		found = found || bytes.Contains(s, row)
	}
	if !found {
		t.Error("the samples outside the region are lost")
	}
}

// TestPartlyHiddenPathLosesAlt checks that a span whose path meets a region,
// without being removed, loses its alternate text.
func TestPartlyHiddenPathLosesAlt(t *testing.T) {
	region := pdf.Dict{"Rect": rect(100, 90, 120, 140)}
	pageContent := func(data string) []byte {
		out := writeAndRead(t, textSource(t, data, region), nil, "")
		_, dicts := pageDicts(t, out)
		return contentBytes(t, out, dicts[0])
	}
	cases := []struct{ name, content string }{
		{"fill", "/Figure <</Alt (secretAlt)>> BDC 90 100 50 100 re f EMC"},
		{"stroke", "/Figure <</Alt (secretAlt)>> BDC 4 w 90 100 m 98 100 l S EMC"},
		{"shading", "/Figure <</Alt (secretAlt)>> BDC /Sh0 sh EMC"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if bytes.Contains(pageContent(c.content), []byte("secretAlt")) {
				t.Error("the span keeps its alternate text")
			}
		})
	}

	// a path clear of the regions keeps it
	if !bytes.Contains(pageContent("/Figure <</Alt (keptAlt)>> BDC 10 10 20 20 re f EMC"), []byte("keptAlt")) {
		t.Error("a span clear of the regions lost its alternate text")
	}
}

// TestIndirectMCID checks that an indirect /MCID entry of a named property
// list identifies the structure element of the removed content, and that
// an entry which does not give an identifier makes every element lose its
// text.
func TestIndirectMCID(t *testing.T) {
	for _, valid := range []bool{true, false} {
		in := structDoc(t, func(p *structParts) {
			mcid := p.alloc()
			if valid {
				p.extra[mcid] = pdf.Integer(0)
			} else {
				p.extra[mcid] = pdf.String("zero")
			}
			p.content = "/P /MC0 BDC BT /F1 12 Tf 10 20 Td (secret) Tj ET EMC\n" +
				"/P <</MCID 1>> BDC BT /F1 12 Tf 10 250 Td (keep) Tj ET EMC\n"
			p.props = pdf.Dict{"MC0": pdf.Dict{"MCID": mcid}}
		})
		out, _ := redactStruct(t, in)
		checkStripped(t, "element of the removed text", outElem(t, out, 0, 0))
		kept := outElem(t, out, 0, 1)
		_, hasText := kept["ActualText"]
		if valid && !hasText {
			t.Error("valid identifier: the element of the kept text lost its text")
		}
		if !valid && hasText {
			t.Error("invalid identifier: the element of the kept text keeps its text")
		}
	}
}

// redactText redacts the first occurrence of the text secret on the page
// and returns the output together with its bytes.  The output is written in
// the human-readable form.
func redactText(t testing.TB, data string, o *srcOpts, secret string) (*pdf.Reader, []byte) {
	t.Helper()
	in := textSourceOpt(t, data, o)
	before := pageGlyphs(t, in)
	pos := bytes.Index([]byte(glyphString(before)), []byte(secret))
	if pos < 0 {
		t.Fatalf("test page does not show %q", secret)
	}
	in = textSourceOpt(t, data, o, regionOver(before[pos:pos+len(secret)]))

	var buf bytes.Buffer
	err := Write(&buf, in, &Options{Writer: &pdf.WriterOptions{HumanReadable: true}})
	if err != nil {
		t.Fatal(err)
	}
	out := openPDF(t, buf.Bytes())
	return out, buf.Bytes()
}

// bdcArgs returns the operands of the BDC operators of the first page.
func bdcArgs(t testing.TB, r *pdf.Reader) [][]pdf.Object {
	t.Helper()
	_, dicts := pageDicts(t, r)
	var res [][]pdf.Object
	for _, op := range pageOps(t, r, dicts[0]) {
		if op.name == content.OpBeginMarkedContentWithProperties {
			res = append(res, op.args)
		}
	}
	return res
}

// bdcSpans returns the BDC operators of the page which carry the tag.
func bdcSpans(t testing.TB, r *pdf.Reader, tag pdf.Name) [][]pdf.Object {
	t.Helper()
	var res [][]pdf.Object
	for _, args := range bdcArgs(t, r) {
		if len(args) >= 2 && args[0] == tag {
			res = append(res, args)
		}
	}
	return res
}

func TestMarkedContentStripped(t *testing.T) {
	const data = "/Span <</ActualText (secretA) /Alt (secretB) /E (secretC) /Phoneme (secretD) /PhoneticAlphabet /ipa /MCID 3>> BDC\n" +
		"BT /F1 12 Tf 50 100 Td (Hello) Tj ET EMC\n" +
		"/Span <</ActualText (keepme) /MCID 4>> BDC\n" +
		"BT /F1 12 Tf 50 200 Td (World) Tj ET EMC"
	out, raw := redactText(t, data, nil, "Hello")

	if bytes.Contains(raw, []byte("secret")) {
		t.Error("output file still contains the text of the removed span")
	}
	_, dicts := pageDicts(t, out)
	if bytes.Contains(contentBytes(t, out, dicts[0]), []byte("secret")) {
		t.Error("content still contains the text of the removed span")
	}

	spans := bdcSpans(t, out, "Span")
	if len(spans) != 2 {
		t.Fatalf("got %d spans, want 2", len(spans))
	}
	first, second := spans[0][1].(pdf.Dict), spans[1][1].(pdf.Dict)
	for _, key := range []pdf.Name{"ActualText", "Alt", "E", "Phoneme"} {
		if _, ok := first[key]; ok {
			t.Errorf("span which lost text keeps /%s", key)
		}
	}
	if first["MCID"] != pdf.Integer(3) {
		t.Error("span which lost text lost its MCID")
	}
	if first["PhoneticAlphabet"] != pdf.Name("ipa") {
		t.Error("span which lost text lost its PhoneticAlphabet")
	}
	if _, ok := second["ActualText"]; !ok {
		t.Error("untouched span lost its ActualText")
	}
}

func TestMarkedContentNested(t *testing.T) {
	const data = "/P <</ActualText (secretOuter) /MCID 1>> BDC\n" +
		"/Span <</ActualText (secretInner) /MCID 2>> BDC\n" +
		"BT /F1 12 Tf 50 100 Td (Hello) Tj ET EMC\n" +
		"/Span <</ActualText (kept) /MCID 5>> BDC\n" +
		"BT /F1 12 Tf 50 200 Td (World) Tj ET EMC EMC"
	out, raw := redactText(t, data, nil, "Hello")

	if bytes.Contains(raw, []byte("secret")) {
		t.Error("output file still contains the text of a lost span")
	}
	spans := bdcSpans(t, out, "Span")
	if _, ok := spans[1][1].(pdf.Dict)["ActualText"]; !ok {
		t.Error("sibling span lost its ActualText")
	}
}

func TestMarkedContentNamedProperties(t *testing.T) {
	const data = "/Span /P0 BDC\n" +
		"BT /F1 12 Tf 50 100 Td (Hello) Tj ET EMC\n" +
		"/Span /P1 BDC\n" +
		"BT /F1 12 Tf 50 200 Td (World) Tj ET EMC\n" +
		"/OC /P2 BDC\n" +
		"BT /F1 12 Tf 50 150 Td (Other) Tj ET EMC"
	o := &srcOpts{props: map[pdf.Name]pdf.Dict{
		"P0": {"ActualText": pdf.String("secretA"), "MCID": pdf.Integer(7)},
		"P1": {"ActualText": pdf.String("kept"), "MCID": pdf.Integer(8)},
		"P2": {"Type": pdf.Name("OCG")},
	}}
	out, raw := redactText(t, data, o, "Hello")

	if bytes.Contains(raw, []byte("secret")) {
		t.Error("output file still contains the text of the removed span")
	}
	spans := bdcSpans(t, out, "Span")
	if len(spans) != 2 {
		t.Fatalf("got %d spans, want 2", len(spans))
	}
	n0, n1 := spans[0][1].(pdf.Name), spans[1][1].(pdf.Name)
	if n0 == "P0" || n1 != "P1" {
		t.Errorf("got property lists %s and %s, want a new name and P1", n0, n1)
	}
	if oc := bdcSpans(t, out, "OC"); len(oc) != 1 || oc[0][1] != pdf.Name("P2") {
		t.Error("optional content span was changed")
	}

	_, dicts := pageDicts(t, out)
	x := pdf.NewExtractor(out)
	res, err := pdf.CursorAt(x, nil).Dict(dicts[0]["Resources"])
	if err != nil {
		t.Fatal(err)
	}
	props, err := pdf.CursorAt(x, nil).Dict(res["Properties"])
	if err != nil {
		t.Fatal(err)
	}
	d0, err := pdf.CursorAt(x, nil).Dict(props[n0])
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d0["ActualText"]; ok || d0["MCID"] != pdf.Integer(7) {
		t.Errorf("bad stripped property list %v", d0)
	}
	d1, err := pdf.CursorAt(x, nil).Dict(props["P1"])
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d1["ActualText"]; !ok {
		t.Error("unrelated property list lost its ActualText")
	}
}

func TestMarkedContentNamedShared(t *testing.T) {
	// one named list used by a span which loses text and by one which does not
	const data = "/Span /P0 BDC\n" +
		"BT /F1 12 Tf 50 100 Td (Hello) Tj ET EMC\n" +
		"/Span /P0 BDC\n" +
		"BT /F1 12 Tf 50 200 Td (World) Tj ET EMC"
	o := &srcOpts{props: map[pdf.Name]pdf.Dict{
		"P0": {"ActualText": pdf.String("shared")},
	}}
	out, _ := redactText(t, data, o, "Hello")

	spans := bdcSpans(t, out, "Span")
	if len(spans) != 2 || spans[0][1] == spans[1][1] || spans[1][1] != pdf.Name("P0") {
		t.Fatalf("got spans %v", spans)
	}
	_, dicts := pageDicts(t, out)
	cur := pdf.CursorAt(pdf.NewExtractor(out), nil)
	res, _ := cur.Dict(dicts[0]["Resources"])
	props, _ := cur.Dict(res["Properties"])
	d, err := cur.Dict(props["P0"])
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d["ActualText"]; !ok {
		t.Error("list which is still in use lost its ActualText")
	}
}

func TestMarkedContentUnbalanced(t *testing.T) {
	// a stray EMC and a span which is never closed
	const data = "EMC\n/Span <</ActualText (secretA)>> BDC\n" +
		"BT /F1 12 Tf 50 100 Td (Hello) Tj ET"
	out, raw := redactText(t, data, nil, "Hello")
	if bytes.Contains(raw, []byte("secret")) {
		t.Error("output file still contains the text of the removed span")
	}
	_, dicts := pageDicts(t, out)
	var emc int
	for _, op := range pageOps(t, out, dicts[0]) {
		if op.name == content.OpEndMarkedContent {
			emc++
		}
	}
	// the stray one is dropped; one closes the open span and one the mark
	if emc != 2 {
		t.Errorf("got %d EMC operators, want 2", emc)
	}
}

// The structure elements of the marked content which was removed lose their
// text, and the elements of other content keep theirs.
func TestMarkedContentStructure(t *testing.T) {
	const tagged = "/P <</MCID %s>> BDC\n" +
		"BT /F1 12 Tf 10 20 Td (secret) Tj ET EMC\n" +
		"/P <</MCID 5>> BDC\nBT /F1 12 Tf 10 250 Td (keep) Tj ET EMC\n"
	cases := []struct {
		name    string
		content string
		props   pdf.Dict
		nums    func(p *structParts) pdf.Array
		lost    []string
	}{
		{
			name:    "direct properties",
			content: strings.Replace(tagged, "%s", "3", 1),
			nums: func(p *structParts) pdf.Array {
				return pdf.Array{pdf.Integer(0), pdf.Array{nil, nil, nil, p.pRef, nil, p.fRef}, pdf.Integer(1), p.aRef}
			},
			lost: []string{"secretAT", "secretAlt", "secretE"},
		},
		{
			name: "nested",
			content: "/P <</MCID 1>> BDC\n/Span <</MCID 2>> BDC\n" +
				"BT /F1 12 Tf 10 20 Td (secret) Tj ET EMC\n" +
				"/Span <</MCID 5>> BDC\nBT /F1 12 Tf 10 250 Td (keep) Tj ET EMC EMC\n",
			nums: func(p *structParts) pdf.Array {
				return pdf.Array{pdf.Integer(0), pdf.Array{nil, p.pRef, p.qRef, nil, nil, p.fRef}, pdf.Integer(1), p.aRef}
			},
			lost: []string{"secretAT", "secretAlt", "keepAT"},
		},
		{
			name: "named properties",
			content: "/Span /P0 BDC\nBT /F1 12 Tf 10 20 Td (secret) Tj ET EMC\n" +
				"/Span /P1 BDC\nBT /F1 12 Tf 10 250 Td (keep) Tj ET EMC\n",
			props: pdf.Dict{"P0": pdf.Dict{"MCID": pdf.Integer(7)}, "P1": pdf.Dict{"MCID": pdf.Integer(5)}},
			nums: func(p *structParts) pdf.Array {
				return pdf.Array{pdf.Integer(0), pdf.Array{nil, nil, nil, nil, nil, p.fRef, nil, p.pRef}, pdf.Integer(1), p.aRef}
			},
			lost: []string{"secretAT", "secretAlt", "secretE"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := structDoc(t, func(p *structParts) {
				p.content, p.props = c.content, c.props
				p.nums = c.nums(p)
			})
			_, data := redactStruct(t, in)
			for _, s := range c.lost {
				if bytes.Contains(data, []byte(s)) {
					t.Errorf("output still contains %q", s)
				}
			}
			if !bytes.Contains(data, []byte("formAT")) {
				t.Error("element of kept content lost its text")
			}
		})
	}
}

// hexShow returns the string operand which shows text in F, allocating codes
// as needed.
func hexShow(t testing.TB, F font.Layouter, text string) string {
	t.Helper()
	var s pdf.String
	for _, g := range F.Layout(nil, 1, text).Seq {
		code, ok := F.Encode(g.GID, g.Text)
		if !ok {
			t.Fatalf("cannot encode %q", g.Text)
		}
		s = F.Codec().AppendCode(s, code)
	}
	return fmt.Sprintf("<%x>", []byte(s))
}

// compositeFont returns a TrueType font which writes text with the given
// CMap, which is Identity-H or Identity-V.
func compositeFont(t testing.TB, cm *cmap.File) *truetype.Composite {
	t.Helper()
	F, err := truetype.NewComposite(makefont.TrueType(), &truetype.OptionsComposite{CMap: cm})
	if err != nil {
		t.Fatal(err)
	}
	return F
}

// codesOf returns the bytes of the codes which show text in F.
func codesOf(t testing.TB, F font.Layouter, text string) []byte {
	t.Helper()
	var s pdf.String
	for _, g := range F.Layout(nil, 1, text).Seq {
		code, _ := F.Encode(g.GID, g.Text)
		s = F.Codec().AppendCode(s, code)
	}
	return s
}

func TestRemoveTextComposite(t *testing.T) {
	cases := []struct {
		name  string
		cm    *cmap.File
		start string   // positions the text
		parts []string // strings and TJ numbers, as in a TJ array
		pre   string   // text state operators
	}{
		{"two-byte Tj", cmap.IdentityH, "50 100 Td", []string{"Hello Secret World"}, ""},
		{"two-byte Tw", cmap.IdentityH, "50 100 Td", []string{"Hello Secret World"}, "9 Tw"},
		{"two-byte Tc", cmap.IdentityH, "50 100 Td", []string{"Hello Secret World"}, "2 Tc"},
		{"two-byte TJ", cmap.IdentityH, "50 100 Td", []string{"Hello Se", "-120", "cret Wo", "40", "rld"}, ""},
		{"two-byte Tz", cmap.IdentityH, "50 100 Td", []string{"Hello Secret World"}, "70 Tz 3 Tc 4 Tw"},
		{"two-byte Tz TJ", cmap.IdentityH, "50 100 Td", []string{"Hello S", "300", "ecret World"}, "150 Tz"},
		{"vertical Tj", cmap.IdentityV, "150 280 Td", []string{"Hello Secret World"}, ""},
		{"vertical Tc", cmap.IdentityV, "150 280 Td", []string{"Hello Secret World"}, "1 Tc"},
		{"vertical Tw", cmap.IdentityV, "150 280 Td", []string{"Hello Secret World"}, "6 Tw"},
		{"vertical Tz", cmap.IdentityV, "150 280 Td", []string{"Hello Secret World"}, "50 Tz"},
		{"vertical TJ", cmap.IdentityV, "150 280 Td", []string{"Hello S", "200", "ecret", "-90", " World"}, ""},
		{"vertical TJ Tc", cmap.IdentityV, "150 280 Td", []string{"Hel", "-30", "lo Sec", "80", "ret World"}, "1.5 Tc 80 Tz"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			F := compositeFont(t, c.cm)
			var tj strings.Builder
			for _, p := range c.parts {
				if _, err := fmt.Sscan(p, new(float64)); err == nil {
					fmt.Fprintf(&tj, " %s ", p)
				} else {
					tj.WriteString(hexShow(t, F, p))
				}
			}
			data := fmt.Sprintf("BT /F2 12 Tf %s %s [%s] TJ ET", c.start, c.pre, tj.String())
			o := &srcOpts{fonts: map[pdf.Name]font.Instance{"F2": F}}
			checkTextRemovalOpt(t, data, o, "Secret", codesOf(t, F, "Secret"))
		})
	}
}

// TestWordSpacingMultiByte checks that word spacing does not apply to a code
// of two bytes, even where its first byte is a space.
func TestWordSpacingMultiByte(t *testing.T) {
	F := compositeFont(t, cmap.IdentityH)
	data := fmt.Sprintf("BT /F2 12 Tf 50 100 Td 20 Tw %s Tj ET", hexShow(t, F, "Ab Secret Cd"))
	o := &srcOpts{fonts: map[pdf.Name]font.Instance{"F2": F}}
	before := pageGlyphs(t, textSourceOpt(t, data, o))
	for i := 1; i < len(before); i++ {
		d := before[i].x - before[i-1].x
		if w := before[i-1].code.Width * 12; math.Abs(d-w) > 1e-3 {
			t.Fatalf("glyph %d: word spacing was applied to a two-byte code", i)
		}
	}
	checkTextRemovalOpt(t, data, o, "Secret", codesOf(t, F, "Secret"))
}

// type3TestFont returns a Type 3 font.  The glyphs A, B and E have no ink
// above 0.7 em.  C and D reach up to 1.5 em, C by the box of its d1
// operator and D by its ink.  F strokes a path with an inherited line width,
// so that its ink has no bound.
func type3TestFont(t testing.TB) font.Layouter {
	t.Helper()
	box := func(name string, d1 bool, w, h float64) *type3.Glyph {
		b := builder.New(content.Glyph, nil, pdf.V2_0)
		if d1 {
			b.Type3UncoloredGlyph(500, 0, 0, 0, w, h)
		} else {
			b.Type3ColoredGlyph(500, 0)
		}
		b.Rectangle(0, 0, w, h)
		b.Fill()
		stream, err := b.Harvest()
		if err != nil {
			t.Fatal(err)
		}
		return &type3.Glyph{Name: name, Content: stream}
	}
	unbounded := builder.New(content.Glyph, nil, pdf.V2_0)
	unbounded.Type3ColoredGlyph(500, 0)
	unbounded.Rectangle(0, 0, 400, 400)
	unbounded.Stroke()
	stream, err := unbounded.Harvest()
	if err != nil {
		t.Fatal(err)
	}
	F := &type3.Font{
		Glyphs: []*type3.Glyph{
			{},
			box("A", true, 500, 700),
			box("B", false, 500, 700),
			box("C", true, 500, 1500),
			box("D", false, 500, 1500),
			{Name: "F", Content: stream},
		},
		FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
	}
	inst, err := F.New()
	if err != nil {
		t.Fatal(err)
	}
	return inst
}

func TestRemoveTextType3(t *testing.T) {
	cases := []struct {
		name, content string
	}{
		{"Tj", "BT /T3 12 Tf 50 100 Td %s Tj ET"},
		{"Tc Tw", "BT /T3 12 Tf 50 100 Td 1 Tc 5 Tw %s Tj ET"},
		{"Tz", "BT /T3 12 Tf 50 100 Td 130 Tz %s Tj ET"},
		{"TJ", "BT /T3 12 Tf 50 100 Td [%s -200 %s 50] TJ ET"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			F := type3TestFont(t)
			var data string
			if strings.Count(c.content, "%s") == 2 {
				data = fmt.Sprintf(c.content, hexShow(t, F, "ABAB"), hexShow(t, F, "ABBA"))
			} else {
				data = fmt.Sprintf(c.content, hexShow(t, F, "ABAABBAB"))
			}
			o := &srcOpts{fonts: map[pdf.Name]font.Instance{"T3": F}}
			checkTextRemovalOpt(t, data, o, "BB", codesOf(t, F, "BB"))
		})
	}
}

// TestType3Extent checks the region test for Type 3 glyphs: the d1 box, or
// the ink where there is no box, decides.  A glyph with ink without bound
// meets every region.
func TestType3Extent(t *testing.T) {
	cases := []struct {
		name, text string
		region     pdf.Array
		want       string
	}{
		// the glyphs are at x = 50 and 56, where each is 6 wide
		{"d1 box hit", "AC", rect(56, 113, 62, 117), "A"},
		{"d1 box missed", "AC", rect(56, 119, 62, 125), "AC"},
		{"ink hit", "AD", rect(56, 113, 62, 117), "A"},
		{"ink missed", "AD", rect(56, 119, 62, 125), "AD"},
		{"first glyph hit", "AC", rect(50, 101, 55, 104), "C"},
		{"above short glyph", "AB", rect(50, 109, 62, 117), "AB"},
		{"no bound", "AF", rect(200, 200, 210, 210), "A"},
		{"bound", "AA", rect(200, 200, 210, 210), "AA"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			F := type3TestFont(t)
			data := fmt.Sprintf("BT /T3 12 Tf 50 100 Td %s Tj ET", hexShow(t, F, c.text))
			o := &srcOpts{fonts: map[pdf.Name]font.Instance{"T3": F}}
			in := textSourceOpt(t, data, o, pdf.Dict{"Rect": c.region})
			got := glyphString(pageGlyphs(t, writeAndRead(t, in, nil, "")))
			if got != c.want {
				t.Errorf("got text %q, want %q", got, c.want)
			}
		})
	}
}

// TestInkAboveAscent checks that a glyph whose ink rises above the ascent of
// the font is removed by a region which covers only this ink.
func TestInkAboveAscent(t *testing.T) {
	win := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": pdf.Name("Helvetica"),
		"Encoding": pdf.Name("WinAnsiEncoding"),
	}
	o := &srcOpts{rawFonts: map[pdf.Name]pdf.Dict{"F3": win}}
	// Aring (code 0xC5) reaches up to 0.929 em, where the ascent of the
	// font is 0.718 em
	const data = "BT /F3 12 Tf 50 100 Td (\xc5H) Tj ET"
	cases := []struct {
		name   string
		region pdf.Array
		want   string
	}{
		{"on the ring", rect(51, 110, 56, 111), "H"},
		{"above the ring", rect(51, 112, 56, 114), "ÅH"},
		{"on H", rect(59, 109, 63, 111), "ÅH"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := textSourceOpt(t, data, o, pdf.Dict{"Rect": c.region})
			got := glyphString(pageGlyphs(t, writeAndRead(t, in, nil, "")))
			if got != c.want {
				t.Errorf("got text %q, want %q", got, c.want)
			}
		})
	}
}

func TestRemoveTextZeroFontSize(t *testing.T) {
	cases := []struct {
		name, content string
		gs            bool
	}{
		{"Tc", "BT 50 100 Td /F1 0 Tf 3 Tc (ab) Tj (Secret) Tj (cd) Tj /F1 12 Tf (Kept) Tj ET", false},
		{"Tc Tw", "BT 50 100 Td /F1 0 Tf 3 Tc 4 Tw (a b) Tj (Se cret) Tj ( c d) Tj /F1 12 Tf 40 0 Td (Kept) Tj ET", false},
		{"TJ", "BT 50 100 Td /F1 0 Tf 3 Tc [(ab) 500 (Sec) -300 (ret) 20 (cd)] TJ /F1 12 Tf (Kept) Tj ET", false},
		{"Tz", "BT 50 100 Td /F1 0 Tf 3 Tc 200 Tz (ab) Tj (Secret) Tj (cd) Tj /F1 12 Tf (Kept) Tj ET", false},
		{"quote", "BT 50 100 Td 12 TL /F1 0 Tf 3 Tc (ab) Tj (Secret) ' (cd) Tj /F1 12 Tf (Kept) Tj ET", false},
		{"double quote", "BT 50 100 Td 12 TL /F1 0 Tf 3 Tc (ab) Tj 1 2 (Secret) \" (cd) Tj /F1 12 Tf (Kept) Tj ET", false},
		{"gs", "BT 50 100 Td /G1 gs 3 Tc (ab) Tj (Secret) Tj (cd) Tj /F1 12 Tf (Kept) Tj ET", true},
		{"gs Tw", "BT 50 100 Td /G1 gs 3 Tc 4 Tw (a b) Tj (Se cret) Tj (c d) Tj /F1 12 Tf (Kept) Tj ET", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var o *srcOpts
			if c.gs {
				// the font is not in the resource dictionary
				bold := pdf.Dict{
					"Type":     pdf.Name("Font"),
					"Subtype":  pdf.Name("Type1"),
					"BaseFont": pdf.Name("Helvetica-Bold"),
				}
				o = &srcOpts{
					rawFonts: map[pdf.Name]pdf.Dict{"F4": bold},
					gsFont:   "F4", gsHide: true,
				}
			}
			secret := "Secret"
			if strings.Contains(c.content, "(Se cret)") {
				secret = "Se cret"
			}
			checkTextRemovalOpt(t, c.content, o, secret, []byte(secret))
		})
	}
}

func TestRemoveTextZeroScaling(t *testing.T) {
	const data = "BT /F1 12 Tf 50 100 Td 0 Tz 2 Tc [(Sec) 100 (ret)] TJ 100 Tz 40 0 Td (After) Tj ET"
	before := pageGlyphs(t, textSource(t, data))
	text := glyphString(before)
	if text != "SecretAfter" {
		t.Fatalf("got text %q", text)
	}
	in := textSource(t, data, regionOver(before[:6]))
	after := pageGlyphs(t, writeAndRead(t, in, nil, ""))
	if got := glyphString(after); got != "After" {
		t.Fatalf("got text %q, want %q", got, "After")
	}
	for i, g := range after {
		w := before[6+i]
		if math.Abs(g.x-w.x) > 1e-3 || math.Abs(g.y-w.y) > 1e-3 {
			t.Errorf("glyph %q moved", g.text)
		}
	}
}

// emptyClipFollows reports whether the operators from the first ET on start
// with an empty clip.
func emptyClipFollows(ops []opRecord) bool {
	for i, op := range ops {
		if op.name != content.OpTextEnd {
			continue
		}
		if i+3 >= len(ops) || ops[i+1].name != content.OpRectangle ||
			ops[i+2].name != content.OpClipNonZero || ops[i+3].name != content.OpEndPath {
			return false
		}
		for _, a := range ops[i+1].args {
			if v, _ := getNumber(a); v != 0 {
				return false
			}
		}
		return true
	}
	return false
}

func TestClipModeRemoved(t *testing.T) {
	for _, mode := range []string{"4", "5", "6", "7"} {
		t.Run(mode, func(t *testing.T) {
			data := "q 0 0 1 rg " + mode + " Tr BT /F1 12 Tf 50 100 Td (Secret) Tj ET 0 0 300 300 re f Q"
			before := pageGlyphs(t, textSource(t, data))

			// all glyphs of the object are removed
			in := textSource(t, data, regionOver(before))
			out := writeAndRead(t, in, nil, "")
			_, dicts := pageDicts(t, out)
			if !emptyClipFollows(pageOps(t, out, dicts[0])) {
				t.Error("no empty clip after the text object")
			}

			// some glyphs are kept
			in = textSource(t, data, regionOver(before[:3]))
			out = writeAndRead(t, in, nil, "")
			_, dicts = pageDicts(t, out)
			if emptyClipFollows(pageOps(t, out, dicts[0])) {
				t.Error("empty clip, but glyphs are kept")
			}

			// nothing is removed
			in = textSource(t, data, pdf.Dict{"Rect": rect(200, 200, 250, 250)})
			out = writeAndRead(t, in, nil, "")
			_, dicts = pageDicts(t, out)
			if emptyClipFollows(pageOps(t, out, dicts[0])) {
				t.Error("empty clip, but nothing was removed")
			}
		})
	}
}

func TestClipModeTwoObjects(t *testing.T) {
	// the clip count restarts with each text object
	const data = "7 Tr BT /F1 12 Tf 50 100 Td (Kept) Tj ET BT 50 200 Td (Secret) Tj ET"
	before := pageGlyphs(t, textSource(t, data))
	in := textSource(t, data, regionOver(before[4:]))
	out := writeAndRead(t, in, nil, "")
	_, dicts := pageDicts(t, out)
	ops := pageOps(t, out, dicts[0])
	n := 0
	for i, op := range ops {
		if op.name == content.OpTextEnd {
			n++
			hasClip := i+1 < len(ops) && ops[i+1].name == content.OpRectangle
			if hasClip != (n == 2) {
				t.Errorf("text object %d: empty clip is %v", n, hasClip)
			}
		}
	}
}

func TestRemoveTextTinyFontSize(t *testing.T) {
	// The gap for the removed text is too large for a TJ number at the
	// font size, or overflows.
	for _, zeros := range []int{3, 60, 315} {
		size := "0." + strings.Repeat("0", zeros) + "1"
		t.Run(fmt.Sprintf("%d zeros", zeros), func(t *testing.T) {
			for _, data := range []string{
				"BT 50 100 Td /F1 " + size + " Tf 5 Tc (abSecretcd) Tj ET",
				"BT 50 100 Td /F1 " + size + " Tf 5 Tc [(ab) -1000 (Secret) 300 (cd)] TJ /F1 12 Tf (Kept) Tj ET",
			} {
				checkTextRemoval(t, data, "Secret")

				// the TJ numbers are within the range of PDF 1.x
				// (ISO 32000-1, Annex C)
				before := pageGlyphs(t, textSource(t, data))
				out := writeAndRead(t, textSource(t, data, regionOver(before[2:8])), nil, "")
				for _, op := range pageOps(t, out, pageAt(t, out, 0)) {
					if op.name != content.OpTextShowArray {
						continue
					}
					for _, x := range op.args[0].(pdf.Array) {
						if v, ok := getNumber(x); ok && math.Abs(v) > 3.403e38 {
							t.Errorf("TJ number %g is out of range", v)
						}
					}
				}
			}
		})
	}
}

func TestRemoveTextHugeAdvance(t *testing.T) {
	// the advance of the removed text is beyond the range of a TJ number at
	// font size one
	checkTextRemoval(t, "BT 50 100 Td /F1 0 Tf 1000000 Tc (Secretab) Tj ET", "Secret")
}
