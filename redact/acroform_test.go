// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"bytes"
	"maps"
	"strconv"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/decode"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// formParts holds the objects of the form test document.
type formParts struct {
	f1, w1                 pdf.Reference // single widget, under the region
	f2, w2a, w2b           pdf.Reference // two widgets, one under the region
	f3, w3a, w3b           pdf.Reference // check box, two widgets
	g, g1, g2              pdf.Reference // group with an inherited value
	sig, f5                pdf.Reference
	elem, acroRef          pdf.Reference
	page0, page1, redactor pdf.Reference
}

// formDoc builds a document with two pages and an interactive form.  Page 0
// carries a redaction annotation, and the widgets named "under the region"
// lie below it.  With noRedact the annotation is missing.
func formDoc(t testing.TB, noRedact bool) (*pdf.Reader, *formParts) {
	t.Helper()
	return formDocWith(t, noRedact, nil)
}

// formDocWith is like formDoc.  The function tweak, if it is not nil, can
// change the field, widget and structure element dictionaries before they
// are written.
func formDocWith(t testing.TB, noRedact bool, tweak func(p *formParts, objs map[pdf.Reference]pdf.Dict)) (*pdf.Reader, *formParts) {
	t.Helper()
	w, buf := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	put := func(obj pdf.Object) pdf.Reference {
		ref := w.Alloc()
		if err := w.Put(ref, obj); err != nil {
			t.Fatal(err)
		}
		return ref
	}
	putStream := func(data string, extra pdf.Dict) pdf.Reference {
		ref := w.Alloc()
		d := pdf.Dict{}
		maps.Copy(d, extra)
		stm, err := w.OpenStream(ref, d)
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
	ap := func(text string) pdf.Dict {
		return pdf.Dict{"N": putStream("BT /Helv 12 Tf 2 2 Td ("+text+") Tj ET",
			pdf.Dict{"Type": pdf.Name("XObject"), "Subtype": pdf.Name("Form"),
				"BBox": rect(0, 0, 100, 20)})}
	}

	p := &formParts{}
	pagesRef := w.Alloc()
	p.page0, p.page1 = w.Alloc(), w.Alloc()
	p.f1, p.f2, p.w2a, p.w2b = w.Alloc(), w.Alloc(), w.Alloc(), w.Alloc()
	p.f3, p.w3a, p.w3b = w.Alloc(), w.Alloc(), w.Alloc()
	p.g, p.g1, p.g2, p.sig, p.f5 = w.Alloc(), w.Alloc(), w.Alloc(), w.Alloc(), w.Alloc()
	p.elem, p.acroRef = w.Alloc(), w.Alloc()
	p.w1 = p.f1 // merged

	widget := func(r pdf.Array, page pdf.Reference, extra pdf.Dict) pdf.Dict {
		d := pdf.Dict{
			"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget"),
			"Rect": r, "P": page, "F": pdf.Integer(4),
		}
		maps.Copy(d, extra)
		return d
	}
	da := pdf.String("/Helv 12 Tf 0 g")
	objs := map[pdf.Reference]pdf.Dict{
		p.f1: widget(rect(60, 95, 100, 110), p.page0, pdf.Dict{
			"FT": pdf.Name("Tx"), "T": pdf.String("single"), "DA": da,
			"V": pdf.String("SECRETSINGLE"), "AP": ap("SECRETSINGLE"),
			"StructParent": pdf.Integer(0),
		}),
		p.f2: {
			"FT": pdf.Name("Tx"), "T": pdf.String("double"), "DA": da,
			"V": pdf.String("DUPVALUE"), "DV": pdf.String("DUPDEFAULT"),
			"Kids": pdf.Array{p.w2a, p.w2b},
		},
		p.w2a: widget(rect(45, 92, 70, 100), p.page0, pdf.Dict{
			"Parent": p.f2, "AP": ap("DUPVALUE")}),
		p.w2b: widget(rect(10, 10, 100, 30), p.page1, pdf.Dict{
			"Parent": p.f2, "AP": ap("DUPVALUE")}),
		p.f3: {
			"FT": pdf.Name("Btn"), "T": pdf.String("check"),
			"V": pdf.Name("Yes"), "Kids": pdf.Array{p.w3a, p.w3b},
		},
		p.w3a: widget(rect(75, 92, 85, 100), p.page0, pdf.Dict{
			"Parent": p.f3, "AS": pdf.Name("Yes"),
			"AP": pdf.Dict{"N": pdf.Dict{
				"Yes": putStream("0 0 10 10 re f", pdf.Dict{"Subtype": pdf.Name("Form"), "BBox": rect(0, 0, 10, 10)}),
				"Off": putStream("", pdf.Dict{"Subtype": pdf.Name("Form"), "BBox": rect(0, 0, 10, 10)}),
			}},
		}),
		p.w3b: widget(rect(10, 40, 20, 50), p.page1, pdf.Dict{
			"Parent": p.f3, "AS": pdf.Name("Yes"),
			"AP": pdf.Dict{"N": pdf.Dict{
				"Yes": putStream("0 0 10 10 re f", pdf.Dict{"Subtype": pdf.Name("Form"), "BBox": rect(0, 0, 10, 10)}),
				"Off": putStream("", pdf.Dict{"Subtype": pdf.Name("Form"), "BBox": rect(0, 0, 10, 10)}),
			}},
		}),
		p.g: {
			"FT": pdf.Name("Tx"), "T": pdf.String("grp"), "DA": da,
			"V":    pdf.String("INHVALUE"),
			"Kids": pdf.Array{p.g1, p.g2},
		},
		p.g1: widget(rect(90, 105, 115, 112), p.page0, pdf.Dict{
			"Parent": p.g, "T": pdf.String("a"), "AP": ap("INHVALUE")}),
		p.g2: widget(rect(10, 60, 90, 80), p.page1, pdf.Dict{
			"Parent": p.g, "T": pdf.String("b"), "AP": ap("INHVALUE")}),
		p.sig: widget(rect(10, 100, 90, 120), p.page1, pdf.Dict{
			"FT": pdf.Name("Sig"), "T": pdf.String("sign"),
			"V": pdf.Dict{
				"Type": pdf.Name("Sig"), "Filter": pdf.Name("Adobe.PPKLite"),
				"Name": pdf.String("SIGSECRET"),
			},
		}),
		p.f5: widget(rect(10, 150, 90, 170), p.page1, pdf.Dict{
			"FT": pdf.Name("Tx"), "T": pdf.String("other"), "DA": da,
			"V": pdf.String("KEEPME"), "AP": ap("KEEPME"),
			"StructParent": pdf.Integer(1),
		}),
	}
	// structure tree with content items for two widgets
	structRoot := w.Alloc()
	objs[p.elem] = pdf.Dict{
		"Type": pdf.Name("StructElem"), "S": pdf.Name("Form"), "P": structRoot,
		"K": pdf.Array{
			pdf.Dict{"Type": pdf.Name("OBJR"), "Obj": p.f1, "Pg": p.page0},
			pdf.Dict{"Type": pdf.Name("OBJR"), "Obj": p.f5, "Pg": p.page1},
		},
	}
	if tweak != nil {
		tweak(p, objs)
	}
	for ref, d := range objs {
		if err := w.Put(ref, d); err != nil {
			t.Fatal(err)
		}
	}

	fontRef := put(pdf.Dict{
		"Type": pdf.Name("Font"), "Subtype": pdf.Name("Type1"),
		"BaseFont": pdf.Name("Helvetica"),
	})
	if err := w.Put(p.acroRef, pdf.Dict{
		"Fields":          pdf.Array{p.f1, p.f2, p.f3, p.g, p.sig, p.f5},
		"CO":              pdf.Array{p.f1, p.f5},
		"DA":              da,
		"DR":              pdf.Dict{"Font": pdf.Dict{"Helv": fontRef}},
		"SigFlags":        pdf.Integer(3),
		"XFA":             pdf.Array{pdf.String("datasets"), putStream("<xfa>SECRETSINGLE</xfa>", nil)},
		"NeedAppearances": pdf.Boolean(false),
	}); err != nil {
		t.Fatal(err)
	}

	if err := w.Put(structRoot, pdf.Dict{
		"Type": pdf.Name("StructTreeRoot"), "K": p.elem,
		"ParentTree": pdf.Dict{"Nums": pdf.Array{
			pdf.Integer(0), p.elem, pdf.Integer(1), p.elem,
		}},
	}); err != nil {
		t.Fatal(err)
	}

	annots0 := pdf.Array{p.f1, p.w2a, p.w3a, p.g1}
	if !noRedact {
		p.redactor = put(pdf.Dict{
			"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Redact"),
			"Rect": rect(40, 90, 120, 115), "P": p.page0,
		})
		annots0 = append(annots0, p.redactor)
	}
	content := putStream("BT /F1 12 Tf 50 100 Td (Secret) Tj ET\n", nil)
	res := pdf.Dict{"Font": pdf.Dict{"F1": fontRef}}
	for ref, d := range map[pdf.Reference]pdf.Dict{
		pagesRef: {
			"Type": pdf.Name("Pages"), "Count": pdf.Integer(2),
			"Kids": pdf.Array{p.page0, p.page1},
		},
		p.page0: {
			"Type": pdf.Name("Page"), "Parent": pagesRef,
			"MediaBox": rect(0, 0, 300, 300), "Resources": res,
			"Contents": content, "Annots": annots0,
		},
		p.page1: {
			"Type": pdf.Name("Page"), "Parent": pagesRef,
			"MediaBox": rect(0, 0, 300, 300), "Resources": res,
			"Annots": pdf.Array{p.w2b, p.w3b, p.g2, p.sig, p.f5},
		},
	} {
		if err := w.Put(ref, d); err != nil {
			t.Fatal(err)
		}
	}
	cat := w.GetMeta().Catalog
	cat.Pages = pagesRef
	cat.AcroForm = p.acroRef
	cat.StructTreeRoot = structRoot
	cat.Perms = pdf.Dict{"DocMDP": put(pdf.Dict{"Type": pdf.Name("Sig"), "Name": pdf.String("DOCMDPSECRET")})}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	r := openSource(t, buf.Data)
	return r, p
}

// formOut is the result of redacting the form test document.
type formOut struct {
	t    testing.TB
	r    *pdf.Reader
	data []byte
	acro pdf.Dict
}

func redactForm(t testing.TB, in *pdf.Reader) *formOut {
	t.Helper()
	var buf bytes.Buffer
	opts := &Options{Writer: &pdf.WriterOptions{HumanReadable: true}}
	if err := Write(&buf, in, opts); err != nil {
		t.Fatal(err)
	}
	r := openPDF(t, buf.Bytes())
	o := &formOut{t: t, r: r, data: buf.Bytes()}
	acro, err := pdf.Resolve(r, r.GetMeta().Catalog.AcroForm)
	if err != nil {
		t.Fatal(err)
	}
	o.acro, _ = acro.(pdf.Dict)
	return o
}

// dict resolves obj to a dictionary.
func (o *formOut) dict(obj pdf.Object) pdf.Dict {
	o.t.Helper()
	v, err := pdf.Resolve(o.r, obj)
	if err != nil {
		o.t.Fatal(err)
	}
	d, _ := v.(pdf.Dict)
	return d
}

// field returns the top-level field with the given name, or nil.
func (o *formOut) field(name string) pdf.Dict {
	o.t.Helper()
	fields, err := pdf.Resolve(o.r, o.acro["Fields"])
	if err != nil {
		o.t.Fatal(err)
	}
	arr, _ := fields.(pdf.Array)
	for _, f := range arr {
		d := o.dict(f)
		if str(d["T"]) == name {
			return d
		}
	}
	return nil
}

// kids returns the dictionaries of the kids of a field.
func (o *formOut) kids(f pdf.Dict) []pdf.Dict {
	o.t.Helper()
	v, err := pdf.Resolve(o.r, f["Kids"])
	if err != nil {
		o.t.Fatal(err)
	}
	var res []pdf.Dict
	arr, _ := v.(pdf.Array)
	for _, k := range arr {
		res = append(res, o.dict(k))
	}
	return res
}

func (o *formOut) lacks(s string) {
	o.t.Helper()
	if bytes.Contains(o.data, []byte(s)) {
		o.t.Errorf("output still contains %q", s)
	}
}

func TestFormSingleWidgetRemoved(t *testing.T) {
	in, p := formDoc(t, false)
	o := redactForm(t, in)

	if o.field("single") != nil {
		t.Error("field without widgets left is still listed")
	}
	o.lacks("SECRETSINGLE")
	o.lacks("DOCMDPSECRET")
	if o.field("other") == nil {
		t.Error("unrelated field was removed")
	}
	co, err := pdf.Resolve(o.r, o.acro["CO"])
	if err != nil {
		t.Fatal(err)
	}
	if arr, _ := co.(pdf.Array); len(arr) != 1 {
		t.Errorf("got %d entries in CO, want 1", len(arr))
	}
	if o.acro["XFA"] != nil {
		t.Error("kept /XFA although a field was removed")
	}
	if o.r.GetMeta().Catalog.Perms != nil {
		t.Error("kept /Perms")
	}

	// no content item refers to a deleted widget
	root := o.dict(o.r.GetMeta().Catalog.StructTreeRoot)
	elem := o.dict(root["K"])
	k, err := pdf.Resolve(o.r, elem["K"])
	if err != nil {
		t.Fatal(err)
	}
	items, _ := k.(pdf.Array)
	if len(items) != 1 {
		t.Fatalf("got %d content items, want 1", len(items))
	}
	if obj := o.dict(o.dict(items[0])["Obj"]); str(obj["T"]) != "other" {
		t.Errorf("remaining content item is wrong: %v", obj)
	}
	_ = p
}

func TestFormSiblingWidgetKept(t *testing.T) {
	in, _ := formDoc(t, false)
	o := redactForm(t, in)

	f := o.field("double")
	if f == nil {
		t.Fatal("field with a remaining widget was removed")
	}
	for _, key := range []pdf.Name{"V", "DV", "RV"} {
		if f[key] != nil {
			t.Errorf("field still has %s", key)
		}
	}
	kids := o.kids(f)
	if len(kids) != 1 {
		t.Fatalf("got %d widgets, want 1", len(kids))
	}
	if kids[0]["AP"] == nil {
		t.Error("remaining widget has no appearance")
	}
	o.lacks("DUPVALUE")
	o.lacks("DUPDEFAULT")
	if o.field("other") == nil {
		t.Fatal("unrelated field was removed")
	}
	if got := o.field("other")["V"]; str(got) != "KEEPME" {
		t.Errorf("unrelated value changed: %v", got)
	}
}

func TestFormCheckBox(t *testing.T) {
	in, _ := formDoc(t, false)
	o := redactForm(t, in)

	f := o.field("check")
	if f == nil {
		t.Fatal("check box was removed")
	}
	if f["V"] != nil {
		t.Error("check box still has a value")
	}
	kids := o.kids(f)
	if len(kids) != 1 {
		t.Fatalf("got %d widgets, want 1", len(kids))
	}
	if kids[0]["AS"] != pdf.Name("Off") {
		t.Errorf("got state %v, want Off", kids[0]["AS"])
	}
}

func TestFormInheritedValue(t *testing.T) {
	in, _ := formDoc(t, false)
	o := redactForm(t, in)

	g := o.field("grp")
	if g == nil {
		t.Fatal("group was removed")
	}
	if g["V"] != nil {
		t.Error("group still supplies the value")
	}
	if kids := o.kids(g); len(kids) != 1 {
		t.Errorf("got %d fields in the group, want 1", len(kids))
	}
	o.lacks("INHVALUE")
}

func TestFormSignature(t *testing.T) {
	in, _ := formDoc(t, false)
	o := redactForm(t, in)

	s := o.field("sign")
	if s == nil {
		t.Fatal("signature field was removed")
	}
	if s["V"] != nil {
		t.Error("signature field is still signed")
	}
	o.lacks("SIGSECRET")
	if o.acro["SigFlags"] != nil {
		t.Error("kept /SigFlags")
	}
}

func TestFormUntouched(t *testing.T) {
	in, _ := formDoc(t, true)
	o := redactForm(t, in)

	if o.acro["XFA"] == nil {
		t.Error("dropped /XFA although no field was touched")
	}
	for _, name := range []string{"single", "double", "check", "grp", "other"} {
		if o.field(name) == nil {
			t.Errorf("field %q is missing", name)
		}
	}
	if got := o.field("double")["V"]; str(got) != "DUPVALUE" {
		t.Errorf("value changed: %v", got)
	}
	if got := o.field("single")["V"]; str(got) != "SECRETSINGLE" {
		t.Errorf("value changed: %v", got)
	}
}

// str returns the contents of a PDF string.
func str(obj pdf.Object) string {
	s, _ := obj.(pdf.String)
	return string(s)
}

// A region on a page removes /XFA, which can hold copies of field values,
// even if no field is touched.
func TestFormXFAWithRegion(t *testing.T) {
	w, buf := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	pagesRef, pageRef, annotRef, xfaRef := w.Alloc(), w.Alloc(), w.Alloc(), w.Alloc()
	for ref, d := range map[pdf.Reference]pdf.Object{
		annotRef: pdf.Dict{
			"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Redact"),
			"Rect": rect(0, 0, 50, 50), "P": pageRef,
		},
		pagesRef: pdf.Dict{
			"Type": pdf.Name("Pages"), "Count": pdf.Integer(1), "Kids": pdf.Array{pageRef},
		},
		pageRef: pdf.Dict{
			"Type": pdf.Name("Page"), "Parent": pagesRef,
			"MediaBox": rect(0, 0, 300, 300), "Annots": pdf.Array{annotRef},
		},
		xfaRef: pdf.Dict{"Fields": pdf.Array{}, "XFA": pdf.String("SECRETXFA")},
	} {
		if err := w.Put(ref, d); err != nil {
			t.Fatal(err)
		}
	}
	cat := w.GetMeta().Catalog
	cat.Pages = pagesRef
	cat.AcroForm = xfaRef
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	in := openSource(t, buf.Data)
	o := redactForm(t, in)
	if o.acro["XFA"] != nil {
		t.Error("kept /XFA")
	}
	o.lacks("SECRETXFA")
}

// formPage builds the objects of a one-page document for the form tests.
// The page carries a redaction annotation over regionRect.
type formPage struct {
	t    testing.TB
	w    *pdf.Writer
	page pdf.Reference
	font pdf.Reference
}

// under and outside are rectangles inside and away from the region
var (
	under   = rect(50, 95, 70, 105)
	outside = rect(10, 10, 60, 30)
)

// formPageDoc builds a document with one page.  The function build writes
// the objects of the form, sets the catalog entries it needs, and returns
// the annotations of the page other than the redaction annotation.
func formPageDoc(t testing.TB, build func(b *formPage) pdf.Array) *pdf.Reader {
	t.Helper()
	w, buf := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	b := &formPage{t: t, w: w, page: w.Alloc()}
	b.font = b.put(pdf.Dict{
		"Type": pdf.Name("Font"), "Subtype": pdf.Name("Type1"),
		"BaseFont": pdf.Name("Helvetica"),
	})
	annots := build(b)
	annots = append(annots, b.put(pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Redact"),
		"Rect": rect(40, 90, 120, 115), "P": b.page,
	}))
	pagesRef := w.Alloc()
	b.set(pagesRef, pdf.Dict{
		"Type": pdf.Name("Pages"), "Count": pdf.Integer(1),
		"Kids": pdf.Array{b.page},
	})
	b.set(b.page, pdf.Dict{
		"Type": pdf.Name("Page"), "Parent": pagesRef,
		"MediaBox": rect(0, 0, 300, 300), "Annots": annots,
	})
	w.GetMeta().Catalog.Pages = pagesRef
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	r := openSource(t, buf.Data)
	return r
}

func (b *formPage) put(obj pdf.Object) pdf.Reference {
	ref := b.w.Alloc()
	b.set(ref, obj)
	return ref
}

func (b *formPage) set(ref pdf.Reference, obj pdf.Object) {
	if err := b.w.Put(ref, obj); err != nil {
		b.t.Fatal(err)
	}
}

// ap returns an appearance dictionary which shows text.
func (b *formPage) ap(text string) pdf.Dict {
	ref := b.w.Alloc()
	stm, err := b.w.OpenStream(ref, pdf.Dict{
		"Type": pdf.Name("XObject"), "Subtype": pdf.Name("Form"),
		"BBox": rect(0, 0, 100, 20),
	})
	if err != nil {
		b.t.Fatal(err)
	}
	if _, err := stm.Write([]byte("BT /Helv 12 Tf 2 2 Td (" + text + ") Tj ET")); err != nil {
		b.t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		b.t.Fatal(err)
	}
	return pdf.Dict{"N": ref}
}

// widget returns a widget annotation dictionary on the page, with the
// annotation name nm.
func (b *formPage) widget(r pdf.Array, nm string, extra pdf.Dict) pdf.Dict {
	d := pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget"),
		"Rect": r, "P": b.page, "F": pdf.Integer(4), "NM": pdf.String(nm),
	}
	maps.Copy(d, extra)
	return d
}

// form makes the dictionary of an interactive form with the given fields
// the form of the document.
func (b *formPage) form(fields ...pdf.Object) {
	b.w.GetMeta().Catalog.AcroForm = b.put(pdf.Dict{
		"Fields": pdf.Array(fields),
		"DA":     formDA,
		"DR":     pdf.Dict{"Font": pdf.Dict{"Helv": b.font}},
	})
}

var formDA = pdf.String("/Helv 12 Tf 0 g")

// annot returns the annotation of the first page with the annotation name
// nm, or nil.  It fails if there are several.
func (o *formOut) annot(nm string) pdf.Dict {
	o.t.Helper()
	pages := o.dict(o.r.GetMeta().Catalog.Pages)
	kids, err := pdf.Resolve(o.r, pages["Kids"])
	if err != nil {
		o.t.Fatal(err)
	}
	page := o.dict(kids.(pdf.Array)[0])
	annots, err := pdf.Resolve(o.r, page["Annots"])
	if err != nil {
		o.t.Fatal(err)
	}
	var res pdf.Dict
	arr, _ := annots.(pdf.Array)
	for _, obj := range arr {
		if d := o.dict(obj); str(d["NM"]) == nm {
			if res != nil {
				o.t.Fatalf("annotation %q is listed twice", nm)
			}
			res = d
		}
	}
	return res
}

// kid returns the kid of a field with the given partial name, or nil.
func (o *formOut) kid(f pdf.Dict, name string) pdf.Dict {
	o.t.Helper()
	for _, k := range o.kids(f) {
		if str(k["T"]) == name {
			return k
		}
	}
	return nil
}

// normal returns the decoded normal appearance of a widget.
func (o *formOut) normal(w pdf.Dict) []byte {
	o.t.Helper()
	ap := o.dict(w["AP"])
	v, err := pdf.Resolve(o.r, ap["N"])
	if err != nil {
		o.t.Fatal(err)
	}
	stm, ok := v.(*pdf.Stream)
	if !ok {
		o.t.Fatalf("normal appearance is %T, want a stream", v)
	}
	data, err := pdf.ReadAll(o.r, nil, stm, 1<<20)
	if err != nil {
		o.t.Fatal(err)
	}
	return data
}

// regenerated checks that the widget with the annotation name nm is kept
// with a new normal appearance, which does not show removed.
func (o *formOut) regenerated(nm, removed string) {
	o.t.Helper()
	w := o.annot(nm)
	if w == nil {
		o.t.Errorf("widget %q is missing", nm)
		return
	}
	if w["AP"] == nil {
		o.t.Errorf("widget %q has no appearance", nm)
		return
	}
	if bytes.Contains(o.normal(w), []byte(removed)) {
		o.t.Errorf("appearance of widget %q shows %q", nm, removed)
	}
}

func (o *formOut) has(s string) {
	o.t.Helper()
	if !bytes.Contains(o.data, []byte(s)) {
		o.t.Errorf("output lacks %q", s)
	}
}

// The widgets below a kid without /T are widgets of the field above it, and
// the remaining widget gets a new appearance.  A root of the field tree
// without /T is not a field: its remaining widget gets a new appearance
// without field contents.
func TestFormNamelessField(t *testing.T) {
	t.Run("kid", func(t *testing.T) {
		in := formPageDoc(t, func(b *formPage) pdf.Array {
			f, x, w1, w2 := b.w.Alloc(), b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
			b.set(f, pdf.Dict{
				"FT": pdf.Name("Tx"), "T": pdf.String("F"), "DA": formDA,
				"V": pdf.String("SECRETVAL"), "Kids": pdf.Array{x},
			})
			b.set(x, pdf.Dict{"Parent": f, "Kids": pdf.Array{w1, w2}})
			b.set(w1, b.widget(under, "w1", pdf.Dict{"Parent": x, "AP": b.ap("SECRETVAL")}))
			b.set(w2, b.widget(outside, "w2", pdf.Dict{"Parent": x, "AP": b.ap("SECRETVAL")}))
			b.form(f)
			return pdf.Array{w1, w2}
		})
		o := redactForm(t, in)

		o.lacks("SECRETVAL")
		f := o.field("F")
		if f == nil {
			t.Fatal("field was removed")
		}
		if f["V"] != nil {
			t.Error("field still has a value")
		}
		o.regenerated("w2", "SECRETVAL")
	})

	t.Run("root", func(t *testing.T) {
		in := formPageDoc(t, func(b *formPage) pdf.Array {
			f, w1, w2 := b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
			b.set(f, pdf.Dict{
				"FT": pdf.Name("Tx"), "DA": formDA,
				"V": pdf.String("ROOTVAL"), "Kids": pdf.Array{w1, w2},
			})
			b.set(w1, b.widget(under, "w1", pdf.Dict{"Parent": f, "AP": b.ap("ROOTVAL")}))
			b.set(w2, b.widget(outside, "w2", pdf.Dict{"Parent": f, "AP": b.ap("ROOTVAL")}))
			b.form(f)
			return pdf.Array{w1, w2}
		})
		o := redactForm(t, in)

		o.lacks("ROOTVAL")
		o.regenerated("w2", "ROOTVAL")
	})
}

// A field which /Fields does not list loses its value when a widget is
// deleted.
func TestFormFieldNotListed(t *testing.T) {
	for _, withForm := range []bool{true, false} {
		in := formPageDoc(t, func(b *formPage) pdf.Array {
			g, f, w1, w2 := b.w.Alloc(), b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
			b.set(g, pdf.Dict{"T": pdf.String("G"), "Kids": pdf.Array{f}})
			b.set(f, pdf.Dict{
				"FT": pdf.Name("Tx"), "T": pdf.String("F"), "DA": formDA,
				"V": pdf.String("UNLISTEDVAL"), "Kids": pdf.Array{w1, w2},
				"Parent": g,
			})
			b.set(w1, b.widget(under, "w1", pdf.Dict{"Parent": f, "AP": b.ap("UNLISTEDVAL")}))
			b.set(w2, b.widget(outside, "w2", pdf.Dict{"Parent": f, "AP": b.ap("UNLISTEDVAL")}))
			if withForm {
				b.form()
			}
			return pdf.Array{w1, w2}
		})
		o := redactForm(t, in)

		o.lacks("UNLISTEDVAL")
		o.regenerated("w2", "UNLISTEDVAL")
		if !withForm {
			if o.acro != nil {
				t.Error("form dictionary was added")
			}
			continue
		}
		fields, err := pdf.Resolve(o.r, o.acro["Fields"])
		if err != nil {
			t.Fatal(err)
		}
		if arr, _ := fields.(pdf.Array); len(arr) != 0 {
			t.Errorf("got %d entries in Fields, want 0", len(arr))
		}
	}
}

// A signature field loses its value also without a form dictionary.
func TestFormSignatureNoForm(t *testing.T) {
	in := formPageDoc(t, func(b *formPage) pdf.Array {
		sig := b.put(b.widget(outside, "sig", pdf.Dict{
			"FT": pdf.Name("Sig"), "T": pdf.String("sign"),
			"V": pdf.Dict{"Type": pdf.Name("Sig"), "Name": pdf.String("SIGNOFORM")},
		}))
		plain := b.put(b.widget(rect(10, 40, 60, 60), "plain", pdf.Dict{"AP": b.ap("PLAINAP")}))
		return pdf.Array{sig, plain}
	})
	o := redactForm(t, in)

	o.lacks("SIGNOFORM")
	if o.annot("sig") == nil {
		t.Error("signature field was removed")
	}
	// a widget which is no field is kept as it is
	if o.annot("plain") == nil {
		t.Error("plain widget was removed")
	}
	o.has("PLAINAP")
}

// A widget which names a field in /Parent, but which the /Kids of the field
// do not list, counts as a widget of the field.
func TestFormUnlistedWidget(t *testing.T) {
	t.Run("deleted", func(t *testing.T) {
		in := formPageDoc(t, func(b *formPage) pdf.Array {
			f, w1, w2 := b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
			b.set(f, pdf.Dict{
				"FT": pdf.Name("Tx"), "T": pdf.String("F"), "DA": formDA,
				"V": pdf.String("UNLISTED"), "Kids": pdf.Array{w1},
			})
			b.set(w1, b.widget(outside, "w1", pdf.Dict{"Parent": f, "AP": b.ap("UNLISTED")}))
			b.set(w2, b.widget(under, "w2", pdf.Dict{"Parent": f, "AP": b.ap("UNLISTED")}))
			b.form(f)
			return pdf.Array{w1, w2}
		})
		o := redactForm(t, in)

		o.lacks("UNLISTED")
		f := o.field("F")
		if f == nil {
			t.Fatal("field with a remaining widget was removed")
		}
		if len(o.kids(f)) != 1 {
			t.Errorf("got %d widgets, want 1", len(o.kids(f)))
		}
	})

	t.Run("kept", func(t *testing.T) {
		in := formPageDoc(t, func(b *formPage) pdf.Array {
			f, w1, w2 := b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
			b.set(f, pdf.Dict{
				"FT": pdf.Name("Tx"), "T": pdf.String("F"), "DA": formDA,
				"V": pdf.String("UNLISTED"), "Kids": pdf.Array{w1},
			})
			b.set(w1, b.widget(under, "w1", pdf.Dict{"Parent": f, "AP": b.ap("UNLISTED")}))
			b.set(w2, b.widget(outside, "w2", pdf.Dict{"Parent": f, "AP": b.ap("UNLISTED")}))
			b.form(f)
			return pdf.Array{w1, w2}
		})
		o := redactForm(t, in)

		o.lacks("UNLISTED")
		o.regenerated("w2", "UNLISTED")
	})
}

// A widget whose field cannot be read gets a new appearance without field
// contents.  The new appearance has no states, so the widget loses its
// appearance state.
func TestFormUnreadableWidget(t *testing.T) {
	in := formPageDoc(t, func(b *formPage) pdf.Array {
		f, w1, w2 := b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
		b.set(f, pdf.Dict{
			"FT": pdf.Name("Bogus"), "T": pdf.String("odd"),
			"V": pdf.Name("Yes"), "Kids": pdf.Array{w1, w2},
		})
		states := func() pdf.Dict {
			return pdf.Dict{"N": pdf.Dict{"Yes": b.ap("ODDON")["N"], "Off": b.ap("")["N"]}}
		}
		b.set(w1, b.widget(under, "w1", pdf.Dict{"Parent": f, "AS": pdf.Name("Yes"), "AP": states()}))
		b.set(w2, b.widget(outside, "w2", pdf.Dict{"Parent": f, "AS": pdf.Name("Yes"), "AP": states()}))
		b.form(f)
		return pdf.Array{w1, w2}
	})
	o := redactForm(t, in)

	o.regenerated("w2", "ODDON")
	w2 := o.annot("w2")
	if w2 == nil {
		t.Fatal("remaining widget is missing")
	}
	if w2["AS"] != nil {
		t.Errorf("got state %v, want none", w2["AS"])
	}
	o.lacks("ODDON")
}

// A remaining widget without a usable /Rect has no area for a new
// appearance, and loses its appearance.  The widget of a button is turned
// off.
func TestFormWidgetWithoutRect(t *testing.T) {
	t.Run("reproducer", func(t *testing.T) {
		data := []byte("%PDF-1.0\n8 0 obj<</FT/Btn/Kids[10 0R]>>\n" +
			"9 0 obj<</Subtype/Widget/Parent 8 0R/Rect[0 1 01 0]>>\n" +
			"10 0 obj<</Subtype/Widget>>\n" +
			"31 0 obj<</Subtype/Redact/Rect[0 0 10 1]0>>\n" +
			"1 0 obj<</Type/Page/Annots[9 0R 31 0R]>>\n" +
			"2 0 obj<</Type/Catalog/Pages 1 0R>>00")
		in := openPDF(t, data)
		o := redactForm(t, in)
		w, err := pdf.Resolve(o.r, pdf.NewReference(10, 0))
		if err != nil {
			t.Fatal(err)
		}
		if d, _ := w.(pdf.Dict); d["AP"] != nil {
			t.Error("widget has an appearance")
		}
	})

	// a rectangle too small to measure
	t.Run("tiny", func(t *testing.T) {
		in := formPageDoc(t, func(b *formPage) pdf.Array {
			f, w1, w2 := b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
			b.set(f, pdf.Dict{
				"FT": pdf.Name("Tx"), "T": pdf.String("F"), "DA": formDA,
				"V": pdf.String("NORECT"), "Kids": pdf.Array{w1, w2},
			})
			b.set(w1, b.widget(under, "w1", pdf.Dict{"Parent": f, "AP": b.ap("NORECT")}))
			b.set(w2, b.widget(pdf.Array{pdf.Integer(20), pdf.Integer(20), pdf.Real(20.001), pdf.Real(20.001)},
				"w2", pdf.Dict{"Parent": f, "AP": b.ap("NORECT")}))
			b.form(f)
			return pdf.Array{w1, w2}
		})
		o := redactForm(t, in)
		o.lacks("NORECT")
	})

	for _, tc := range []struct {
		name string
		rect pdf.Object // nil for no /Rect
		ft   pdf.Name
		as   pdf.Object
	}{
		{"text", nil, "Tx", nil},
		{"text degenerate", rect(20, 20, 20, 20), "Tx", nil},
		{"button", nil, "Btn", pdf.Name("Off")},
		{"button degenerate", rect(20, 20, 20, 20), "Btn", pdf.Name("Off")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := formPageDoc(t, func(b *formPage) pdf.Array {
				f, w1, w2 := b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
				var v pdf.Object = pdf.String("NORECT")
				if tc.ft == "Btn" {
					v = pdf.Name("NORECT")
				}
				b.set(f, pdf.Dict{
					"FT": tc.ft, "T": pdf.String("F"), "DA": formDA,
					"V": v, "Kids": pdf.Array{w1, w2},
				})
				b.set(w1, b.widget(under, "w1", pdf.Dict{"Parent": f, "AP": b.ap("NORECT")}))
				d := b.widget(outside, "w2", pdf.Dict{"Parent": f, "AP": b.ap("NORECT")})
				if tc.rect == nil {
					delete(d, "Rect")
				} else {
					d["Rect"] = tc.rect
				}
				b.set(w2, d)
				b.form(f)
				return pdf.Array{w1, w2}
			})
			o := redactForm(t, in)

			o.lacks("NORECT")
			w2 := o.annot("w2")
			if w2 == nil {
				t.Fatal("remaining widget is missing")
			}
			if w2["AP"] != nil {
				t.Error("remaining widget has an appearance")
			}
			if w2["AS"] != tc.as {
				t.Errorf("got state %v, want %v", w2["AS"], tc.as)
			}
		})
	}
}

// PDF 2.0 requires an appearance for a widget whose rectangle is not a
// single point.  A remaining widget without an area to draw on then gets a
// new appearance which does not show the removed value.
func TestFormWidgetWithoutAreaV2(t *testing.T) {
	in := formPageDoc(t, func(b *formPage) pdf.Array {
		b.w.GetMeta().Catalog.Version = pdf.V2_0
		f, w1, w2 := b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
		b.set(f, pdf.Dict{
			"FT": pdf.Name("Tx"), "T": pdf.String("F"), "DA": formDA,
			"V": pdf.String("NOAREA"), "Kids": pdf.Array{w1, w2},
		})
		b.set(w1, b.widget(under, "w1", pdf.Dict{"Parent": f, "AP": b.ap("NOAREA")}))
		b.set(w2, b.widget(rect(20, 20, 20, 40), "w2", pdf.Dict{"Parent": f, "AP": b.ap("NOAREA")}))
		b.form(f)
		return pdf.Array{w1, w2}
	})
	o := redactForm(t, in)

	o.lacks("NOAREA")
	o.regenerated("w2", "NOAREA")
}

// Changes to the copy of a decoded field leave the decoded field and its
// widgets unchanged.
func TestFieldCopy(t *testing.T) {
	var f, w1, w2 pdf.Reference
	in := formPageDoc(t, func(b *formPage) pdf.Array {
		f, w1, w2 = b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
		b.set(f, pdf.Dict{
			"FT": pdf.Name("Tx"), "T": pdf.String("F"), "DA": formDA,
			"V": pdf.String("VALUE"), "Kids": pdf.Array{w1},
		})
		b.set(w1, b.widget(outside, "w1", pdf.Dict{"Parent": f, "AP": b.ap("VALUE")}))
		b.set(w2, b.widget(outside, "w2", pdf.Dict{"Parent": f}))
		b.form(f)
		return pdf.Array{w1, w2}
	})
	x := pdf.NewExtractor(in)
	cur := pdf.CursorAt(x, nil)
	orig, _, err := decode.FieldAt(x, f)
	if err != nil {
		t.Fatal(err)
	}
	tf, ok := orig.(*acroform.TextField)
	if !ok || tf.Text == nil || len(tf.Widgets) != 1 {
		t.Fatalf("unexpected field %#v", orig)
	}
	text := tf.Text
	w0 := tf.Widgets[0].(*annotation.Widget)
	ap := w0.Appearance

	fc := copyField(orig)
	clearField(fc.f, [len(valueKeys)]bool{true, true, true, true})
	if _, err := fc.orphanWidget(cur, w2); err != nil {
		t.Fatal(err)
	}
	copied := fc.f.GetCommon().Widgets
	if len(copied) != 2 {
		t.Fatalf("got %d widgets of the copy, want 2", len(copied))
	}
	for _, w := range copied {
		aw := w.(*annotation.Widget)
		if aw.Field != fc.f {
			t.Error("widget of the copy belongs to another field")
		}
		aw.Appearance = nil
		aw.AppearanceState = "Off"
	}

	again, _, err := decode.FieldAt(x, f)
	if err != nil {
		t.Fatal(err)
	}
	if again != orig {
		t.Fatal("fieldAt returned a different field")
	}
	if tf.Text != text || len(tf.Widgets) != 1 || tf.Widgets[0] != acroform.Widget(w0) {
		t.Error("decoded field was changed")
	}
	if w0.Field != orig || w0.Appearance != ap || w0.AppearanceState != "" {
		t.Error("decoded widget was changed")
	}
	a, err := cur.Decode(w2, decode.Annotation)
	if err != nil {
		t.Fatal(err)
	}
	if aw, ok := a.(*annotation.Widget); !ok || aw.Field != nil || aw.AppearanceState != "" {
		t.Error("decoded unlisted widget was changed")
	}
}

// Removing an entry from an ancestor affects exactly the fields which
// inherit this entry.
func TestFormInheritedEntries(t *testing.T) {
	in := formPageDoc(t, func(b *formPage) pdf.Array {
		g, a, bb, c := b.w.Alloc(), b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
		wa, wb, wc := b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
		b.set(g, pdf.Dict{
			"FT": pdf.Name("Tx"), "T": pdf.String("g"), "DA": formDA,
			"V": pdf.String("GRPVALUE"), "DV": pdf.String("GRPDEFAULT"),
			"RV":   pdf.String("GRPRICH"),
			"Kids": pdf.Array{a, bb, c},
		})
		b.set(a, pdf.Dict{"T": pdf.String("a"), "Parent": g, "Kids": pdf.Array{wa}})
		b.set(bb, pdf.Dict{
			"T": pdf.String("b"), "Parent": g, "DV": pdf.String("BOWNDEFAULT"),
			"Kids": pdf.Array{wb},
		})
		b.set(c, pdf.Dict{
			"T": pdf.String("c"), "Parent": g, "V": pdf.String("COWNVALUE"),
			"Kids": pdf.Array{wc},
		})
		b.set(wa, b.widget(under, "wa", pdf.Dict{"Parent": a, "AP": b.ap("GRPVALUE")}))
		b.set(wb, b.widget(outside, "wb", pdf.Dict{"Parent": bb, "AP": b.ap("GRPVALUE")}))
		b.set(wc, b.widget(rect(10, 40, 60, 60), "wc", pdf.Dict{"Parent": c, "AP": b.ap("CAPPEARANCE")}))
		b.form(g)
		return pdf.Array{wa, wb, wc}
	})
	o := redactForm(t, in)

	o.lacks("GRPVALUE")
	o.lacks("GRPDEFAULT")
	o.lacks("GRPRICH")
	g := o.field("g")
	if g == nil {
		t.Fatal("group was removed")
	}
	if o.kid(g, "a") != nil {
		t.Error("field without widgets left is still listed")
	}

	fb := o.kid(g, "b")
	if fb == nil {
		t.Fatal("field b was removed")
	}
	if got := str(fb["DV"]); got != "BOWNDEFAULT" {
		t.Errorf("own default value of b changed: %q", got)
	}
	if o.annot("wb")["AP"] == nil {
		t.Error("widget of b has no appearance")
	}

	fc := o.kid(g, "c")
	if fc == nil {
		t.Fatal("field c was removed")
	}
	if got := str(fc["V"]); got != "COWNVALUE" {
		t.Errorf("own value of c changed: %q", got)
	}
	// c keeps its value, so its appearance is kept
	o.has("CAPPEARANCE")
}

// A choice field whose inherited /I is removed gets a new appearance, and
// keeps its own entries.
func TestFormInheritedSelection(t *testing.T) {
	in := formPageDoc(t, func(b *formPage) pdf.Array {
		g, d, e, wd, we := b.w.Alloc(), b.w.Alloc(), b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
		b.set(g, pdf.Dict{
			"FT": pdf.Name("Ch"), "T": pdf.String("ch"), "DA": formDA,
			"Opt": pdf.Array{pdf.String("one"), pdf.String("two"), pdf.String("three")},
			"I":   pdf.Array{pdf.Integer(1)}, "Kids": pdf.Array{d, e},
		})
		b.set(d, pdf.Dict{"T": pdf.String("d"), "Parent": g, "Kids": pdf.Array{wd}})
		b.set(e, pdf.Dict{
			"T": pdf.String("e"), "Parent": g, "DV": pdf.String("three"),
			"Kids": pdf.Array{we},
		})
		b.set(wd, b.widget(under, "wd", pdf.Dict{"Parent": d, "AP": b.ap("DAPPEARANCE")}))
		b.set(we, b.widget(outside, "we", pdf.Dict{"Parent": e, "AP": b.ap("EAPPEARANCE")}))
		b.form(g)
		return pdf.Array{wd, we}
	})
	o := redactForm(t, in)

	g := o.field("ch")
	if g == nil {
		t.Fatal("group was removed")
	}
	if g["I"] != nil {
		t.Error("group still supplies the selection")
	}
	fe := o.kid(g, "e")
	if fe == nil {
		t.Fatal("field e was removed")
	}
	if got := str(fe["DV"]); got != "three" {
		t.Errorf("own default value of e changed: %q", got)
	}
	o.lacks("DAPPEARANCE")
	o.lacks("EAPPEARANCE")
	if o.annot("we")["AP"] == nil {
		t.Error("widget of e has no appearance")
	}
}

// A choice field loses its value and selection, and its remaining widget no
// longer shows the value.
func TestFormChoiceValue(t *testing.T) {
	in := formPageDoc(t, func(b *formPage) pdf.Array {
		f, w1, w2 := b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
		b.set(f, pdf.Dict{
			"FT": pdf.Name("Ch"), "Ff": pdf.Integer(1 << 17), "T": pdf.String("fruit"),
			"DA":  formDA,
			"Opt": pdf.Array{pdf.String("Apple"), pdf.String("Banana")},
			"V":   pdf.String("Banana"), "I": pdf.Array{pdf.Integer(1)},
			"Kids": pdf.Array{w1, w2},
		})
		b.set(w1, b.widget(under, "w1", pdf.Dict{"Parent": f, "AP": b.ap("Banana")}))
		b.set(w2, b.widget(outside, "w2", pdf.Dict{"Parent": f, "AP": b.ap("Banana")}))
		b.form(f)
		return pdf.Array{w1, w2}
	})
	o := redactForm(t, in)

	f := o.field("fruit")
	if f == nil {
		t.Fatal("field was removed")
	}
	for _, key := range []pdf.Name{"V", "I"} {
		if f[key] != nil {
			t.Errorf("field still has %s", key)
		}
	}
	w2 := o.annot("w2")
	if w2 == nil {
		t.Fatal("remaining widget is missing")
	}
	if w2["AP"] == nil {
		t.Fatal("remaining widget has no appearance")
	}
	if bytes.Contains(o.normal(w2), []byte("Banana")) {
		t.Error("remaining widget still shows the value")
	}
}

// Field dictionaries in /Fields and in /Kids may be direct objects.
func TestFormDirectFields(t *testing.T) {
	in := formPageDoc(t, func(b *formPage) pdf.Array {
		w1, w2, w3, w4 := b.w.Alloc(), b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
		direct := pdf.Dict{
			"FT": pdf.Name("Tx"), "T": pdf.String("direct"), "DA": formDA,
			"V": pdf.String("DIRVAL"),
			"Kids": pdf.Array{w1, w2, b.widget(rect(10, 40, 60, 60), "w5", pdf.Dict{
				"AP": b.ap("DIRVAL"), "AS": pdf.Name("On"),
			})},
		}
		grp := b.put(pdf.Dict{
			"T": pdf.String("grp"),
			"Kids": pdf.Array{pdf.Dict{
				"FT": pdf.Name("Tx"), "T": pdf.String("k"), "DA": formDA,
				"V": pdf.String("DIRKIDVAL"), "Kids": pdf.Array{w3, w4},
			}},
		})
		b.set(w1, b.widget(under, "w1", pdf.Dict{"AP": b.ap("DIRVAL")}))
		b.set(w2, b.widget(outside, "w2", pdf.Dict{"AP": b.ap("DIRVAL")}))
		b.set(w3, b.widget(rect(70, 95, 90, 105), "w3", pdf.Dict{"AP": b.ap("DIRKIDVAL")}))
		b.set(w4, b.widget(rect(10, 40, 60, 60), "w4", pdf.Dict{"AP": b.ap("DIRKIDVAL")}))
		b.form(direct, grp)
		return pdf.Array{w1, w2, w3, w4}
	})
	o := redactForm(t, in)

	o.lacks("DIRVAL")
	o.lacks("DIRKIDVAL")
	f := o.field("direct")
	if f == nil {
		t.Fatal("direct field was removed")
	}
	if f["V"] != nil {
		t.Error("direct field still has a value")
	}
	if n := len(o.kids(f)); n != 2 {
		t.Errorf("got %d widgets of the direct field, want 2", n)
	}
	for _, w := range o.kids(f) {
		if str(w["NM"]) == "w5" && w["AS"] != nil {
			t.Errorf("got state %v of the direct widget, want none", w["AS"])
		}
	}
	g := o.field("grp")
	if g == nil {
		t.Fatal("group was removed")
	}
	k := o.kid(g, "k")
	if k == nil {
		t.Fatal("direct kid was removed")
	}
	if k["V"] != nil {
		t.Error("direct kid still has a value")
	}
	if n := len(o.kids(k)); n != 1 {
		t.Errorf("got %d widgets of the direct kid, want 1", n)
	}
}

// A kept widget whose pop-up is deleted is written once, without the pop-up
// and with its new appearance.
func TestFormWidgetPopup(t *testing.T) {
	in := formPageDoc(t, func(b *formPage) pdf.Array {
		f, w1, w2, pp := b.w.Alloc(), b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
		b.set(f, pdf.Dict{
			"FT": pdf.Name("Tx"), "T": pdf.String("F"), "DA": formDA,
			"V": pdf.String("POPVAL"), "Kids": pdf.Array{w1, w2},
		})
		b.set(w1, b.widget(under, "w1", pdf.Dict{"Parent": f, "AP": b.ap("POPVAL")}))
		b.set(w2, b.widget(outside, "w2", pdf.Dict{
			"Parent": f, "AP": b.ap("POPVAL"), "Popup": pp,
		}))
		b.set(pp, pdf.Dict{
			"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Popup"),
			"Rect": under, "Parent": w2,
		})
		b.form(f)
		return pdf.Array{w1, w2, pp}
	})
	o := redactForm(t, in)

	o.lacks("POPVAL")
	w2 := o.annot("w2")
	if w2 == nil {
		t.Fatal("remaining widget is missing")
	}
	if w2["Popup"] != nil {
		t.Error("widget still refers to the deleted pop-up")
	}
	if w2["AP"] == nil {
		t.Error("remaining widget has no appearance")
	}
}

// A field merged with its widget, whose pop-up is deleted, is written once
// without the pop-up.  It loses its value with a field of the same name.
func TestFormMergedFieldPopup(t *testing.T) {
	in := formPageDoc(t, func(b *formPage) pdf.Array {
		m1, m2, pp := b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
		b.set(m1, b.widget(under, "m1", pdf.Dict{
			"FT": pdf.Name("Tx"), "T": pdf.String("same"), "DA": formDA,
			"V": pdf.String("SAMEVAL"), "AP": b.ap("SAMEVAL"),
		}))
		b.set(m2, b.widget(outside, "m2", pdf.Dict{
			"FT": pdf.Name("Tx"), "T": pdf.String("same"), "DA": formDA,
			"V": pdf.String("SAMEVAL"), "AP": b.ap("SAMEVAL"), "Popup": pp,
		}))
		b.set(pp, pdf.Dict{
			"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Popup"),
			"Rect": under, "Parent": m2,
		})
		b.form(m1, m2)
		return pdf.Array{m1, m2, pp}
	})
	o := redactForm(t, in)

	o.lacks("SAMEVAL")
	m2 := o.annot("m2")
	if m2 == nil {
		t.Fatal("remaining field is missing")
	}
	if m2["Popup"] != nil {
		t.Error("field still refers to the deleted pop-up")
	}
	if m2["V"] != nil {
		t.Error("field still has a value")
	}
	if m2["AP"] == nil {
		t.Error("field has no appearance")
	}
}

// A /Parent chain with a cycle ends where it repeats.
func TestFormParentCycle(t *testing.T) {
	in := formPageDoc(t, func(b *formPage) pdf.Array {
		w, p1, p2 := b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
		b.set(w, b.widget(under, "w", pdf.Dict{"Parent": p1}))
		b.set(p1, pdf.Dict{"T": pdf.String("p1"), "V": pdf.String("CYCLEVAL"), "Parent": p2})
		b.set(p2, pdf.Dict{"T": pdf.String("p2"), "Parent": p1})
		return pdf.Array{w}
	})
	o := redactForm(t, in)
	o.lacks("CYCLEVAL")
}

// A long /Parent chain is followed to the field which supplies the value.
func TestFormLongParentChain(t *testing.T) {
	in := formPageDoc(t, func(b *formPage) pdf.Array {
		refs := make([]pdf.Reference, 5000)
		for i := range refs {
			refs[i] = b.w.Alloc()
		}
		b.set(refs[0], b.widget(under, "w", pdf.Dict{"Parent": refs[1]}))
		for i := 1; i < len(refs); i++ {
			d := pdf.Dict{}
			if i+1 < len(refs) {
				d["Parent"] = refs[i+1]
			} else {
				d["T"] = pdf.String("p")
				d["FT"] = pdf.Name("Tx")
				d["V"] = pdf.String("CHAINVAL")
			}
			b.set(refs[i], d)
		}
		return pdf.Array{refs[0]}
	})
	o := redactForm(t, in)
	o.lacks("CHAINVAL")
}

// deepFieldDoc builds a document whose form is a chain of fields, deeper
// than the limit for nested indirect objects, with a text field at the end
// whose single widget lies under the region if redact is set.
func deepFieldDoc(t *testing.T, depth int, redact bool) *pdf.Reader {
	return formPageDoc(t, func(b *formPage) pdf.Array {
		refs := make([]pdf.Reference, depth)
		for i := range refs {
			refs[i] = b.w.Alloc()
		}
		r := outside
		if redact {
			r = under
		}
		for i, ref := range refs {
			d := pdf.Dict{"T": pdf.String("a")}
			if i > 0 {
				d["Parent"] = refs[i-1]
			}
			if i+1 < depth {
				d["Kids"] = pdf.Array{refs[i+1]}
			} else {
				d = b.widget(r, "leaf", d)
				d["FT"] = pdf.Name("Tx")
				d["DA"] = formDA
				d["V"] = pdf.String("DEEPVAL")
			}
			b.set(ref, d)
		}
		b.form(refs[0])
		return pdf.Array{refs[depth-1]}
	})
}

// A deep field tree is processed: only the work budget limits it.
func TestFormDeepFieldTree(t *testing.T) {
	const depth = 300
	o := redactForm(t, deepFieldDoc(t, depth, false))
	o.has("DEEPVAL")
	if o.annot("leaf") == nil {
		t.Error("widget of the deep field is missing")
	}

	o = redactForm(t, deepFieldDoc(t, depth, true))
	o.lacks("DEEPVAL")
	if o.field("a") != nil {
		t.Error("field without widgets is still listed")
	}
}

// A field which names a group in /Parent, but which the /Kids of the group
// do not list, loses its value when its widget is deleted.
func TestFormUnlistedField(t *testing.T) {
	in := formPageDoc(t, func(b *formPage) pdf.Array {
		g, a, bb, wa, wb := b.w.Alloc(), b.w.Alloc(), b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
		b.set(g, pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("g"), "DA": formDA, "Kids": pdf.Array{a}})
		b.set(a, pdf.Dict{
			"T": pdf.String("a"), "Parent": g, "V": pdf.String("KEEPA"),
			"Kids": pdf.Array{wa},
		})
		b.set(bb, pdf.Dict{
			"T": pdf.String("b"), "Parent": g, "V": pdf.String("UNLISTEDKID"),
			"Kids": pdf.Array{wb},
		})
		b.set(wa, b.widget(outside, "wa", pdf.Dict{"Parent": a, "AP": b.ap("KEEPA")}))
		b.set(wb, b.widget(under, "wb", pdf.Dict{"Parent": bb, "AP": b.ap("UNLISTEDKID")}))
		b.form(g)
		return pdf.Array{wa, wb}
	})
	o := redactForm(t, in)

	o.lacks("UNLISTEDKID")
	g := o.field("g")
	if g == nil {
		t.Fatal("group was removed")
	}
	a := o.kid(g, "a")
	if a == nil {
		t.Fatal("listed field was removed")
	}
	if got := str(a["V"]); got != "KEEPA" {
		t.Errorf("value of the listed field changed: %q", got)
	}
}

// A group whose fields are all removed is removed.
func TestFormEmptyGroupRemoved(t *testing.T) {
	in := formPageDoc(t, func(b *formPage) pdf.Array {
		g, a, wa := b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
		b.set(g, pdf.Dict{"T": pdf.String("g"), "Kids": pdf.Array{a}})
		b.set(a, pdf.Dict{
			"FT": pdf.Name("Tx"), "T": pdf.String("a"), "Parent": g, "DA": formDA,
			"V": pdf.String("ONLYVAL"), "Kids": pdf.Array{wa},
		})
		b.set(wa, b.widget(under, "wa", pdf.Dict{"Parent": a, "AP": b.ap("ONLYVAL")}))
		b.form(g)
		return pdf.Array{wa}
	})
	o := redactForm(t, in)

	o.lacks("ONLYVAL")
	if o.field("g") != nil {
		t.Error("group without fields is still listed")
	}
}

// Fields without a name are not merged by name.
func TestFormNamelessFieldsIndependent(t *testing.T) {
	in := formPageDoc(t, func(b *formPage) pdf.Array {
		f1, f2, w1, w2 := b.w.Alloc(), b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
		b.set(f1, pdf.Dict{
			"FT": pdf.Name("Tx"), "DA": formDA,
			"V": pdf.String("SECRETVAL"), "Kids": pdf.Array{w1},
		})
		b.set(f2, pdf.Dict{
			"FT": pdf.Name("Tx"), "DA": formDA,
			"V": pdf.String("OTHERVAL"), "Kids": pdf.Array{w2},
		})
		b.set(w1, b.widget(under, "w1", pdf.Dict{"Parent": f1, "AP": b.ap("SECRETVAL")}))
		b.set(w2, b.widget(outside, "w2", pdf.Dict{"Parent": f2, "AP": b.ap("OTHERVAL")}))
		b.form(f1, f2)
		return pdf.Array{w1, w2}
	})
	o := redactForm(t, in)

	o.lacks("SECRETVAL")
	o.has("OTHERVAL")
}

// A field which names a removed group in /Parent, but which the /Kids of the
// group do not list, keeps the group alive, so that /Parent does not dangle.
func TestFormOrphanKeepsGroup(t *testing.T) {
	in := formPageDoc(t, func(b *formPage) pdf.Array {
		g, a, o, wa, wo := b.w.Alloc(), b.w.Alloc(), b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
		b.set(g, pdf.Dict{"T": pdf.String("g"), "Kids": pdf.Array{a}})
		b.set(a, pdf.Dict{
			"FT": pdf.Name("Tx"), "T": pdf.String("a"), "Parent": g, "DA": formDA,
			"V": pdf.String("SECRETVAL"), "Kids": pdf.Array{wa},
		})
		b.set(o, pdf.Dict{
			"FT": pdf.Name("Tx"), "T": pdf.String("o"), "Parent": g, "DA": formDA,
			"V": pdf.String("OTHERVAL"), "Kids": pdf.Array{wo},
		})
		b.set(wa, b.widget(under, "wa", pdf.Dict{"Parent": a, "AP": b.ap("SECRETVAL")}))
		b.set(wo, b.widget(outside, "wo", pdf.Dict{"Parent": o, "AP": b.ap("OTHERVAL")}))
		b.form(g)
		return pdf.Array{wa, wo}
	})
	out := redactForm(t, in)

	out.lacks("SECRETVAL")
	wo := out.annot("wo")
	if wo == nil {
		t.Fatal("widget was removed")
	}
	f := out.dict(wo["Parent"])
	if f == nil {
		t.Fatal("widget has no parent")
	}
	if g := out.dict(f["Parent"]); g == nil || str(g["T"]) != "g" {
		t.Error("parent of the field dangles")
	}
}

// Kids without a partial name which give a value are representations of the
// same field: they lose their values together with the field.
func TestFormNamelessKidValue(t *testing.T) {
	for _, parentValue := range []bool{false, true} {
		in := formPageDoc(t, func(b *formPage) pdf.Array {
			f, w1, w2 := b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
			fd := pdf.Dict{
				"FT": pdf.Name("Tx"), "T": pdf.String("ssn"), "DA": formDA,
				"Kids": pdf.Array{w1, w2},
			}
			if parentValue {
				fd["V"] = pdf.String("SECRETPARENT")
			}
			b.set(f, fd)
			for ref, r := range map[pdf.Reference]pdf.Array{w1: under, w2: outside} {
				nm := "w1"
				if ref == w2 {
					nm = "w2"
				}
				b.set(ref, b.widget(r, nm, pdf.Dict{
					"Parent": f, "AP": b.ap("SECRETSSN"),
					"V": pdf.String("SECRETSSN"), "DV": pdf.String("SECRETDV"),
				}))
			}
			b.form(f)
			return pdf.Array{w1, w2}
		})
		o := redactForm(t, in)

		for _, s := range []string{"SECRETSSN", "SECRETDV", "SECRETPARENT"} {
			o.lacks(s)
		}
		f := o.field("ssn")
		if f == nil {
			t.Fatal("field was removed")
		}
		if kids := o.kids(f); len(kids) != 1 {
			t.Errorf("got %d kids, want 1", len(kids))
		}
		o.regenerated("w2", "SECRETSSN")
	}
}

// radioDoc builds a document with a group of radio buttons whose first
// button lies under the region.  The /Opt entry is given to the field, or
// to a group above it if inherit is set; with sibling, the group holds a
// second radio button field without buttons under the region.
func radioDoc(t *testing.T, opt pdf.Array, inherit, sibling bool) *pdf.Reader {
	return formPageDoc(t, func(b *formPage) pdf.Array {
		g, f, w1, w2 := b.w.Alloc(), b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
		fd := pdf.Dict{
			"FT": pdf.Name("Btn"), "Ff": pdf.Integer(acroform.FieldRadio | acroform.FieldNoToggleToOff),
			"T": pdf.String("r"), "DA": formDA, "V": pdf.Name("0"), "DV": pdf.Name("0"),
			"Kids": pdf.Array{w1, w2},
		}
		gd := pdf.Dict{"T": pdf.String("g"), "Kids": pdf.Array{f}}
		if inherit {
			gd["Opt"] = opt
			fd["Parent"] = g
		} else {
			fd["Opt"] = opt
		}
		states := func(on pdf.Name) pdf.Dict {
			return pdf.Dict{"N": pdf.Dict{on: b.ap("on")["N"], "Off": b.ap("off")["N"]}}
		}
		b.set(w1, b.widget(under, "w1", pdf.Dict{"Parent": f, "AS": pdf.Name("0"), "AP": states("0")}))
		b.set(w2, b.widget(outside, "w2", pdf.Dict{"Parent": f, "AS": pdf.Name("Off"), "AP": states("1")}))
		annots := pdf.Array{w1, w2}
		if sibling {
			s, w3, w4 := b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
			b.set(s, pdf.Dict{
				"FT": pdf.Name("Btn"), "Ff": pdf.Integer(acroform.FieldRadio),
				"T": pdf.String("s"), "Parent": g, "Kids": pdf.Array{w3, w4},
			})
			b.set(w3, b.widget(rect(10, 200, 30, 220), "w3", pdf.Dict{"Parent": s, "AS": pdf.Name("Off"), "AP": states("0")}))
			b.set(w4, b.widget(rect(40, 200, 60, 220), "w4", pdf.Dict{"Parent": s, "AS": pdf.Name("Off"), "AP": states("1")}))
			gd["Kids"] = pdf.Array{f, s}
			annots = append(annots, w3, w4)
		}
		b.set(f, fd)
		if inherit {
			b.set(g, gd)
			b.form(g)
		} else {
			b.form(f)
		}
		return annots
	})
}

// checkRadio checks the radio button field fd of the output, which has
// one button left: /Opt, if present, has one entry for it, the on-state of
// the button is a valid position in /Opt, and the field has no value.
func checkRadio(t *testing.T, o *formOut, fd pdf.Dict, opt pdf.Object) {
	t.Helper()
	if fd["V"] != nil || fd["DV"] != nil {
		t.Error("field kept its value")
	}
	kids := o.kids(fd)
	if len(kids) != 1 {
		t.Fatalf("got %d buttons, want 1", len(kids))
	}
	n := 0
	if opt != nil {
		v, err := pdf.Resolve(o.r, opt)
		if err != nil {
			t.Fatal(err)
		}
		n = len(v.(pdf.Array))
		if n != 1 {
			t.Errorf("got %d export values, want 1", n)
		}
	}
	states := o.dict(o.dict(kids[0]["AP"])["N"])
	for name := range states {
		if name == "Off" {
			continue
		}
		if i, err := strconv.Atoi(string(name)); err == nil && i >= n && n > 0 {
			t.Errorf("on-state %q does not index /Opt", name)
		}
	}
}

// A radio button field which loses a button loses its export value, and the
// remaining buttons are numbered by their new positions.
func TestFormRadioOpt(t *testing.T) {
	opt := pdf.Array{pdf.String("HIVPOSITIVE"), pdf.String("NEGATIVE")}
	o := redactForm(t, radioDoc(t, opt, false, false))
	o.lacks("HIVPOSITIVE")
	o.has("NEGATIVE")
	f := o.field("r")
	if f == nil {
		t.Fatal("field was removed")
	}
	checkRadio(t, o, f, f["Opt"])
	checkStable(t, o.r)

	// an /Opt entry which does not match /Kids is removed
	opt = append(opt, pdf.String("EXTRA"))
	o = redactForm(t, radioDoc(t, opt, false, false))
	o.lacks("HIVPOSITIVE")
	if f := o.field("r"); f == nil || f["Opt"] != nil {
		t.Error("kept /Opt which does not match /Kids")
	}
}

// A radio button field which inherits /Opt gets an /Opt of its own.  The
// group keeps /Opt only if another field uses it.
func TestFormRadioInheritedOpt(t *testing.T) {
	opt := pdf.Array{pdf.String("HIVPOSITIVE"), pdf.String("NEGATIVE")}
	o := redactForm(t, radioDoc(t, opt, true, false))
	o.lacks("HIVPOSITIVE")
	g := o.field("g")
	if g == nil {
		t.Fatal("group was removed")
	}
	if g["Opt"] != nil {
		t.Error("group kept /Opt")
	}
	f := o.kid(g, "r")
	if f == nil {
		t.Fatal("field was removed")
	}
	checkRadio(t, o, f, f["Opt"])

	o = redactForm(t, radioDoc(t, opt, true, true))
	g = o.field("g")
	if g == nil {
		t.Fatal("group was removed")
	}
	if g["Opt"] == nil {
		t.Error("group lost /Opt which another field uses")
	}
	f = o.kid(g, "r")
	if f == nil {
		t.Fatal("field was removed")
	}
	checkRadio(t, o, f, f["Opt"])
}
