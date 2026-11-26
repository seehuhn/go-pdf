// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/page"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/reader"
)

func rect(llx, lly, urx, ury int) pdf.Array {
	return pdf.Array{pdf.Integer(llx), pdf.Integer(lly), pdf.Integer(urx), pdf.Integer(ury)}
}

// annotTestSource builds a document with three pages.  Page 0 has text, a
// number of annotations around the text, and thumbnail-like entries.
// Page 1 has no redaction annotation, and page 2 has one.
func annotTestSource(t testing.TB) *pdf.Reader {
	t.Helper()
	w, buf := memfile.NewPDFWriter(t, pdf.V1_7, nil)

	putStream := func(data string) pdf.Reference {
		ref := w.Alloc()
		stm, err := w.OpenStream(ref, pdf.Dict{}, pdf.FilterCompress{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := stm.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
		if err := stm.Close(); err != nil {
			t.Fatal(err)
		}
		return ref
	}
	put := func(obj pdf.Object) pdf.Reference {
		ref := w.Alloc()
		if err := w.Put(ref, obj); err != nil {
			t.Fatal(err)
		}
		return ref
	}

	pagesRef := w.Alloc()
	page0, page1, page2 := w.Alloc(), w.Alloc(), w.Alloc()
	fontRef := put(pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": pdf.Name("Helvetica"),
	})

	annot := func(subtype pdf.Name, r pdf.Array, extra pdf.Dict) pdf.Dict {
		d := pdf.Dict{
			"Type":    pdf.Name("Annot"),
			"Subtype": subtype,
			"Rect":    r,
			"P":       page0,
		}
		maps.Copy(d, extra)
		return d
	}
	aText, aPopup, aKeep, aPopup2, aReply := w.Alloc(), w.Alloc(), w.Alloc(), w.Alloc(), w.Alloc()
	redactRect := put(annot("Redact", rect(40, 90, 120, 115), pdf.Dict{
		"OverlayText": pdf.String("REDACTED"),
		"DA":          pdf.String("/Helv 12 Tf 1 0 0 rg"),
		"IC":          pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(0)},
	}))
	redactQuad := put(annot("Redact", rect(190, 190, 230, 230), pdf.Dict{
		"QuadPoints": pdf.Array{
			pdf.Integer(190), pdf.Integer(230), pdf.Integer(230), pdf.Integer(230),
			pdf.Integer(190), pdf.Integer(190), pdf.Integer(230), pdf.Integer(190),
		},
	}))
	hit := put(annot("Square", rect(60, 95, 80, 110), nil))
	miss := put(annot("Square", rect(250, 10, 280, 40), nil))
	reply2 := put(annot("Text", rect(10, 10, 20, 20), pdf.Dict{"IRT": aReply}))
	for ref, obj := range map[pdf.Reference]pdf.Dict{
		aText:   annot("Text", rect(100, 100, 120, 120), pdf.Dict{"Popup": aPopup}),
		aPopup:  annot("Popup", rect(10, 250, 90, 290), pdf.Dict{"Parent": aText}),
		aReply:  annot("Text", rect(250, 250, 270, 270), pdf.Dict{"IRT": aText, "RT": pdf.Name("R")}),
		aKeep:   annot("Text", rect(250, 100, 262, 112), pdf.Dict{"Popup": aPopup2}),
		aPopup2: annot("Popup", rect(45, 95, 100, 110), pdf.Dict{"Parent": aKeep}),
	} {
		if err := w.Put(ref, obj); err != nil {
			t.Fatal(err)
		}
	}

	content0 := putStream("Q q BT /F1 12 Tf 50 100 Td (Secret) Tj ET\n")
	content1 := putStream("BT /F1 12 Tf 50 100 Td (Public) Tj ET\n")
	content2a := putStream("BT /F1 12 Tf 50 100 Td")
	content2b := putStream("(Other) Tj ET\n")
	redact2 := put(pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Redact"),
		"Rect": rect(40, 90, 120, 115), "P": page2,
	})

	res := pdf.Dict{"Font": pdf.Dict{"F1": fontRef}}
	media := rect(0, 0, 300, 300)
	for ref, obj := range map[pdf.Reference]pdf.Dict{
		pagesRef: {
			"Type": pdf.Name("Pages"), "Count": pdf.Integer(3),
			"Kids": pdf.Array{page0, page1, page2},
		},
		page0: {
			"Type": pdf.Name("Page"), "Parent": pagesRef, "MediaBox": media,
			"Resources": res, "Contents": content0,
			"Annots": pdf.Array{
				redactRect, redactQuad, hit, miss, aText, aPopup, aReply, reply2, aKeep, aPopup2,
			},
			"Thumb":     putStream("thumb"),
			"PieceInfo": pdf.Dict{},
			"Metadata":  putStream("<x/>"),
			"AF":        pdf.Array{},
		},
		page1: {
			"Type": pdf.Name("Page"), "Parent": pagesRef, "MediaBox": media,
			"Resources": res, "Contents": content1,
			"Annots": pdf.Array{put(annot("Square", rect(60, 95, 80, 110), nil))},
		},
		page2: {
			"Type": pdf.Name("Page"), "Parent": pagesRef, "MediaBox": media,
			"Resources": res, "Contents": pdf.Array{content2a, content2b},
			"Annots": pdf.Array{redact2},
		},
	} {
		if err := w.Put(ref, obj); err != nil {
			t.Fatal(err)
		}
	}
	w.GetMeta().Catalog.Pages = pagesRef
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	r := openPDF(t, buf.Data)
	return r
}

// pageDicts returns the page references and dictionaries of the document.
func pageDicts(t testing.TB, r *pdf.Reader) ([]pdf.Reference, []pdf.Dict) {
	t.Helper()
	var refs []pdf.Reference
	var dicts []pdf.Dict
	it := pagetree.NewIterator(r)
	for ref, dict := range it.All() {
		refs = append(refs, ref)
		dicts = append(dicts, dict)
	}
	if it.Err != nil {
		t.Fatal(it.Err)
	}
	return refs, dicts
}

// contentBytes returns the concatenated decoded content of a page.
func contentBytes(t testing.TB, r *pdf.Reader, dict pdf.Dict) []byte {
	t.Helper()
	segs, err := page.ExtractContents(pdf.CursorAt(pdf.NewExtractor(r), nil), dict["Contents"])
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(page.SegmentsReader(segs))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// annotSubtypes returns the subtypes of the annotations of a page.
func annotSubtypes(t testing.TB, r *pdf.Reader, dict pdf.Dict) []pdf.Name {
	t.Helper()
	cur := pdf.CursorAt(pdf.NewExtractor(r), nil)
	arr, err := cur.Array(dict["Annots"])
	if err != nil {
		t.Fatal(err)
	}
	var res []pdf.Name
	for _, item := range arr {
		d, err := cur.Dict(item)
		if err != nil {
			t.Fatal(err)
		}
		name, _ := cur.Name(d["Subtype"])
		res = append(res, name)
	}
	return res
}

type opRecord struct {
	name content.OpName
	args []pdf.Object
}

// pageOps returns the operators of a page's content.
func pageOps(t testing.TB, r *pdf.Reader, dict pdf.Dict) []opRecord {
	t.Helper()
	return scanOps(t, contentBytes(t, r, dict))
}

// scanOps returns the operators of a content stream.
func scanOps(t testing.TB, data []byte) []opRecord {
	t.Helper()
	it := content.NewScanner(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(data)), nil
	}).NewIter()
	var ops []opRecord
	for name, args := range it.All() {
		ops = append(ops, opRecord{name, slices.Clone(args)})
	}
	if err := it.Err(); err != nil {
		t.Fatal(err)
	}
	return ops
}

func TestRedactPage(t *testing.T) {
	in := annotTestSource(t)
	out := writeAndRead(t, in, nil, "")
	_, dicts := pageDicts(t, out)
	d := dicts[0]

	// annotations: only the ones clear of the regions survive
	got := annotSubtypes(t, out, d)
	want := []pdf.Name{"Square", "Text"}
	if !slices.Equal(got, want) {
		t.Errorf("annotations = %v, want %v", got, want)
	}
	cur := pdf.CursorAt(pdf.NewExtractor(out), nil)
	annots, _ := cur.Array(d["Annots"])
	keep, _ := cur.Dict(annots[1])
	if _, ok := keep["Popup"]; ok {
		t.Error("kept annotation still refers to its deleted pop-up")
	}

	// deleted annotations do not survive anywhere in the file
	for i := 1; i < 200; i++ {
		obj, err := pdf.Resolve(out, pdf.NewReference(uint32(i), 0))
		if err != nil {
			continue
		}
		od, ok := obj.(pdf.Dict)
		if !ok {
			continue
		}
		switch od["Subtype"] {
		case pdf.Name("Redact"), pdf.Name("Popup"):
			t.Errorf("object %d is a %v annotation", i, od["Subtype"])
		}
	}

	// entries which can hold the original artwork
	for _, key := range []pdf.Name{"Thumb", "PieceInfo", "Metadata", "AF"} {
		if _, ok := d[key]; ok {
			t.Errorf("page still has /%s", key)
		}
	}

	// the original content comes after the exclusion clip
	ops := pageOps(t, out, d)
	var names []content.OpName
	for _, op := range ops {
		names = append(names, op.name)
	}
	firstText := slices.Index(names, content.OpTextBegin)
	if firstText < 0 {
		t.Fatalf("original text is missing from %v", names)
	}
	state := content.NewState(content.Page, &content.Resources{})
	for i, op := range ops[:firstText] {
		if err := state.ApplyStateChanges(op.name, op.args); err != nil {
			t.Fatalf("operator %d (%s): %v", i, op.name, err)
		}
	}
	var clip *graphics.ClipPath
	for i := range state.GState.ClipPaths {
		if state.GState.ClipPaths[i].EvenOdd {
			clip = &state.GState.ClipPaths[i]
		}
	}
	if clip == nil {
		t.Fatal("original content is not preceded by an exclusion clip")
	}
	if len(state.GState.ClipPaths) != 2 {
		t.Errorf("got %d clips, want one for each region", len(state.GState.ClipPaths))
	}

	// the content stays balanced
	depth := 0
	for _, op := range ops {
		switch op.name {
		case content.OpPushGraphicsState:
			depth++
		case content.OpPopGraphicsState:
			depth--
			if depth < 0 {
				t.Fatal("content pops more than it pushes")
			}
		}
	}
	if depth != 0 {
		t.Errorf("content leaves %d graphics states open", depth)
	}

	// the mark is an artifact which draws a form
	n := len(ops)
	if n < 3 || ops[n-1].name != content.OpEndMarkedContent ||
		ops[n-2].name != content.OpXObject || ops[n-3].name != content.OpBeginMarkedContentWithProperties {
		t.Fatalf("content does not end with a marked-content mark: %v", names)
	}
	if ops[n-3].args[0] != pdf.Name("Artifact") {
		t.Errorf("mark has tag %v, want Artifact", ops[n-3].args[0])
	}
	if props, _ := ops[n-3].args[1].(pdf.Dict); props["Subtype"] != pdf.Name("Redaction") {
		t.Errorf("mark has properties %v, want Subtype Redaction", ops[n-3].args[1])
	}

	if text := overlayText(t, out, 0); !strings.Contains(text, "REDACTED") {
		t.Errorf("overlay text = %q, want REDACTED", text)
	}

	// the original resources are kept
	res, _ := cur.Dict(d["Resources"])
	fonts, _ := cur.Dict(res["Font"])
	if _, ok := fonts["F1"]; !ok {
		t.Error("original font resource is missing")
	}
}

// overlayText returns the text drawn by the forms which a page draws inside
// marked content tagged /Artifact.
func overlayText(t testing.TB, r *pdf.Reader, pageNo int) string {
	t.Helper()
	x := pdf.NewExtractor(r)
	refs, _ := pageDicts(t, r)
	pg, err := pdf.CursorAt(x, nil).Decode(refs[pageNo], page.Decode)
	if err != nil {
		t.Fatal(err)
	}

	var sb strings.Builder
	inArtifact := 0
	rd := reader.New(x)
	rd.MarkedContent = func(event reader.MarkedContentEvent, mc *graphics.MarkedContent) error {
		switch event {
		case reader.MarkedContentBegin:
			if mc.Tag == "Artifact" {
				inArtifact++
			}
		case reader.MarkedContentEnd:
			if mc.Tag == "Artifact" {
				inArtifact--
			}
		}
		return nil
	}
	rd.XObject = func(obj graphics.XObject, _ matrix.Matrix) error {
		f, ok := obj.(*form.Form)
		if !ok || inArtifact == 0 {
			return nil
		}
		sub := reader.New(x)
		sub.State = content.NewState(content.Form, f.Res)
		sub.Character = func(c font.Code) error {
			sb.WriteString(c.Text)
			return nil
		}
		return sub.ProcessIter(f.Content.NewIter())
	}
	if err := rd.ProcessPage(pg); err != nil {
		t.Fatal(err)
	}
	return sb.String()
}

func TestRedactPageWithoutRedactions(t *testing.T) {
	in := annotTestSource(t)
	out := writeAndRead(t, in, nil, "")
	_, inDicts := pageDicts(t, in)
	_, outDicts := pageDicts(t, out)

	if !bytes.Equal(contentBytes(t, in, inDicts[1]), contentBytes(t, out, outDicts[1])) {
		t.Error("content of a page without redactions changed")
	}
	if got := annotSubtypes(t, out, outDicts[1]); !slices.Equal(got, []pdf.Name{"Square"}) {
		t.Errorf("annotations = %v, want a square", got)
	}
}

func TestRedactPageSplitContent(t *testing.T) {
	in := annotTestSource(t)
	out := writeAndRead(t, in, nil, "")
	_, dicts := pageDicts(t, out)

	if got := annotSubtypes(t, out, dicts[2]); len(got) != 0 {
		t.Errorf("annotations = %v, want none", got)
	}
	// the font is set in the first segment, the text is shown in the second
	for _, op := range pageOps(t, out, dicts[2]) {
		if op.name == content.OpTextShow {
			t.Errorf("text under the region is still shown")
		}
		if op.name == content.OpTextShowArray {
			for _, el := range op.args[0].(pdf.Array) {
				if _, isString := el.(pdf.String); isString {
					t.Errorf("text under the region is still shown")
				}
			}
		}
	}
}

func TestRedactPageUnusableRegion(t *testing.T) {
	for _, r := range []pdf.Object{
		pdf.Array{pdf.Integer(1)},
		rect(40, 90, 40, 115),
	} {
		w, buf := memfile.NewPDFWriter(t, pdf.V1_7, nil)
		pagesRef, pageRef := w.Alloc(), w.Alloc()
		annotRef := w.Alloc()
		contents := w.Alloc()
		stm, err := w.OpenStream(contents, pdf.Dict{}, pdf.FilterCompress{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := stm.Write([]byte("0 0 10 10 re f\n")); err != nil {
			t.Fatal(err)
		}
		if err := stm.Close(); err != nil {
			t.Fatal(err)
		}
		for ref, obj := range map[pdf.Reference]pdf.Dict{
			pagesRef: {"Type": pdf.Name("Pages"), "Count": pdf.Integer(1), "Kids": pdf.Array{pageRef}},
			pageRef: {
				"Type": pdf.Name("Page"), "Parent": pagesRef, "MediaBox": rect(0, 0, 300, 300),
				"Contents": contents, "Annots": pdf.Array{annotRef},
			},
			annotRef: {"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Redact"), "Rect": r},
		} {
			if err := w.Put(ref, obj); err != nil {
				t.Fatal(err)
			}
		}
		w.GetMeta().Catalog.Pages = pagesRef
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		in := openPDF(t, buf.Data)

		err = Write(io.Discard, in, nil)
		var e *UnusableRedactionError
		if !errors.As(err, &e) || e.Page != 0 {
			t.Errorf("rectangle %v: got error %v, want UnusableRedactionError for page 0", r, err)
		}
	}
}

// checkBalanced checks that the graphics states, text objects and
// marked-content sequences of a content stream are properly nested and
// closed.  It returns the depth of the nesting at each operator.
func checkBalanced(t testing.TB, ops []opRecord) []int {
	t.Helper()
	var open []content.OpName
	depth := make([]int, len(ops))
	for i, op := range ops {
		depth[i] = len(open)
		switch op.name {
		case content.OpPushGraphicsState:
			open = append(open, content.OpPopGraphicsState)
		case content.OpTextBegin:
			open = append(open, content.OpTextEnd)
		case content.OpBeginMarkedContent, content.OpBeginMarkedContentWithProperties:
			open = append(open, content.OpEndMarkedContent)
		case content.OpPopGraphicsState, content.OpTextEnd, content.OpEndMarkedContent:
			if n := len(open); n == 0 || open[n-1] != op.name {
				t.Errorf("operator %d: %s closes nothing", i, op.name)
			} else {
				open = open[:n-1]
			}
		}
	}
	if len(open) > 0 {
		t.Errorf("content leaves %v open", open)
	}
	return depth
}

// TestRedactPageOpenAtEnd checks that page content which ends inside a text
// object and a marked-content sequence is closed before the marks.
func TestRedactPageOpenAtEnd(t *testing.T) {
	const data = "/Span <</MCID 0>> BDC q BT /F1 12 Tf 50 100 Td (Hello Secret) Tj"
	checkTextRemoval(t, data, "Secret")

	before := pageGlyphs(t, textSource(t, data))
	in := textSource(t, data, regionOver(before[6:12]))
	out := writeAndRead(t, in, nil, "")
	_, dicts := pageDicts(t, out)
	ops := pageOps(t, out, dicts[0])
	depth := checkBalanced(t, ops)
	found := false
	for i, op := range ops {
		if op.name == content.OpBeginMarkedContentWithProperties && len(op.args) > 0 && op.args[0] == pdf.Name("Artifact") {
			found = true
			if depth[i] != 0 {
				t.Errorf("the marks are drawn at nesting depth %d", depth[i])
			}
		}
	}
	if !found {
		t.Error("the marks are missing")
	}
}

// pagesDoc builds a document with n pages of size 300x300.  The function
// setup completes the page dictionaries, and can write further objects to w.
// Use [addRedaction] to add redaction annotations.
func pagesDoc(t testing.TB, n int, setup func(w *pdf.Writer, refs []pdf.Reference, pages []pdf.Dict)) *pdf.Reader {
	t.Helper()
	w, buf := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	pagesRef := w.Alloc()
	refs := make([]pdf.Reference, n)
	pages := make([]pdf.Dict, n)
	kids := make(pdf.Array, n)
	for i := range n {
		refs[i] = w.Alloc()
		kids[i] = refs[i]
		pages[i] = pdf.Dict{
			"Type": pdf.Name("Page"), "Parent": pagesRef, "MediaBox": rect(0, 0, 300, 300),
		}
	}
	setup(w, refs, pages)
	if err := w.Put(pagesRef, pdf.Dict{
		"Type": pdf.Name("Pages"), "Count": pdf.Integer(n), "Kids": kids,
	}); err != nil {
		t.Fatal(err)
	}
	for i, d := range pages {
		if err := w.Put(refs[i], d); err != nil {
			t.Fatal(err)
		}
	}
	w.GetMeta().Catalog.Pages = pagesRef
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	r := openSource(t, buf.Data)
	return r
}

// addRedaction adds a redaction annotation with the rectangle r to a page
// dictionary of [pagesDoc].
func addRedaction(t testing.TB, w *pdf.Writer, pageRef pdf.Reference, page pdf.Dict, r pdf.Object) {
	t.Helper()
	ref := w.Alloc()
	if err := w.Put(ref, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Redact"), "P": pageRef, "Rect": r,
	}); err != nil {
		t.Fatal(err)
	}
	annots, _ := page["Annots"].(pdf.Array)
	page["Annots"] = append(annots, ref)
}

// putRawStream writes a stream with the dictionary dict and the data as
// given, without applying any filter.
func putRawStream(t testing.TB, w *pdf.Writer, dict pdf.Dict, data []byte) pdf.Reference {
	t.Helper()
	ref := w.Alloc()
	stm, err := w.OpenStream(ref, dict)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stm.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	return ref
}

// rewriteTwice applies the redactions of in, and then writes the output
// again.  It checks that both outputs have the same page content and returns
// the first output and its raw bytes.
func rewriteTwice(t testing.TB, in *pdf.Reader) (*pdf.Reader, []byte) {
	t.Helper()
	var buf1 bytes.Buffer
	if err := Write(&buf1, in, nil); err != nil {
		t.Fatal(err)
	}
	out1 := openSource(t, buf1.Bytes())
	var buf2 bytes.Buffer
	if err := Write(&buf2, out1, nil); err != nil {
		t.Fatal(err)
	}
	out2 := openPDF(t, buf2.Bytes())
	_, d1 := pageDicts(t, out1)
	_, d2 := pageDicts(t, out2)
	if len(d1) != len(d2) {
		t.Fatalf("page count changed from %d to %d", len(d1), len(d2))
	}
	for i := range d1 {
		if c1, c2 := contentBytes(t, out1, d1[i]), contentBytes(t, out2, d2[i]); !bytes.Equal(c1, c2) {
			t.Errorf("page %d: content changed in the second round:\n%s\n%s", i, c1, c2)
		}
	}
	return out1, buf1.Bytes()
}

// TestRedactPageMalformedResources checks that a redacted page with
// malformed resources can be written.
func TestRedactPageMalformedResources(t *testing.T) {
	in := pagesDoc(t, 1, func(w *pdf.Writer, refs []pdf.Reference, pages []pdf.Dict) {
		pages[0]["Contents"] = putRawStream(t, w, pdf.Dict{}, []byte("0 0 10 10 re f\n"))
		pages[0]["Resources"] = pdf.Integer(7)
		addRedaction(t, w, refs[0], pages[0], rect(100, 100, 200, 200))
	})
	rewriteTwice(t, in)
}

// truncatedFlate returns Flate data which decodes to head, followed by a
// truncated encoding of tail.
func truncatedFlate(head, tail string) []byte {
	var zbuf bytes.Buffer
	zw := zlib.NewWriter(&zbuf)
	zw.Write([]byte(head))
	zw.Flush()
	cut := zbuf.Len()
	zw.Write([]byte(tail))
	zw.Close()
	compressed := zbuf.Bytes()
	return compressed[:cut+(len(compressed)-cut)/2]
}

// TestRedactPageBrokenContent checks that page content which cannot be
// decoded completely is replaced by a rewritten copy of the decoded part.
func TestRedactPageBrokenContent(t *testing.T) {
	const visible = "0 0 10 10 re f\n"
	const secret = "BT /F1 12 Tf 20 20 Td (TopSecretText) Tj ET\n"
	truncated := truncatedFlate(visible, secret)

	cases := []struct {
		name string
		dict pdf.Dict
		data []byte
	}{
		{"truncated", pdf.Dict{"Filter": pdf.Name("FlateDecode")}, truncated},
		{"unknown filter", pdf.Dict{"Filter": pdf.Name("NoSuchDecode")}, []byte(visible + secret)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := pagesDoc(t, 1, func(w *pdf.Writer, refs []pdf.Reference, pages []pdf.Dict) {
				pages[0]["Contents"] = putRawStream(t, w, c.dict, c.data)
				pages[0]["Resources"] = pdf.Dict{"Font": pdf.Dict{"F1": pdf.Dict{
					"Type": pdf.Name("Font"), "Subtype": pdf.Name("Type1"), "BaseFont": pdf.Name("Helvetica"),
				}}}
				addRedaction(t, w, refs[0], pages[0], rect(200, 200, 250, 250))
			})
			out, data := rewriteTwice(t, in)
			if bytes.Contains(data, c.data) {
				t.Error("output holds the original stream data")
			}
			for i, s := range fileStreams(t, out) {
				if bytes.Contains(s, []byte("TopSecretText")) {
					t.Errorf("stream %d holds the undecoded text", i)
				}
			}
		})
	}
}

// TestRedactPageOpenPath checks that page content which ends inside a path
// object has the path ended before the marks.
func TestRedactPageOpenPath(t *testing.T) {
	for _, data := range []string{
		"0 0 m 10 10 l",
		"q 0 0 100 100 re W",
		"BT ET 5 5 m 20 20 30 30 40 40 c h",
	} {
		in := pagesDoc(t, 1, func(w *pdf.Writer, refs []pdf.Reference, pages []pdf.Dict) {
			pages[0]["Contents"] = putRawStream(t, w, pdf.Dict{}, []byte(data))
			addRedaction(t, w, refs[0], pages[0], rect(200, 200, 250, 250))
		})
		out := writeAndRead(t, in, nil, "")
		_, dicts := pageDicts(t, out)
		ops := pageOps(t, out, dicts[0])
		checkBalanced(t, ops)
		inPath := false
		for i, op := range ops {
			_, paints := content.PathPainting(op.name)
			switch {
			case paints:
				inPath = false
			case isPathOp(op.name):
				inPath = true
			case inPath:
				t.Errorf("%q: operator %d (%s) is inside a path object", data, i, op.name)
			}
		}
		if inPath {
			t.Errorf("%q: the content ends inside a path object", data)
		}
	}
}

// TestRedactPageContentShared checks that the original content stream of a
// page with regions is copied only for a page without regions which shows
// it, and not for other references to the stream.
func TestRedactPageContentShared(t *testing.T) {
	const data = "BT /F1 12 Tf 20 20 Td (TopSecretText) Tj ET"
	font := pdf.Dict{"Font": pdf.Dict{"F1": pdf.Dict{
		"Type": pdf.Name("Font"), "Subtype": pdf.Name("Type1"), "BaseFont": pdf.Name("Helvetica"),
	}}}
	for _, array := range []bool{false, true} {
		for _, plainUse := range []bool{false, true} {
			in := pagesDoc(t, 2, func(w *pdf.Writer, refs []pdf.Reference, pages []pdf.Dict) {
				c := putRawStream(t, w, pdf.Dict{}, []byte(data))
				var contents pdf.Object = c
				if array {
					contents = w.Alloc()
					if err := w.Put(contents.(pdf.Reference), pdf.Array{c}); err != nil {
						t.Fatal(err)
					}
				}
				for i, pg := range pages {
					pg["Resources"] = font
					pg["Contents"] = putRawStream(t, w, pdf.Dict{}, nil)
					if i == 0 || plainUse {
						pg["Contents"] = contents
					}
				}
				addRedaction(t, w, refs[0], pages[0], rect(10, 10, 200, 40))

				// a marked-content reference to the content stream
				root, elem := w.Alloc(), w.Alloc()
				if err := w.Put(elem, pdf.Dict{
					"Type": pdf.Name("StructElem"), "S": pdf.Name("P"), "P": root,
					"K": pdf.Dict{"Type": pdf.Name("MCR"), "Stm": c, "MCID": pdf.Integer(0)},
				}); err != nil {
					t.Fatal(err)
				}
				if err := w.Put(root, pdf.Dict{"Type": pdf.Name("StructTreeRoot"), "K": elem}); err != nil {
					t.Fatal(err)
				}
				w.GetMeta().Catalog.StructTreeRoot = root
			})
			out, raw := writeRaw(t, in)
			want := 0
			if plainUse {
				want = 1
			}
			if n := bytes.Count(raw, []byte(data)); n != want {
				t.Errorf("array=%t, plain=%t: output holds %d copies of the content, want %d",
					array, plainUse, n, want)
			}
			pcs, _ := pageContents(t, out)
			if got := glyphString(pcs[0].glyphs); strings.Contains(got, "TopSecretText") {
				t.Errorf("array=%t, plain=%t: page with regions shows %q", array, plainUse, got)
			}
			if got := glyphString(pcs[1].glyphs); plainUse && got != "TopSecretText" {
				t.Errorf("array=%t: page without regions shows %q", array, got)
			}
		}
	}
}

// TestRedactPageRepeatedAnnot checks that a redaction annotation which the
// /Annots array lists repeatedly is applied once.
func TestRedactPageRepeatedAnnot(t *testing.T) {
	build := func(repeat int) *pdf.Reader {
		var annots strings.Builder
		for range repeat {
			annots.WriteString("4 0 R ")
		}
		return openPDF(t, rawDoc("1.7", "",
			"<</Type/Catalog/Pages 2 0 R>>",
			"<</Type/Pages/Kids[3 0 R]/Count 1>>",
			"<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 200]/Annots["+annots.String()+"]>>",
			"<</Type/Annot/Subtype/Redact/Rect[0 0 200 200]/QuadPoints[10 20 30 20 10 10 30 10 50 60 70 60 50 50 70 50]>>",
		))
	}
	outOnce := writeAndRead(t, build(1), nil, "")
	_, once := pageDicts(t, outOnce)
	out := writeAndRead(t, build(50), nil, "")
	_, repeated := pageDicts(t, out)
	if !bytes.Equal(contentBytes(t, out, repeated[0]), contentBytes(t, outOnce, once[0])) {
		t.Error("the repeated annotation changes the page content")
	}
}

// TestRedactPagePopupNotAnnot checks that a /Popup entry of a deleted
// annotation which does not refer to a pop-up annotation deletes nothing:
// here it refers to the content stream, the font and the dictionary of
// another page.
func TestRedactPagePopupNotAnnot(t *testing.T) {
	for _, target := range []string{"9 0 R", "5 0 R", "7 0 R"} {
		data := rawDoc("1.7", "",
			"<</Type/Catalog/Pages 2 0 R>>",
			"<</Type/Pages/Kids[3 0 R 7 0 R]/Count 2>>",
			"<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 200]/Resources<</Font<</F1 5 0 R>>>>/Contents 4 0 R/Annots[6 0 R 8 0 R]>>",
			rawStream("BT /F1 12 Tf 10 50 Td (Secret) Tj ET"),
			"<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>",
			"<</Type/Annot/Subtype/Redact/Rect[0 0 100 100]>>",
			"<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 200]/Resources<</Font<</F1 5 0 R>>>>/Contents 9 0 R>>",
			"<</Type/Annot/Subtype/Text/Rect[10 10 20 20]/Popup "+target+">>",
			rawStream("BT /F1 12 Tf 10 150 Td (Public) Tj ET"),
		)
		out := writeAndRead(t, openPDF(t, data), nil, "")
		_, dicts := pageDicts(t, out)
		if len(dicts) != 2 {
			t.Fatalf("popup %s: got %d pages, want 2", target, len(dicts))
		}
		if !bytes.Contains(contentBytes(t, out, dicts[1]), []byte("Public")) {
			t.Errorf("popup %s: the second page lost its content", target)
		}
		fonts, err := pdf.CursorAt(pdf.NewExtractor(out), nil).Dict(path(t, out, dicts[1], "Resources", "Font"))
		if err != nil {
			t.Fatal(err)
		}
		if f := resolveDict(t, out, fonts["F1"]); f["BaseFont"] != pdf.Name("Helvetica") {
			t.Errorf("popup %s: the second page lost its font", target)
		}
		checkStable(t, out)
	}
}

// TestRedactPageAnnotOnTwoPages checks that an annotation which is deleted
// is removed from every page which lists it, and that the pop-ups of the
// deleted annotations and the replies to them are deleted on all pages.
func TestRedactPageAnnotOnTwoPages(t *testing.T) {
	data := rawDoc("1.7", "",
		"<</Type/Catalog/Pages 2 0 R>>",
		"<</Type/Pages/Kids[3 0 R 4 0 R]/Count 2>>",
		// page 0 has the region; 6 meets it
		"<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 200]/Annots[5 0 R 6 0 R 10 0 R]>>",
		// page 1 lists 6, a reply to it, its pop-up, and an annotation whose
		// pop-up meets the region on page 0
		"<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 200]/Annots[6 0 R 7 0 R 8 0 R 9 0 R 11 0 R]>>",
		"<</Type/Annot/Subtype/Redact/Rect[0 0 100 100]>>",
		"<</Type/Annot/Subtype/Text/Rect[10 10 20 20]/Popup 8 0 R/Contents(Note6)>>",
		"<</Type/Annot/Subtype/Text/Rect[150 150 160 160]/IRT 6 0 R/Contents(Reply)>>",
		"<</Type/Annot/Subtype/Popup/Rect[150 150 190 190]/Parent 6 0 R>>",
		"<</Type/Annot/Subtype/Text/Rect[150 150 160 160]/Popup 10 0 R/Contents(Note9)>>",
		"<</Type/Annot/Subtype/Popup/Rect[10 10 50 50]/Parent 9 0 R>>",
		"<</Type/Annot/Subtype/Text/Rect[170 170 180 180]/Contents(Kept)>>",
	)
	out := writeAndRead(t, openPDF(t, data), nil, "")
	_, dicts := pageDicts(t, out)
	if len(dicts) != 2 {
		t.Fatalf("got %d pages, want 2", len(dicts))
	}
	if got := annotSubtypes(t, out, dicts[0]); len(got) != 0 {
		t.Errorf("page 0 keeps the annotations %v", got)
	}
	cur := pdf.CursorAt(pdf.NewExtractor(out), nil)
	annots, err := cur.Array(dicts[1]["Annots"])
	if err != nil {
		t.Fatal(err)
	}
	var contents []string
	for _, obj := range annots {
		d, err := cur.Dict(obj)
		if err != nil || d == nil {
			t.Fatalf("/Annots entry %v is not a dictionary", obj)
		}
		if _, hasPopup := d["Popup"]; hasPopup {
			t.Errorf("annotation %v keeps the deleted pop-up", d["Contents"])
		}
		s, _ := cur.String(d["Contents"])
		contents = append(contents, string(s))
	}
	if d := cmp.Diff([]string{"Note9", "Kept"}, contents); d != "" {
		t.Errorf("annotations of page 1 (-want +got):\n%s", d)
	}
	for _, text := range []string{"Note6", "Reply"} {
		hasText := func(obj pdf.Native) bool {
			s, _ := dictOf(obj)["Contents"].(pdf.String)
			return string(s) == text
		}
		if n := countObjects(t, out, hasText); n != 0 {
			t.Errorf("%d objects hold the deleted annotation %s", n, text)
		}
	}
	checkStable(t, out)
}

// TestRedactPageAnnotKeptThenDeleted checks that an annotation which is kept
// on a page, but deleted on a later one, leaves no copy in the output, even
// though its pop-up is deleted on the first page.
func TestRedactPageAnnotKeptThenDeleted(t *testing.T) {
	data := rawDoc("1.7", "",
		"<</Type/Catalog/Pages 2 0 R>>",
		"<</Type/Pages/Kids[3 0 R 4 0 R]/Count 2>>",
		"<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 200]/Annots[5 0 R 7 0 R 8 0 R]>>",
		"<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 200]/Annots[6 0 R 7 0 R]>>",
		"<</Type/Annot/Subtype/Redact/Rect[0 0 50 50]>>",
		"<</Type/Annot/Subtype/Redact/Rect[100 100 200 200]>>",
		"<</Type/Annot/Subtype/Text/Rect[150 150 160 160]/Popup 8 0 R/Contents(Note)>>",
		"<</Type/Annot/Subtype/Popup/Rect[10 10 40 40]/Parent 7 0 R>>",
	)
	out := writeAndRead(t, openPDF(t, data), nil, "")
	hasNote := func(obj pdf.Native) bool {
		s, _ := dictOf(obj)["Contents"].(pdf.String)
		return string(s) == "Note"
	}
	if n := countObjects(t, out, hasNote); n != 0 {
		t.Errorf("%d objects hold the deleted annotation", n)
	}
	_, dicts := pageDicts(t, out)
	for i, d := range dicts {
		if got := annotSubtypes(t, out, d); len(got) != 0 {
			t.Errorf("page %d keeps the annotations %v", i, got)
		}
	}
}

// TestRedactPageNonDictAnnots checks that /Annots entries which are not
// dictionaries are removed from a page with regions, without removing the
// objects they refer to.
func TestRedactPageNonDictAnnots(t *testing.T) {
	data := rawDoc("1.7", "",
		"<</Type/Catalog/Pages 2 0 R>>",
		"<</Type/Pages/Kids[3 0 R 4 0 R]/Count 2>>",
		"<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 200]/Annots[5 0 R 6 0 R 7 0 R 12]>>",
		"<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 200]/Contents 6 0 R/Resources<</Font<</F1 7 0 R>>>>>>",
		"<</Type/Annot/Subtype/Redact/Rect[0 0 50 50]>>",
		rawStream("BT /F1 12 Tf 10 150 Td (Public) Tj ET"),
		"<</Type/Font/Subtype/Type1/BaseFont/Helvetica/Rect[0 0 50 50]>>",
	)
	out := writeAndRead(t, openPDF(t, data), nil, "")
	_, dicts := pageDicts(t, out)
	if _, has := dicts[0]["Annots"]; has {
		t.Errorf("page 0 keeps /Annots %v", dicts[0]["Annots"])
	}
	if !bytes.Contains(contentBytes(t, out, dicts[1]), []byte("Public")) {
		t.Error("the second page lost its content")
	}
	if f := resolveDict(t, out, path(t, out, dicts[1], "Resources", "Font", "F1")); f["BaseFont"] != pdf.Name("Helvetica") {
		t.Error("the second page lost its font")
	}
}

// TestOverlayMarks checks that the marks of redaction annotations with
// unusual overlay entries are drawn and remove the content under them.
func TestOverlayMarks(t *testing.T) {
	region := func() pdf.Dict { return pdf.Dict{"Rect": rect(0, 0, 150, 300)} }
	cases := []struct {
		name  string
		annot func(b *nestBuilder) pdf.Dict
		text  string // the overlay text which the mark shows
	}{
		{"overlay form", func(b *nestBuilder) pdf.Dict {
			d := region()
			d["RO"] = b.form(pdf.Dict{"BBox": rect(0, 0, 100, 20), "Resources": helv()},
				"BT /F1 12 Tf 2 2 Td (OVERLAYFORM) Tj ET")
			return d
		}, ""},
		{"repeated and centred", func(*nestBuilder) pdf.Dict {
			d := region()
			d["OverlayText"] = pdf.String("AB")
			d["DA"] = pdf.String("/Helv 10 Tf 1 0 0 rg")
			d["Repeat"] = pdf.Boolean(true)
			d["Q"] = pdf.Integer(1)
			return d
		}, "AB"},
		{"unparsable appearance string", func(*nestBuilder) pdf.Dict {
			d := region()
			d["OverlayText"] = pdf.String("AB")
			d["DA"] = pdf.String("garbage ] Tf")
			return d
		}, "AB"},
		{"quad without area", func(*nestBuilder) pdf.Dict {
			d := region()
			d["OverlayText"] = pdf.String("AB")
			d["DA"] = pdf.String("/Helv 0 Tf")
			d["QuadPoints"] = pdf.Array{
				pdf.Integer(0), pdf.Integer(300), pdf.Integer(150), pdf.Integer(300),
				pdf.Integer(0), pdf.Integer(0), pdf.Integer(150), pdf.Integer(0),
				pdf.Integer(10), pdf.Integer(10), pdf.Integer(10), pdf.Integer(10),
				pdf.Integer(10), pdf.Integer(10), pdf.Integer(10), pdf.Integer(10),
			}
			return d
		}, "AB"},
		{"characters which the font lacks", func(*nestBuilder) pdf.Dict {
			d := region()
			d["OverlayText"] = pdf.String("\xfe\xff\x65\xe5\x67\x2c")
			d["DA"] = pdf.String("/Helv 12 Tf")
			return d
		}, "日本"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := nestDoc(t, nil, func(b *nestBuilder) []nestPage {
				reg := c.annot(b)
				reg["Type"], reg["Subtype"] = pdf.Name("Annot"), pdf.Name("Redact")
				return []nestPage{{
					content: "BT /F1 12 Tf 20 100 Td (Secret) Tj 180 0 Td (Kept) Tj ET",
					res:     helv(),
					annots:  []pdf.Object{b.put(reg)},
				}}
			})
			out := writeAndRead(t, in, nil, "")
			if got := glyphString(pageGlyphs(t, out)); strings.Contains(got, "Secret") || !strings.Contains(got, "Kept") {
				t.Errorf("page shows %q", got)
			}
			if got := overlayText(t, out, 0); !strings.Contains(got, c.text) || (c.text == "") != (got == "") {
				t.Errorf("mark shows %q, want %q", got, c.text)
			}
			checkNames(t, out, pageAt(t, out, 0))
			if c.name == "overlay form" && streamsContaining(t, out, "OVERLAYFORM") != 1 {
				t.Error("the overlay form is missing")
			}
		})
	}
}

// TestOverlayBudget checks that the overlay text of the marks counts
// towards the work which Write may do.
func TestOverlayBudget(t *testing.T) {
	build := func(n int) *pdf.Reader {
		regs := make([]pdf.Dict, n)
		for i := range regs {
			regs[i] = pdf.Dict{
				"Rect":        rect(0, 0, 300, 300),
				"OverlayText": pdf.String("AB"),
				"DA":          pdf.String("/Helv 1 Tf"),
				"Repeat":      pdf.Boolean(true),
			}
		}
		return nestDoc(t, regs, func(b *nestBuilder) []nestPage {
			return []nestPage{{content: "0 0 1 1 re f"}}
		})
	}
	if err := Write(io.Discard, build(1), nil); err != nil {
		t.Errorf("one mark: %v", err)
	}
	if err := Write(io.Discard, build(100), nil); !errors.Is(err, ErrTooComplex) {
		t.Errorf("many marks: got error %v, want ErrTooComplex", err)
	}
}
