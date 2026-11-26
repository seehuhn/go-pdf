// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/numtree"
	"seehuhn.de/go/pdf/pagetree"
)

// makeSource builds a three-page document.  The page tree root carries
// MediaBox, Rotate and Resources, which page 1 overrides with its own
// MediaBox.  Page 0 holds a form field, and the catalog has an outline.
func makeSource(t testing.TB, v pdf.Version, catVersion pdf.Version, opt *pdf.WriterOptions) *pdf.Reader {
	t.Helper()
	w, buf := memfile.NewPDFWriter(t, v, opt)

	pagesRef := w.Alloc()
	pageRefs := []pdf.Reference{w.Alloc(), w.Alloc(), w.Alloc()}
	fieldRef := w.Alloc()
	acroRef := w.Alloc()
	outlinesRef := w.Alloc()
	itemRef := w.Alloc()
	fontRef := w.Alloc()

	objs := map[pdf.Reference]pdf.Object{
		pagesRef: pdf.Dict{
			"Type":     pdf.Name("Pages"),
			"Kids":     pdf.Array{pageRefs[0], pageRefs[1], pageRefs[2]},
			"Count":    pdf.Integer(3),
			"MediaBox": pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(200), pdf.Integer(300)},
			"Rotate":   pdf.Integer(90),
			"Resources": pdf.Dict{
				"Font": pdf.Dict{"F1": fontRef},
			},
		},
		fontRef: pdf.Dict{
			"Type":     pdf.Name("Font"),
			"Subtype":  pdf.Name("Type1"),
			"BaseFont": pdf.Name("Helvetica"),
		},
		pageRefs[0]: pdf.Dict{
			"Type":   pdf.Name("Page"),
			"Parent": pagesRef,
			"Annots": pdf.Array{fieldRef},
		},
		pageRefs[1]: pdf.Dict{
			"Type":     pdf.Name("Page"),
			"Parent":   pagesRef,
			"MediaBox": pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(100), pdf.Integer(100)},
		},
		pageRefs[2]: pdf.Dict{
			"Type":   pdf.Name("Page"),
			"Parent": pagesRef,
			"Rotate": pdf.Integer(180),
		},
		fieldRef: pdf.Dict{
			"Type":    pdf.Name("Annot"),
			"Subtype": pdf.Name("Widget"),
			"Rect":    pdf.Array{pdf.Integer(10), pdf.Integer(10), pdf.Integer(50), pdf.Integer(30)},
			"P":       pageRefs[0],
			"FT":      pdf.Name("Tx"),
			"T":       pdf.String("name"),
			"V":       pdf.String("value"),
		},
		acroRef: pdf.Dict{
			"Fields": pdf.Array{fieldRef},
		},
		outlinesRef: pdf.Dict{
			"Type":  pdf.Name("Outlines"),
			"First": itemRef,
			"Last":  itemRef,
			"Count": pdf.Integer(1),
		},
		itemRef: pdf.Dict{
			"Title":  pdf.String("Chapter One"),
			"Parent": outlinesRef,
			"Dest":   pdf.Array{pageRefs[2], pdf.Name("Fit")},
		},
	}
	for ref, obj := range objs {
		if err := w.Put(ref, obj); err != nil {
			t.Fatal(err)
		}
	}

	meta := w.GetMeta()
	meta.Catalog.Pages = pagesRef
	meta.Catalog.Outlines = outlinesRef
	if v >= pdf.V1_2 {
		meta.Catalog.AcroForm = acroRef
	}
	meta.Catalog.Version = catVersion
	meta.Info = &pdf.Info{Title: pdf.Text("a title")}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	pwd := ""
	if opt != nil {
		pwd = opt.UserPassword
	}
	return openPDFPassword(t, buf.Data, pwd)
}

// openPDF opens the PDF file data.
func openPDF(t testing.TB, data []byte) *pdf.Reader {
	t.Helper()
	return openPDFPassword(t, data, "")
}

// openPDFPassword opens the PDF file data, using the given user password.
func openPDFPassword(t testing.TB, data []byte, pwd string) *pdf.Reader {
	t.Helper()
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)),
		&pdf.ReaderOptions{Password: pwd})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// writeAndRead applies the redactions of r with [Write] and reads the output.
// The argument pwd is the password needed to read the output.
func writeAndRead(t testing.TB, r *pdf.Reader, opts *Options, pwd string) *pdf.Reader {
	t.Helper()
	var out bytes.Buffer
	if err := Write(&out, r, opts); err != nil {
		t.Fatal(err)
	}
	return openPDFPassword(t, out.Bytes(), pwd)
}

// pageViews returns the inheritance-resolved page dictionaries, with
// references removed so that two documents can be compared.
func pageViews(t testing.TB, r *pdf.Reader) []pdf.Dict {
	t.Helper()
	var res []pdf.Dict
	it := pagetree.NewIterator(r)
	for _, dict := range it.All() {
		d := pdf.Dict{}
		for _, key := range []pdf.Name{"MediaBox", "Rotate"} {
			obj, err := pdf.Resolve(r, dict[key])
			if err != nil {
				t.Fatal(err)
			}
			d[key] = obj
		}
		fonts, err := pdf.CursorAt(pdf.NewExtractor(r), nil).Dict(dict["Resources"])
		if err != nil {
			t.Fatal(err)
		}
		if fonts != nil {
			f, err := pdf.Resolve(r, fonts["Font"])
			if err != nil {
				t.Fatal(err)
			}
			if fd, ok := f.(pdf.Dict); ok {
				names := pdf.Array{}
				for name := range fd {
					names = append(names, name)
				}
				d["Fonts"] = names
			}
		}
		res = append(res, d)
	}
	if it.Err != nil {
		t.Fatal(it.Err)
	}
	return res
}

func TestWriteCopiesDocument(t *testing.T) {
	r := makeSource(t, pdf.V1_7, 0, nil)
	rr := writeAndRead(t, r, nil, "")

	want := pageViews(t, r)
	got := pageViews(t, rr)
	if len(want) != 3 {
		t.Fatalf("test input has %d pages, want 3", len(want))
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("pages differ (-want +got):\n%s", diff)
	}

	// outline title and destination
	meta := rr.GetMeta()
	outlines, err := pdf.Resolve(rr, meta.Catalog.Outlines)
	if err != nil {
		t.Fatal(err)
	}
	item, err := pdf.Resolve(rr, outlines.(pdf.Dict)["First"])
	if err != nil {
		t.Fatal(err)
	}
	title, _ := item.(pdf.Dict)["Title"].(pdf.String)
	if string(title) != "Chapter One" {
		t.Errorf("outline title = %q, want %q", title, "Chapter One")
	}
	var pageRefs []pdf.Reference
	for ref := range pagetree.NewIterator(rr).All() {
		pageRefs = append(pageRefs, ref)
	}
	dest := item.(pdf.Dict)["Dest"].(pdf.Array)
	if dest[0] != pageRefs[2] {
		t.Errorf("outline destination = %v, want page %v", dest[0], pageRefs[2])
	}

	// form field: listed in the AcroForm and the widget of page 0
	acro, err := pdf.Resolve(rr, meta.Catalog.AcroForm)
	if err != nil {
		t.Fatal(err)
	}
	fields := acro.(pdf.Dict)["Fields"].(pdf.Array)
	if len(fields) != 1 {
		t.Fatalf("got %d fields, want 1", len(fields))
	}
	field, err := pdf.Resolve(rr, fields[0])
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := field.(pdf.Dict)["T"].(pdf.String); string(v) != "name" {
		t.Errorf("field name = %q, want %q", v, "name")
	}
	page0, err := pdf.Resolve(rr, pageRefs[0])
	if err != nil {
		t.Fatal(err)
	}
	annots := page0.(pdf.Dict)["Annots"].(pdf.Array)
	if len(annots) != 1 || annots[0] != fields[0] {
		t.Errorf("page annotations = %v, want the field %v", annots, fields[0])
	}
	if p := field.(pdf.Dict)["P"]; p != pageRefs[0] {
		t.Errorf("widget page = %v, want %v", p, pageRefs[0])
	}

	if meta.Info == nil || meta.Info.Title != pdf.Text("a title") {
		t.Errorf("info is %v, want title %q", meta.Info, "a title")
	}
}

func TestWriteIdentifier(t *testing.T) {
	in := makeSource(t, pdf.V1_7, 0, &pdf.WriterOptions{
		ID: [][]byte{[]byte("0123456789abcdef"), []byte("fedcba9876543210")},
	})
	out := writeAndRead(t, in, nil, "")

	id := out.GetMeta().ID
	if len(id) != 2 {
		t.Fatalf("output has %d identifiers, want 2", len(id))
	}
	if !bytes.Equal(id[0], in.GetMeta().ID[0]) {
		t.Error("permanent identifier changed")
	}
	if bytes.Equal(id[1], in.GetMeta().ID[1]) {
		t.Error("changing identifier was kept")
	}
}

func TestWriteVersion(t *testing.T) {
	cases := []struct {
		name    string
		header  pdf.Version
		catalog pdf.Version
		want    pdf.Version
	}{
		{"raised", pdf.V1_1, 0, pdf.V1_2},
		{"kept", pdf.V1_5, 0, pdf.V1_5},
		{"catalog later", pdf.V1_5, pdf.V1_7, pdf.V1_7},
		{"catalog 2.0", pdf.V1_7, pdf.V2_0, pdf.V2_0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := makeSource(t, c.header, c.catalog, nil)
			rr := writeAndRead(t, r, nil, "")
			if got := rr.GetMeta().Version; got != c.want {
				t.Errorf("version = %v, want %v", got, c.want)
			}
		})
	}
}

func TestWriteEncrypted(t *testing.T) {
	r := makeSource(t, pdf.V1_7, 0, &pdf.WriterOptions{UserPassword: "secret"})

	var out bytes.Buffer
	err := Write(&out, r, nil)
	if !errors.Is(err, ErrEncrypted) {
		t.Errorf("got error %v, want ErrEncrypted", err)
	}

	rr := writeAndRead(t, r, &Options{Writer: &pdf.WriterOptions{UserPassword: "other"}}, "other")
	if rr.GetMeta().Encryption == nil {
		t.Error("output is not encrypted")
	}
	if got := len(pageViews(t, rr)); got != 3 {
		t.Errorf("got %d pages, want 3", got)
	}
}

// TestCatalogFieldsHandled checks that copyCatalog accounts for every field
// of [pdf.Catalog].
func TestCatalogFieldsHandled(t *testing.T) {
	// the fields which copyCatalog copies, overrides or drops
	handled := map[string]bool{}
	for _, name := range []string{
		"Version", "Extensions", "Pages", "PageLabels", "Names", "Dests",
		"ViewerPreferences", "PageLayout", "PageMode", "Outlines", "Threads",
		"OpenAction", "AA", "URI", "AcroForm", "Metadata", "StructTreeRoot",
		"MarkInfo", "Lang", "SpiderInfo", "OutputIntents", "PieceInfo",
		"OCProperties", "Perms", "Legal", "Requirements", "Collection",
		"NeedsRendering", "DSS", "AF", "DPartRoot",
	} {
		handled[name] = true
	}
	typ := reflect.TypeFor[pdf.Catalog]()
	for field := range typ.Fields() {
		name := field.Name
		if !handled[name] {
			t.Errorf("catalog field %s is not handled by copyCatalog", name)
		}
		delete(handled, name)
	}
	for name := range handled {
		t.Errorf("copyCatalog lists unknown catalog field %s", name)
	}
}

// TestWriteRaisesVersionForCatalog checks that catalog entries newer than
// the file header do not make the redaction fail.
func TestWriteRaisesVersionForCatalog(t *testing.T) {
	w, buf := memfile.NewPDFWriter(t, pdf.V1_5, nil)
	pagesRef := w.Alloc()
	pageRef := w.Alloc()
	err := w.Put(pagesRef, pdf.Dict{
		"Type":  pdf.Name("Pages"),
		"Kids":  pdf.Array{pageRef},
		"Count": pdf.Integer(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	err = w.Put(pageRef, pdf.Dict{
		"Type":     pdf.Name("Page"),
		"Parent":   pagesRef,
		"MediaBox": pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(100), pdf.Integer(100)},
	})
	if err != nil {
		t.Fatal(err)
	}
	cat := w.GetMeta().Catalog
	cat.Pages = pagesRef
	cat.OCProperties = pdf.Dict{"OCGs": pdf.Array{}}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// declare a version older than the catalog entries need
	data := bytes.Replace(buf.Data, []byte("%PDF-1.5"), []byte("%PDF-1.1"), 1)
	r := openPDF(t, data)
	if got := r.GetMeta().Version; got != pdf.V1_1 {
		t.Fatalf("input version = %v, want 1.1", got)
	}

	rr := writeAndRead(t, r, nil, "")
	if got := rr.GetMeta().Version; got < pdf.V1_5 {
		t.Errorf("output version = %v, want at least 1.5", got)
	}
	if rr.GetMeta().Catalog.OCProperties == nil {
		t.Error("lost /OCProperties")
	}
}

// structSecrets are the text entries of the structure elements of
// [structDoc] which the redaction removes.
var structSecrets = []string{"secretAT", "secretAlt", "secretE", "docalt"}

// resolveDict resolves obj to a dictionary in r.  The dictionary of a
// stream is returned for a stream.
func resolveDict(t testing.TB, r *pdf.Reader, obj pdf.Object) pdf.Dict {
	t.Helper()
	v, err := pdf.Resolve(r, obj)
	if err != nil {
		t.Fatal(err)
	}
	switch v := v.(type) {
	case pdf.Dict:
		return v
	case *pdf.Stream:
		return v.Dict
	}
	t.Fatalf("%v is not a dictionary", obj)
	return nil
}

// outElemRef returns the reference of the structure element for entry
// (key, idx) of the parent tree of the output.  If idx is negative, the
// entry is the element itself.
func outElemRef(t testing.TB, r *pdf.Reader, key, idx int) pdf.Reference {
	t.Helper()
	root := resolveDict(t, r, r.GetMeta().Catalog.StructTreeRoot)
	pt, err := numtree.ExtractFromFile(r, root["ParentTree"])
	if err != nil {
		t.Fatal(err)
	}
	val, err := pt.Lookup(pdf.Integer(key))
	if err != nil {
		t.Fatal(err)
	}
	if idx >= 0 {
		arr, err := pdf.Resolve(r, val)
		if err != nil {
			t.Fatal(err)
		}
		val = arr.(pdf.Array)[idx]
	}
	ref, ok := val.(pdf.Reference)
	if !ok {
		t.Fatalf("parent tree entry %d is not a reference", key)
	}
	return ref
}

// checkLinkedElem checks that got, which the output reaches by a path
// outside the structure tree, is the structure element of parent tree entry
// (key, idx), and that the element is stripped.  The secrets must be absent
// from the output data.
func checkLinkedElem(t testing.TB, out *pdf.Reader, data []byte, got pdf.Object, key, idx int, secrets ...string) {
	t.Helper()
	if want := outElemRef(t, out, key, idx); got != want {
		t.Errorf("element is reached as %v, but the structure tree has %v", got, want)
	}
	checkStripped(t, "element", outElem(t, out, key, idx))
	for _, s := range secrets {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("output still contains %q", s)
		}
	}
}

// pageAnnot returns the annotation of the first page of r for which match
// is true.
func pageAnnot(t testing.TB, r *pdf.Reader, match func(d pdf.Dict) bool) pdf.Dict {
	t.Helper()
	annots, err := pdf.Resolve(r, pageAt(t, r, 0)["Annots"])
	if err != nil {
		t.Fatal(err)
	}
	arr, _ := annots.(pdf.Array)
	for _, obj := range arr {
		if d := resolveDict(t, r, obj); match(d) {
			return d
		}
	}
	t.Fatal("annotation not found")
	return nil
}

// pageResource returns the dictionaries of a resource category of the first
// page of r.
func pageResource(t testing.TB, r *pdf.Reader, category pdf.Name) map[pdf.Name]pdf.Dict {
	t.Helper()
	res := resolveDict(t, r, pageAt(t, r, 0)["Resources"])
	cat := resolveDict(t, r, res[category])
	m := map[pdf.Name]pdf.Dict{}
	for name, obj := range cat {
		m[name] = resolveDict(t, r, obj)
	}
	return m
}

// path follows a sequence of dictionary keys from d.
func path(t testing.TB, r *pdf.Reader, d pdf.Dict, keys ...pdf.Name) pdf.Object {
	t.Helper()
	var obj pdf.Object = d
	for i, k := range keys {
		if i > 0 {
			d = resolveDict(t, r, obj)
		}
		obj = d[k]
	}
	return obj
}

// The tests below check that objects reached from the rewritten parts of a
// document are copied only after all redirections are known.

func TestRewrittenAnnotReachesElement(t *testing.T) {
	// a kept annotation which loses its pop-up is written again; it reaches
	// the element of the removed text through a link
	in := structDoc(t, func(p *structParts) {
		linkRef, popupRef, textRef := p.alloc(), p.alloc(), p.alloc()
		p.extra[linkRef] = pdf.Dict{
			"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Link"), "Rect": rect(50, 200, 70, 220),
			"A": pdf.Dict{
				"S": pdf.Name("GoTo"), "D": pdf.String("dest"),
				"SD": pdf.Array{p.pRef, pdf.Name("Fit")},
			},
		}
		p.extra[popupRef] = pdf.Dict{
			"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Popup"), "Rect": rect(10, 40, 30, 60),
			"Parent": textRef,
		}
		p.extra[textRef] = pdf.Dict{
			"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Text"), "Rect": rect(80, 200, 100, 220),
			"Popup": popupRef, "IRT": linkRef, "Contents": pdf.String("reply"),
		}
		p.moreAnnots = append(p.moreAnnots, linkRef, textRef, popupRef)
	})
	out, data := redactStruct(t, in)

	text := pageAnnot(t, out, func(d pdf.Dict) bool { return isString(d["Contents"], "reply") })
	if _, ok := text["Popup"]; ok {
		t.Error("annotation keeps its deleted pop-up")
	}
	sd, err := pdf.Resolve(out, path(t, out, text, "IRT", "A", "SD"))
	if err != nil {
		t.Fatal(err)
	}
	checkLinkedElem(t, out, data, sd.(pdf.Array)[0], 0, 0, structSecrets...)
}

func TestRewrittenWidgetReachesElement(t *testing.T) {
	// a kept widget of a field which loses its value is written again; its
	// action reaches the structure element of the deleted widget
	in, _ := formDocWith(t, false, func(p *formParts, objs map[pdf.Reference]pdf.Dict) {
		objs[p.elem]["ActualText"] = pdf.String("SECRETELEM")
		objs[p.w2b]["A"] = pdf.Dict{
			"S": pdf.Name("GoTo"), "D": pdf.String("dest"),
			"SD": pdf.Array{p.elem, pdf.Name("Fit")},
		}
	})
	o := redactForm(t, in)
	kids := o.kids(o.field("double"))
	if len(kids) != 1 {
		t.Fatalf("field has %d widgets, want 1", len(kids))
	}
	sd, err := pdf.Resolve(o.r, path(t, o.r, kids[0], "A", "SD"))
	if err != nil {
		t.Fatal(err)
	}
	checkLinkedElem(t, o.r, o.data, sd.(pdf.Array)[0], 1, -1, "SECRETELEM")
}

func TestKeptFormReachesElement(t *testing.T) {
	// a form which is kept on a rewritten page refers to the element of
	// the removed text
	in := structDoc(t, func(p *structParts) {
		p.keptForm = pdf.Dict{"PieceInfo": pdf.Dict{"App": pdf.Dict{"Private": p.pRef}}}
	})
	out, data := redactStruct(t, in)
	fm := pageResource(t, out, "XObject")["Fm2"]
	if fm == nil {
		t.Fatal("kept form is missing")
	}
	got := path(t, out, fm, "PieceInfo", "App", "Private")
	checkLinkedElem(t, out, data, got, 0, 0, structSecrets...)
}

func TestKeptFormReachesDeletedAnnot(t *testing.T) {
	// a form which is kept on a rewritten page refers to a deleted
	// annotation
	in := structDoc(t, func(p *structParts) {
		note := p.alloc()
		p.extra[note] = pdf.Dict{
			"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Text"), "Rect": rect(10, 40, 30, 60),
			"Contents": pdf.String("SECRETNOTE"),
		}
		p.moreAnnots = append(p.moreAnnots, note)
		p.keptForm = pdf.Dict{"PieceInfo": pdf.Dict{"App": pdf.Dict{"Private": note}}}
	})
	_, data := redactStruct(t, in)
	if bytes.Contains(data, []byte("SECRETNOTE")) {
		t.Error("output contains the deleted annotation")
	}
}

func TestKeptFormReachesLaterDeletedAnnot(t *testing.T) {
	// the annotation is deleted on a page after the page of the form
	in := nestDoc(t, []pdf.Dict{{"Rect": rect(0, 0, 300, 100)}}, func(b *nestBuilder) []nestPage {
		note := b.put(pdf.Dict{
			"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Text"), "Rect": rect(10, 40, 30, 60),
			"Contents": pdf.String("SECRETNOTE"),
		})
		reg := b.put(pdf.Dict{
			"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Redact"), "Rect": rect(0, 0, 300, 100),
		})
		fm := b.form(pdf.Dict{
			"Resources": helv(),
			"PieceInfo": pdf.Dict{"App": pdf.Dict{"Private": note}},
		}, "BT /F1 12 Tf 10 280 Td (keep) Tj ET")
		return []nestPage{
			{content: "q /Fm1 Do Q", res: pdf.Dict{"XObject": pdf.Dict{"Fm1": fm}}},
			{annots: []pdf.Object{reg, note}},
		}
	})
	_, data := redactStruct(t, in)
	if bytes.Contains(data, []byte("SECRETNOTE")) {
		t.Error("output contains the deleted annotation")
	}
}

func TestStrippedPropertyListReachesElement(t *testing.T) {
	// a property list which loses its text refers to the element of the
	// removed text
	in := structDoc(t, func(p *structParts) {
		p.content = "/P /MC0 BDC BT /F1 12 Tf 10 20 Td (secret) Tj ET EMC\n" +
			"/P <</MCID 1>> BDC BT /F1 12 Tf 10 250 Td (keep) Tj ET EMC\n"
		p.props = pdf.Dict{"MC0": pdf.Dict{
			"MCID": pdf.Integer(0), "ActualText": pdf.String("propsecret"), "Ref": p.pRef,
		}}
	})
	out, data := redactStruct(t, in)
	var refs []pdf.Object
	for _, d := range pageResource(t, out, "Properties") {
		if r, ok := d["Ref"]; ok {
			refs = append(refs, r)
		}
	}
	if len(refs) != 1 {
		t.Fatalf("%d property lists refer to the element, want 1", len(refs))
	}
	checkLinkedElem(t, out, data, refs[0], 0, 0, append(structSecrets, "propsecret")...)
}

func TestRewrittenFormReachesElement(t *testing.T) {
	// the resources of a rewritten form refer to the element of the
	// removed text
	in := structDoc(t, func(p *structParts) {
		p.form = pdf.Dict{"StructParents": pdf.Integer(5)}
		p.formRes = pdf.Dict{"Properties": pdf.Dict{"X": pdf.Dict{"Ref": p.pRef}}}
	})
	out, data := redactStruct(t, in)
	var refs []pdf.Object
	for _, fm := range pageResource(t, out, "XObject") {
		if fm["Subtype"] != pdf.Name("Form") || fm["Resources"] == nil {
			continue
		}
		res := resolveDict(t, out, fm["Resources"])
		if res["Properties"] != nil {
			refs = append(refs, path(t, out, res, "Properties", "X", "Ref"))
		}
	}
	if len(refs) != 1 {
		t.Fatalf("%d forms refer to the element, want 1", len(refs))
	}
	checkLinkedElem(t, out, data, refs[0], 0, 0, append(structSecrets, "formAT")...)
}

func TestSoftMaskStateReachesDeletedAnnot(t *testing.T) {
	// an extended graphics state whose soft mask is rewritten refers to a
	// deleted annotation
	in := nestDoc(t, []pdf.Dict{{"Rect": rect(0, 0, 300, 100)}}, func(b *nestBuilder) []nestPage {
		note := b.put(pdf.Dict{
			"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Text"), "Rect": rect(10, 40, 30, 60),
			"Contents": pdf.String("SECRETNOTE"),
		})
		group := b.form(pdf.Dict{
			"Resources": helv(),
			"Group":     pdf.Dict{"S": pdf.Name("Transparency")},
		}, "BT /F1 12 Tf 10 20 Td (masked) Tj ET")
		gs := pdf.Dict{
			"Type": pdf.Name("ExtGState"),
			"SMask": pdf.Dict{
				"Type": pdf.Name("Mask"), "S": pdf.Name("Luminosity"), "G": group,
				"Private": note,
			},
			"Private": note,
		}
		return []nestPage{{
			content: "/GS0 gs 0 0 300 300 re f",
			res:     pdf.Dict{"ExtGState": pdf.Dict{"GS0": gs}},
			annots:  []pdf.Object{note},
		}}
	})
	out, data := redactStruct(t, in)
	if bytes.Contains(data, []byte("SECRETNOTE")) {
		t.Error("output contains the deleted annotation")
	}
	if n := streamsContaining(t, out, "masked"); n != 0 {
		t.Errorf("%d streams still hold the text of the soft mask", n)
	}
}

func TestWriteNoPages(t *testing.T) {
	// the page tree of the catalog is the catalog itself
	data := []byte("%PDF-1.0\n1 0 obj<</Type/Catalog/Pages 1 0 R>>endobj\n")
	r := openPDF(t, data)
	var buf bytes.Buffer
	if err := Write(&buf, r, nil); err == nil {
		t.Error("document without pages was written")
	}
	if buf.Len() > 0 {
		t.Errorf("%d bytes were written", buf.Len())
	}
}

func TestWriteUnusableRedaction(t *testing.T) {
	// The first page has a usable redaction, the second an unusable one.
	// The content of the first page is long enough that its rewritten copy
	// would not fit into an output buffer.
	var b strings.Builder
	x := uint32(1)
	for range 5000 {
		x = x*1664525 + 1013904223
		fmt.Fprintf(&b, "%d %d m %d %d l S\n", x%300, x>>8%300, x>>16%300, x>>24)
	}
	in := pagesDoc(t, 2, func(w *pdf.Writer, refs []pdf.Reference, pages []pdf.Dict) {
		pages[0]["Contents"] = putRawStream(t, w, pdf.Dict{}, []byte(b.String()))
		pages[1]["Contents"] = putRawStream(t, w, pdf.Dict{}, []byte("0 0 10 10 re f\n"))
		addRedaction(t, w, refs[0], pages[0], rect(100, 100, 200, 200))
		addRedaction(t, w, refs[1], pages[1], rect(40, 90, 40, 115))
	})
	var buf bytes.Buffer
	err := Write(&buf, in, nil)
	var e *UnusableRedactionError
	if !errors.As(err, &e) {
		t.Fatalf("got error %v, want UnusableRedactionError", err)
	}
	if e.Page != 1 {
		t.Errorf("error for page %d, want 1", e.Page)
	}
	if buf.Len() > 0 {
		t.Errorf("%d bytes were written", buf.Len())
	}
}

// checkStable checks that writing the document r, and then writing the
// result again, gives the same file.  The document r is typically the output
// of [Write] for a malformed input.
func checkStable(t testing.TB, r *pdf.Reader) {
	t.Helper()
	var first, second bytes.Buffer
	if err := Write(&first, r, nil); err != nil {
		t.Fatal(err)
	}
	if err := Write(&second, openPDF(t, first.Bytes()), nil); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Error("read-write-read cycle is not stable")
	}
}

// TestWriteReadError checks that Write fails if the stream data of a
// redacted image cannot be read from the file.  Only the middle of the image
// data fails, so that buffered reads of the neighbouring objects succeed.
func TestWriteReadError(t *testing.T) {
	const mw, mh = 300, 300
	const margin = 64 << 10
	samples := bytes.Repeat([]byte{0xC3, 0x5A, 0x96}, mw*mh)
	data := imageDocData(t, pdf.V1_7, func(w *pdf.Writer, rm *pdf.ResourceManager) []imgPage {
		ref := putRawStream(t, w, pdf.Dict{
			"Subtype": pdf.Name("Image"),
			"Width":   pdf.Integer(mw), "Height": pdf.Integer(mh),
			"ColorSpace":       pdf.Name("DeviceRGB"),
			"BitsPerComponent": pdf.Integer(8),
		}, samples)
		return []imgPage{{
			content: placeAt100, xobjects: pdf.Dict{"Im1": ref},
			redacts: [][4]int{region},
		}}
	})
	start := bytes.Index(data, samples)
	if start < 0 {
		t.Fatal("image data not found")
	}

	ioErr := errors.New("device failure")
	ra := &faultyReaderAt{
		data: data,
		from: int64(start + margin), to: int64(start + len(samples) - margin),
		err: ioErr,
	}
	r, err := pdf.NewReader(ra, int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(io.Discard, r, nil); !errors.Is(err, ioErr) {
		t.Errorf("got error %v, want the read error", err)
	}
}

// rawDoc returns a PDF file with the given header version, whose objects
// 1, 2, ... are objs.  Object 1 is the catalog.  A non-empty trailerExtra
// is added to the trailer dictionary.
func rawDoc(version, trailerExtra string, objs ...string) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "%%PDF-%s\n", version)
	offsets := make([]int, len(objs))
	for i, obj := range objs {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<</Size %d/Root 1 0 R%s>>\nstartxref\n%d\n%%%%EOF\n",
		len(objs)+1, trailerExtra, xref)
	return b.Bytes()
}

// rawStream returns the source of a stream object with the given data.
func rawStream(data string) string {
	return fmt.Sprintf("<</Length %d>>\nstream\n%s\nendstream", len(data), data)
}

// TestWriteLeafWithoutType checks that a page tree leaf without /Type is
// redacted like any other page, and that no reference brings back an
// original node of the page tree.
func TestWriteLeafWithoutType(t *testing.T) {
	data := rawDoc("1.7", "",
		"<</Type/Catalog/Pages 2 0 R/Outlines 7 0 R>>",
		"<</Type/Pages/Kids[3 0 R 9 0 R]/Count 2>>",
		"<</Parent 2 0 R/MediaBox[0 0 200 200]/Resources<</Font<</F1 5 0 R>>>>/Contents 4 0 R/Annots[6 0 R]>>",
		rawStream("BT /F1 12 Tf 10 50 Td (Secret) Tj ET"),
		"<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>",
		"<</Type/Annot/Subtype/Redact/Rect[0 0 200 200]>>",
		"<</Type/Outlines/First 8 0 R/Last 8 0 R/Count 1>>",
		"<</Title(leaf)/Parent 7 0 R/Next 10 0 R/Dest[3 0 R/Fit]>>",
		"<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 200]>>",
		"<</Title(root)/Parent 7 0 R/Prev 8 0 R/Dest[2 0 R/Fit]>>",
	)
	out := writeAndRead(t, openPDF(t, data), nil, "")

	refs, _ := pageDicts(t, out)
	if len(refs) != 2 {
		t.Fatalf("got %d pages, want 2", len(refs))
	}
	if n := streamsContaining(t, out, "Secret"); n != 0 {
		t.Errorf("%d streams hold the redacted text", n)
	}
	isRedact := func(obj pdf.Native) bool { return dictOf(obj)["Subtype"] == pdf.Name("Redact") }
	if n := countObjects(t, out, isRedact); n != 0 {
		t.Errorf("%d redaction annotations remain", n)
	}
	isPages := func(obj pdf.Native) bool { return dictOf(obj)["Type"] == pdf.Name("Pages") }
	if n := countObjects(t, out, isPages); n != 1 {
		t.Errorf("%d page tree nodes, want 1", n)
	}

	outlines := resolveDict(t, out, out.GetMeta().Catalog.Outlines)
	item := resolveDict(t, out, outlines["First"])
	dest, _ := item["Dest"].(pdf.Array)
	if len(dest) == 0 || dest[0] != refs[0] {
		t.Errorf("destination %v does not refer to the first page %v", dest, refs[0])
	}
	checkStable(t, out)
}

// inheritDoc returns a document with the given number of pages, which
// inherit a direct resource dictionary with the given number of fonts from
// the page tree root.  If redacted is set, every page has a redaction
// annotation.
func inheritDoc(pages, fonts int, redacted bool) []byte {
	var fontList, kids strings.Builder
	for i := range fonts {
		fmt.Fprintf(&fontList, "/F%d 3 0 R", i)
	}
	objs := []string{
		"<</Type/Catalog/Pages 2 0 R>>",
		"", // the page tree root
		"<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>",
		"<</Type/Annot/Subtype/Redact/Rect[0 0 10 10]>>",
	}
	for range pages {
		fmt.Fprintf(&kids, "%d 0 R ", len(objs)+1)
		page := "<</Type/Page/Parent 2 0 R>>"
		if redacted {
			page = "<</Type/Page/Parent 2 0 R/Annots[4 0 R]>>"
		}
		objs = append(objs, page)
	}
	objs[1] = fmt.Sprintf("<</Type/Pages/MediaBox[0 0 100 100]/Resources<</Font<<%s>>>>/Kids[%s]/Count %d>>",
		fontList.String(), kids.String(), pages)
	return rawDoc("1.7", "", objs...)
}

// TestWriteInheritedResources checks that resources which many pages inherit
// from an intermediate node of the page tree are not copied for each page:
// the output grows with the resources in proportion to the input.
func TestWriteInheritedResources(t *testing.T) {
	const pages, fonts = 200, 200
	opts := &Options{Writer: &pdf.WriterOptions{HumanReadable: true}}
	for _, redacted := range []bool{false, true} {
		var sizeIn, sizeOut [2]int
		for i, k := range []int{1, fonts} {
			data := inheritDoc(pages, k, redacted)
			var buf bytes.Buffer
			if err := Write(&buf, openPDF(t, data), opts); err != nil {
				t.Fatal(err)
			}
			sizeIn[i], sizeOut[i] = len(data), buf.Len()

			out := openPDF(t, buf.Bytes())
			for j, view := range pageViews(t, out) {
				if names, _ := view["Fonts"].(pdf.Array); len(names) != k {
					t.Fatalf("redacted=%t: page %d has %d fonts, want %d", redacted, j, len(names), k)
				}
			}
		}
		if dIn, dOut := sizeIn[1]-sizeIn[0], sizeOut[1]-sizeOut[0]; dOut > 4*dIn {
			t.Errorf("redacted=%t: %d more bytes of input give %d more bytes of output",
				redacted, dIn, dOut)
		}
	}
}

// idDoc returns a one-page document with the given header version and, if
// id is not empty, the file identifier [id id].
func idDoc(t *testing.T, version, id string) *pdf.Reader {
	t.Helper()
	extra := ""
	if id != "" {
		extra = fmt.Sprintf("/ID[(%s)(%s)]", id, id)
	}
	return openPDF(t, rawDoc(version, extra,
		"<</Type/Catalog/Pages 2 0 R>>",
		"<</Type/Pages/Kids[3 0 R]/Count 1>>",
		"<</Type/Page/Parent 2 0 R/MediaBox[0 0 100 100]>>",
	))
}

func TestWriteIdentifierCases(t *testing.T) {
	const long = "0123456789abcdef"
	encrypt := &Options{Writer: &pdf.WriterOptions{UserPassword: "pw"}}
	cases := []struct {
		name     string
		version  string
		id       string
		opts     *Options
		pwd      string
		wantID   bool
		keepPerm bool
	}{
		{"kept", "1.7", long, nil, "", true, true},
		{"short 1.7", "1.7", "abc", nil, "", true, true},
		{"short 2.0", "2.0", "abc", nil, "", true, false},
		{"long 2.0", "2.0", long, nil, "", true, true},
		{"none 1.7", "1.7", "", nil, "", false, false},
		{"none 2.0", "2.0", "", nil, "", true, false},
		{"none encrypted", "1.7", "", encrypt, "pw", true, false},
		{"kept encrypted", "1.7", long, encrypt, "pw", true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := writeAndRead(t, idDoc(t, c.version, c.id), c.opts, c.pwd)
			id := out.GetMeta().ID
			if !c.wantID {
				if id != nil {
					t.Errorf("output has identifier %q", id)
				}
				return
			}
			if len(id) != 2 {
				t.Fatalf("output has %d identifiers, want 2", len(id))
			}
			if got := string(id[0]) == c.id; got != c.keepPerm {
				t.Errorf("permanent identifier kept: %t, want %t", got, c.keepPerm)
			}
			if string(id[1]) == c.id {
				t.Error("changing identifier was kept")
			}
		})
	}
}

// TestUnusableRedactionError checks that the error is found through a
// wrapping error.
func TestUnusableRedactionError(t *testing.T) {
	err := fmt.Errorf("context: %w", &UnusableRedactionError{Page: 3})
	var e *UnusableRedactionError
	if !errors.As(err, &e) {
		t.Fatal("errors.As does not find the error")
	}
	if e.Page != 3 {
		t.Errorf("page %d, want 3", e.Page)
	}
}
