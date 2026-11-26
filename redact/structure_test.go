// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/numtree"
)

// structParts describes a tagged document.  The page shows the text "secret"
// inside the redaction region and "keep" outside of it.  A test can change
// the parts before the document is written.
type structParts struct {
	root   pdf.Dict
	elems  map[pdf.Reference]pdf.Dict
	nums   pdf.Array
	form   pdf.Dict // extra entries for the form, nil for no form
	annots []pdf.Dict

	rootRef, docRef, pRef, qRef, aRef, fRef, gRef pdf.Reference
	annotRef, keptRef                             pdf.Reference

	// directRoot makes the structure tree root a direct dictionary of the
	// catalog.  Entries of extra are written as additional objects, and
	// arrRef is a reference which a test can use for one of them.
	directRoot bool
	extra      map[pdf.Reference]pdf.Object
	arrRef     pdf.Reference

	// streams are written as additional streams without data
	streams map[pdf.Reference]pdf.Dict

	// alloc allocates a reference for an object of extra
	alloc func() pdf.Reference

	// formRes holds extra resource categories of the form.  keptForm holds
	// extra entries for a second form, which paints outside the region, and
	// is nil for no such form.
	formRes  pdf.Dict
	keptForm pdf.Dict

	// content replaces the page content if it is not empty, and props is
	// the /Properties resource category of the page
	content string
	props   pdf.Dict

	// pageParents is the /StructParents entry of the page, annotParent the
	// /StructParent entry of the deleted annotation of the structure tree,
	// and moreAnnots lists objects of extra which are added to the
	// annotations of the page
	pageParents, annotParent pdf.Object
	moreAnnots               []pdf.Reference
}

func structDoc(t testing.TB, tweak func(p *structParts)) *pdf.Reader {
	t.Helper()
	w, buf := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	put := func(obj pdf.Object) pdf.Reference {
		ref := w.Alloc()
		if err := w.Put(ref, obj); err != nil {
			t.Fatal(err)
		}
		return ref
	}
	stream := func(dict pdf.Dict, data string) pdf.Reference {
		ref := w.Alloc()
		stm, err := w.OpenStream(ref, dict, pdf.FilterCompress{})
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

	p := &structParts{
		rootRef: w.Alloc(), docRef: w.Alloc(), pRef: w.Alloc(), qRef: w.Alloc(),
		aRef: w.Alloc(), fRef: w.Alloc(), gRef: w.Alloc(),
		annotRef: w.Alloc(), keptRef: w.Alloc(), arrRef: w.Alloc(),
		pageParents: pdf.Integer(0), annotParent: pdf.Integer(1),
		alloc: w.Alloc, extra: map[pdf.Reference]pdf.Object{},
	}
	pagesRef, pageRef := w.Alloc(), w.Alloc()

	p.root = pdf.Dict{"Type": pdf.Name("StructTreeRoot"), "K": p.docRef}
	elem := func(s pdf.Name, parent pdf.Reference, extra pdf.Dict) pdf.Dict {
		d := pdf.Dict{
			"Type": pdf.Name("StructElem"), "S": s, "P": parent, "Pg": pageRef,
		}
		maps.Copy(d, extra)
		return d
	}
	p.elems = map[pdf.Reference]pdf.Dict{
		p.docRef: elem("Document", p.rootRef, pdf.Dict{
			"Alt": pdf.String("docalt"),
			"K":   pdf.Array{p.pRef, p.qRef, p.aRef, p.fRef, p.gRef},
		}),
		p.pRef: elem("P", p.docRef, pdf.Dict{
			"ActualText": pdf.String("secretAT"), "Alt": pdf.String("secretAlt"),
			"E": pdf.String("secretE"), "K": pdf.Integer(0),
		}),
		p.qRef: elem("P", p.docRef, pdf.Dict{
			"ActualText": pdf.String("keepAT"), "K": pdf.Integer(1),
		}),
		p.aRef: elem("Link", p.docRef, pdf.Dict{
			"Alt": pdf.String("linkAlt"),
			"K": pdf.Array{
				pdf.Dict{"Type": pdf.Name("OBJR"), "Obj": p.annotRef, "Pg": pageRef},
				pdf.Dict{"Type": pdf.Name("OBJR"), "Obj": p.keptRef, "Pg": pageRef},
			},
		}),
		p.fRef: elem("P", p.docRef, pdf.Dict{
			"ActualText": pdf.String("formAT"), "K": pdf.Integer(0),
		}),
		p.gRef: elem("Figure", p.docRef, pdf.Dict{
			"Alt": pdf.String("formAlt"),
			"K":   pdf.Dict{"Type": pdf.Name("OBJR"), "Obj": pageRef},
		}),
	}
	p.nums = pdf.Array{
		pdf.Integer(0), pdf.Array{p.pRef, p.qRef},
		pdf.Integer(1), p.aRef,
		pdf.Integer(5), pdf.Array{p.fRef},
		pdf.Integer(9), p.gRef,
	}
	p.annots = []pdf.Dict{
		{"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Redact"), "P": pageRef,
			"Rect": rect(0, 0, 300, 100)},
	}
	if tweak != nil {
		tweak(p)
	}

	fontRef := put(pdf.Dict{
		"Type": pdf.Name("Font"), "Subtype": pdf.Name("Type1"),
		"BaseFont": pdf.Name("Helvetica"),
	})
	fonts := pdf.Dict{"Font": pdf.Dict{"F1": fontRef}}
	resources := pdf.Dict{"Font": pdf.Dict{"F1": fontRef}}
	data := "/P <</MCID 0>> BDC BT /F1 12 Tf 10 20 Td (secret) Tj ET EMC\n" +
		"/P <</MCID 1>> BDC BT /F1 12 Tf 10 250 Td (keep) Tj ET EMC\n"
	if p.content != "" {
		data = p.content
	}
	if p.props != nil {
		resources["Properties"] = p.props
	}
	xobjects := pdf.Dict{}
	if p.form != nil {
		formRes := maps.Clone(fonts)
		maps.Copy(formRes, p.formRes)
		fd := pdf.Dict{
			"Type": pdf.Name("XObject"), "Subtype": pdf.Name("Form"),
			"BBox": rect(0, 0, 300, 300), "Resources": formRes,
		}
		maps.Copy(fd, p.form)
		fm := stream(fd, "/P <</MCID 0>> BDC BT /F1 12 Tf 10 50 Td (formsecret) Tj ET EMC")
		xobjects["Fm1"] = fm
		data += "q /Fm1 Do Q\n"
	}
	if p.keptForm != nil {
		fd := pdf.Dict{
			"Type": pdf.Name("XObject"), "Subtype": pdf.Name("Form"),
			"BBox": rect(0, 0, 300, 300), "Resources": fonts,
		}
		maps.Copy(fd, p.keptForm)
		xobjects["Fm2"] = stream(fd, "BT /F1 12 Tf 10 280 Td (formkeep) Tj ET")
		data += "q /Fm2 Do Q\n"
	}
	if len(xobjects) > 0 {
		resources["XObject"] = xobjects
	}
	contentRef := stream(pdf.Dict{}, data)

	annots := pdf.Array{}
	for _, a := range p.annots {
		annots = append(annots, put(a))
	}
	// the deleted and the kept annotation of the structure tree
	for ref, a := range map[pdf.Reference]pdf.Dict{
		p.annotRef: {"Rect": rect(10, 10, 30, 30), "StructParent": p.annotParent},
		p.keptRef:  {"Rect": rect(10, 200, 30, 220), "StructParent": pdf.Integer(2)},
	} {
		a["Type"] = pdf.Name("Annot")
		a["Subtype"] = pdf.Name("Text")
		a["P"] = pageRef
		a["Contents"] = pdf.String("note")
		if err := w.Put(ref, a); err != nil {
			t.Fatal(err)
		}
		annots = append(annots, ref)
	}
	for _, ref := range p.moreAnnots {
		annots = append(annots, ref)
	}
	for ref, d := range p.elems {
		if err := w.Put(ref, d); err != nil {
			t.Fatal(err)
		}
	}
	for ref, o := range p.extra {
		if err := w.Put(ref, o); err != nil {
			t.Fatal(err)
		}
	}
	for ref, d := range p.streams {
		stm, err := w.OpenStream(ref, d)
		if err != nil {
			t.Fatal(err)
		}
		if err := stm.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if p.nums != nil {
		p.root["ParentTree"] = pdf.Dict{"Nums": p.nums}
	}
	if !p.directRoot {
		if err := w.Put(p.rootRef, p.root); err != nil {
			t.Fatal(err)
		}
	}

	if err := w.Put(pagesRef, pdf.Dict{
		"Type": pdf.Name("Pages"), "Count": pdf.Integer(1), "Kids": pdf.Array{pageRef},
	}); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(pageRef, pdf.Dict{
		"Type": pdf.Name("Page"), "Parent": pagesRef,
		"MediaBox": rect(0, 0, 300, 300), "Resources": resources,
		"Contents": contentRef, "Annots": annots, "StructParents": p.pageParents,
	}); err != nil {
		t.Fatal(err)
	}
	meta := w.GetMeta()
	meta.Catalog.Pages = pagesRef
	meta.Catalog.StructTreeRoot = p.rootRef
	if p.directRoot {
		meta.Catalog.StructTreeRoot = p.root
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	r := openSource(t, buf.Data)
	return r
}

// redactStruct applies the redactions of the document and returns the output
// together with its bytes.
func redactStruct(t testing.TB, in *pdf.Reader) (*pdf.Reader, []byte) {
	t.Helper()
	var buf bytes.Buffer
	err := Write(&buf, in, &Options{Writer: &pdf.WriterOptions{HumanReadable: true}})
	if err != nil {
		t.Fatal(err)
	}
	out := openPDF(t, buf.Bytes())
	return out, buf.Bytes()
}

// outElem returns the structure element for a parent tree entry of the
// output.  If idx is negative, the entry is the element itself.
func outElem(t testing.TB, r *pdf.Reader, key, idx int) pdf.Dict {
	t.Helper()
	root, err := pdf.Resolve(r, r.GetMeta().Catalog.StructTreeRoot)
	if err != nil {
		t.Fatal(err)
	}
	pt, err := numtree.ExtractFromFile(r, root.(pdf.Dict)["ParentTree"])
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
	d, err := pdf.Resolve(r, val)
	if err != nil {
		t.Fatal(err)
	}
	return d.(pdf.Dict)
}

func parentOf(t testing.TB, r *pdf.Reader, e pdf.Dict) pdf.Dict {
	t.Helper()
	d, err := pdf.Resolve(r, e["P"])
	if err != nil {
		t.Fatal(err)
	}
	return d.(pdf.Dict)
}

// strippedKeys lists the entries of a structure element which can repeat its
// content
var strippedKeys = []pdf.Name{"ActualText", "Alt", "E", "Phoneme", "T", "AF"}

func checkStripped(t testing.TB, name string, e pdf.Dict) {
	t.Helper()
	for _, k := range strippedKeys {
		if _, ok := e[k]; ok {
			t.Errorf("%s still has /%s", name, k)
		}
	}
}

func TestStructureMCID(t *testing.T) {
	out, data := redactStruct(t, structDoc(t, nil))

	p := outElem(t, out, 0, 0)
	checkStripped(t, "element of the removed text", p)
	doc := parentOf(t, out, p)
	checkStripped(t, "ancestor", doc)
	if doc["S"] != pdf.Name("Document") {
		t.Errorf("wrong ancestor %v", doc["S"])
	}
	for _, s := range []string{"secretAT", "secretAlt", "secretE", "docalt"} {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("output still contains %q", s)
		}
	}

	q := outElem(t, out, 0, 1)
	if q["ActualText"] == nil {
		t.Error("element of untouched content lost ActualText")
	}
}

func TestStructureOBJR(t *testing.T) {
	out, data := redactStruct(t, structDoc(t, nil))
	if bytes.Contains(data, []byte("linkAlt")) {
		t.Error("element of the deleted annotation kept its Alt")
	}
	checkKeptOBJR(t, out)
}

// checkKeptOBJR checks that the element of parent tree key 1 has only the
// content item for the kept annotation.
func checkKeptOBJR(t testing.TB, out *pdf.Reader) {
	t.Helper()
	a := outElem(t, out, 1, -1)
	k, err := pdf.Resolve(out, a["K"])
	if err != nil {
		t.Fatal(err)
	}
	var items pdf.Array
	switch k := k.(type) {
	case pdf.Array:
		items = k
	case pdf.Dict:
		items = pdf.Array{k}
	}
	if len(items) != 1 {
		t.Fatalf("got %d content items, want 1", len(items))
	}
	d, err := pdf.Resolve(out, items[0])
	if err != nil {
		t.Fatal(err)
	}
	obj, err := pdf.Resolve(out, d.(pdf.Dict)["Obj"])
	if err != nil {
		t.Fatal(err)
	}
	if obj == nil {
		t.Fatal("remaining content item refers to nothing")
	}
	if got := obj.(pdf.Dict)["StructParent"]; got != pdf.Integer(2) {
		t.Errorf("remaining object is not the kept annotation: %v", obj)
	}
}

func TestStructureForm(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		p.form = pdf.Dict{"StructParents": pdf.Integer(5)}
	})
	out, data := redactStruct(t, in)
	checkStripped(t, "form element", outElem(t, out, 5, 0))
	if bytes.Contains(data, []byte("formAT")) {
		t.Error("output still contains the form's ActualText")
	}
}

func TestStructureFormObject(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		p.form = pdf.Dict{"StructParent": pdf.Integer(9)}
	})
	out, data := redactStruct(t, in)
	checkStripped(t, "element of the form object", outElem(t, out, 9, -1))
	if bytes.Contains(data, []byte("formAlt")) {
		t.Error("output still contains the Alt text")
	}
}

func TestStructureMalformed(t *testing.T) {
	cases := map[string]func(p *structParts){
		"parent cycle": func(p *structParts) {
			p.elems[p.docRef]["P"] = p.pRef
			p.elems[p.pRef]["P"] = p.docRef
		},
		"self parent": func(p *structParts) {
			p.elems[p.pRef]["P"] = p.pRef
		},
		"no parent tree": func(p *structParts) { p.nums = nil },
		"parent tree type": func(p *structParts) {
			p.nums = nil
			p.root["ParentTree"] = pdf.Integer(3)
		},
		"nums type": func(p *structParts) {
			p.nums = pdf.Array{pdf.Integer(0), pdf.Integer(7), pdf.Name("x")}
		},
		"entry types": func(p *structParts) {
			p.nums = pdf.Array{
				pdf.Integer(0), pdf.Array{pdf.Integer(1), nil, pdf.String("x")},
				pdf.Integer(1), pdf.Integer(3),
			}
		},
		"missing element": func(p *structParts) {
			p.nums = pdf.Array{pdf.Integer(0), pdf.Array{pdf.Reference(9999)}}
		},
		"parent type": func(p *structParts) {
			p.elems[p.pRef]["P"] = pdf.Integer(1)
			p.elems[p.aRef]["K"] = pdf.Array{pdf.Integer(1), pdf.Name("x")}
		},
	}
	for name, tweak := range cases {
		t.Run(name, func(t *testing.T) {
			in := structDoc(t, tweak)
			out, _ := redactStruct(t, in)
			checkStable(t, out)
		})
	}
}

func TestStructureAbsent(t *testing.T) {
	in := structDoc(t, func(p *structParts) { p.nums = nil; p.root = pdf.Dict{} })
	out, _ := redactStruct(t, in)
	checkStable(t, out)

	// no structure tree at all
	in = textSourceOpt(t, "BT /F1 12 Tf 0 10 Td (secret) Tj ET", nil,
		pdf.Dict{"Rect": rect(0, 0, 300, 300)})
	out, _ = redactStruct(t, in)
	if out.GetMeta().Catalog.StructTreeRoot != nil {
		t.Error("structure tree appeared")
	}
}

// the texts of all elements of the structure test document
var structTexts = []string{
	"docalt", "secretAT", "secretAlt", "secretE", "keepAT", "linkAlt",
	"formAT", "formAlt",
}

func TestStructureFallback(t *testing.T) {
	withForm := func(p *structParts) { p.form = pdf.Dict{"StructParents": pdf.Integer(5)} }
	cases := map[string]func(p *structParts){
		"no parent tree": func(p *structParts) { p.nums = nil },
		"parent tree type": func(p *structParts) {
			p.nums = nil
			p.root["ParentTree"] = pdf.Integer(3)
		},
		"key not found": func(p *structParts) {
			p.nums = pdf.Array{pdf.Integer(7), pdf.Array{p.pRef}}
		},
		"entry type": func(p *structParts) {
			p.nums = pdf.Array{pdf.Integer(0), pdf.Integer(3)}
		},
		"index out of range": func(p *structParts) {
			p.nums = pdf.Array{pdf.Integer(0), pdf.Array{p.qRef}}
		},
		"null entry": func(p *structParts) {
			p.nums = pdf.Array{pdf.Integer(0), pdf.Array{nil, p.qRef}}
		},
		"missing element": func(p *structParts) {
			p.nums = pdf.Array{pdf.Integer(0), pdf.Array{pdf.Reference(9999), p.qRef}}
		},
		"missing parent": func(p *structParts) {
			delete(p.elems[p.pRef], "P")
		},
		"parent not a dictionary": func(p *structParts) {
			p.elems[p.pRef]["P"] = pdf.Reference(9999)
		},
		"parent not an element": func(p *structParts) {
			p.elems[p.pRef]["P"] = p.keptRef
		},
		"form without entry": func(p *structParts) {
			withForm(p)
			p.nums = pdf.Array{pdf.Integer(0), pdf.Array{p.pRef, p.qRef}}
		},
	}
	for name, tweak := range cases {
		t.Run(name, func(t *testing.T) {
			in := structDoc(t, tweak)
			_, data := redactStruct(t, in)
			for _, s := range structTexts {
				if bytes.Contains(data, []byte(s)) {
					t.Errorf("output still contains %q", s)
				}
			}
		})
	}
}

func TestStructureFallbackDirectElement(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		p.nums = nil
		p.elems[p.docRef]["K"] = pdf.Array{
			p.pRef, p.qRef,
			pdf.Dict{
				"Type": pdf.Name("StructElem"), "S": pdf.Name("Sect"), "P": p.docRef,
				"Alt": pdf.String("directAlt"),
				"K": pdf.Dict{
					"S": pdf.Name("P"), "P": p.docRef, "ActualText": pdf.String("nestedAT"),
				},
			},
		}
	})
	_, data := redactStruct(t, in)
	for _, s := range append([]string{"directAlt", "nestedAT"}, structTexts...) {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("output still contains %q", s)
		}
	}
}

func TestStructureFallbackCycle(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		p.nums = nil
		p.elems[p.pRef]["K"] = pdf.Array{p.docRef, p.pRef}
		p.elems[p.qRef]["K"] = p.qRef
	})
	redactStruct(t, in)
}

func TestStructureNoFallback(t *testing.T) {
	// a well-formed tree keeps the text of elements without lost content
	_, data := redactStruct(t, structDoc(t, nil))
	if !bytes.Contains(data, []byte("keepAT")) {
		t.Error("fallback applied to a well-formed tree")
	}
}

func TestStructureFallbackDirectRoot(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		p.nums = nil
		p.directRoot = true
	})
	_, data := redactStruct(t, in)
	for _, s := range structTexts {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("output still contains %q", s)
		}
	}
}

func TestStructureFallbackDirectRootKids(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		p.nums = nil
		p.directRoot = true
		p.root["K"] = pdf.Array{
			p.docRef,
			pdf.Dict{
				"Type": pdf.Name("StructElem"), "S": pdf.Name("Sect"),
				"Alt": pdf.String("directAlt"),
			},
		}
	})
	_, data := redactStruct(t, in)
	for _, s := range append([]string{"directAlt"}, structTexts...) {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("output still contains %q", s)
		}
	}
}

func TestStructureFallbackIndirectKids(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		p.nums = nil
		p.extra = map[pdf.Reference]pdf.Object{
			p.arrRef: pdf.Array{
				p.pRef, p.qRef,
				pdf.Dict{
					"Type": pdf.Name("StructElem"), "S": pdf.Name("Sect"), "P": p.docRef,
					"Alt": pdf.String("directAlt"),
				},
			},
		}
		p.elems[p.docRef]["K"] = p.arrRef
	})
	_, data := redactStruct(t, in)
	for _, s := range append([]string{"directAlt"}, structTexts...) {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("output still contains %q", s)
		}
	}
}

func TestStructureFallbackSelfReferencingKids(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		p.nums = nil
		p.extra = map[pdf.Reference]pdf.Object{
			p.arrRef: pdf.Array{p.arrRef, p.pRef, p.qRef, p.arrRef},
		}
		p.elems[p.docRef]["K"] = p.arrRef
	})
	_, data := redactStruct(t, in)
	for _, s := range structTexts {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("output still contains %q", s)
		}
	}
}

func TestStructureTextKeys(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		maps.Copy(p.elems[p.pRef], pdf.Dict{
			"Phoneme": pdf.String("secretPhoneme"),
			"T":       pdf.String("secretTitle"),
			"AF": pdf.Array{pdf.Dict{
				"Type": pdf.Name("Filespec"), "F": pdf.String("secretAF.xml"),
			}},
		})
		p.elems[p.docRef]["T"] = pdf.String("docTitle")
		p.elems[p.qRef]["T"] = pdf.String("keepTitle")
	})
	out, data := redactStruct(t, in)
	checkStripped(t, "element of the removed text", outElem(t, out, 0, 0))
	for _, s := range []string{"secretPhoneme", "secretTitle", "secretAF", "docTitle"} {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("output still contains %q", s)
		}
	}
	if !bytes.Contains(data, []byte("keepTitle")) {
		t.Error("element of untouched content lost its title")
	}
}

// TestStructureIndirectKeys checks that indirect /StructParents and
// /StructParent entries lead to the structure elements.
func TestStructureIndirectKeys(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		pageKey, annotKey := p.rootRef+500, p.rootRef+501
		p.extra = map[pdf.Reference]pdf.Object{
			pageKey:  pdf.Integer(0),
			annotKey: pdf.Integer(1),
		}
		p.pageParents = pageKey
		p.annotParent = annotKey
	})
	out, data := redactStruct(t, in)
	for _, s := range []string{"secretAT", "docalt", "linkAlt"} {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("output still contains %q", s)
		}
	}
	if !bytes.Contains(data, []byte("keepAT")) {
		t.Error("fallback applied")
	}
	checkKeptOBJR(t, out)
}

// TestStructureFallbackKeys checks that removed content whose element
// cannot be found, because of the key in the parent tree, makes every
// element lose its text entries.
func TestStructureFallbackKeys(t *testing.T) {
	cases := map[string]func(p *structParts){
		"page key missing":  func(p *structParts) { p.pageParents = nil },
		"page key type":     func(p *structParts) { p.pageParents = pdf.Name("x") },
		"page key negative": func(p *structParts) { p.pageParents = pdf.Integer(-1) },
		"page key dangling": func(p *structParts) { p.pageParents = pdf.Reference(9999) },
		"annotation key type": func(p *structParts) {
			p.annotParent = pdf.Name("x")
		},
		"annotation key dangling": func(p *structParts) {
			p.annotParent = pdf.Reference(9999)
		},
		"annotation key not found": func(p *structParts) {
			p.annotParent = pdf.Integer(77)
		},
	}
	for name, tweak := range cases {
		t.Run(name, func(t *testing.T) {
			_, data := redactStruct(t, structDoc(t, tweak))
			for _, s := range structTexts {
				if bytes.Contains(data, []byte(s)) {
					t.Errorf("output still contains %q", s)
				}
			}
		})
	}
}

// TestStructureFallbackDirectRootNoParent checks that a structure element
// without /P does not count as a child of a direct root.
func TestStructureFallbackDirectRootNoParent(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		p.directRoot = true
		p.root["K"] = pdf.Array{p.pRef, p.docRef}
		delete(p.elems[p.pRef], "P")
		p.annotParent = nil
	})
	_, data := redactStruct(t, in)
	for _, s := range []string{"secretAT", "keepAT"} {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("output still contains %q", s)
		}
	}
}

// TestBudgetParentTree checks that many removed marked-content sequences
// and many deleted annotations, all with entries in one parent tree leaf,
// are processed.
func TestBudgetParentTree(t *testing.T) {
	const n = 10000
	in := structDoc(t, func(p *structParts) {
		var content strings.Builder
		mcidElems := make(pdf.Array, n)
		linkKids := pdf.Array{p.elems[p.aRef]["K"].(pdf.Array)[1]}
		p.nums = pdf.Array{pdf.Integer(0), p.arrRef, pdf.Integer(1), p.aRef}
		p.extra = map[pdf.Reference]pdf.Object{p.arrRef: mcidElems}
		for i := range n {
			fmt.Fprintf(&content, "/P <</MCID %d>> BDC BT /F1 12 Tf 10 20 Td (s) Tj ET EMC\n", i)
			elem := p.rootRef + pdf.Reference(1000+i)
			p.elems[elem] = pdf.Dict{
				"Type": pdf.Name("StructElem"), "S": pdf.Name("Span"), "P": p.pRef,
				"ActualText": pdf.String(fmt.Sprintf("bulk%d", i)), "K": pdf.Integer(i),
			}
			mcidElems[i] = elem

			annot := p.rootRef + pdf.Reference(1000+n+i)
			p.extra[annot] = pdf.Dict{
				"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Square"),
				"Rect": rect(10, 10, 30, 30), "StructParent": pdf.Integer(10 + i),
			}
			p.moreAnnots = append(p.moreAnnots, annot)
			linkKids = append(linkKids, pdf.Dict{"Type": pdf.Name("OBJR"), "Obj": annot})
			p.nums = append(p.nums, pdf.Integer(10+i), p.aRef)
		}
		// the kept text has an identifier of its own
		fmt.Fprintf(&content, "/P <</MCID %d>> BDC BT /F1 12 Tf 10 250 Td (keep) Tj ET EMC\n", n)
		p.elems[p.qRef]["K"] = pdf.Integer(n)
		mcidElems = append(mcidElems, p.qRef)
		p.extra[p.arrRef] = mcidElems
		p.content = content.String()
		p.elems[p.aRef]["K"] = linkKids
		p.annotParent = nil
	})
	out, data := redactStruct(t, in)
	if bytes.Contains(data, []byte("bulk")) {
		t.Error("output still contains text entries of removed content")
	}
	if !bytes.Contains(data, []byte("keepAT")) {
		t.Error("fallback applied")
	}
	checkKeptOBJR(t, out)
}

// TestBudgetParentTreeFallback checks that the input of TestBudgetParentTree
// is processed in proportion to its size when the element of some removed
// content cannot be found, so that every structure element is stripped.
func TestBudgetParentTreeFallback(t *testing.T) {
	const n = 10000
	in := structDoc(t, func(p *structParts) {
		var content strings.Builder
		mcidElems := make(pdf.Array, n)
		linkKids := pdf.Array{p.elems[p.aRef]["K"].(pdf.Array)[1]}
		p.nums = pdf.Array{pdf.Integer(0), p.arrRef, pdf.Integer(1), p.aRef}
		p.extra = map[pdf.Reference]pdf.Object{p.arrRef: mcidElems}
		for i := range n {
			fmt.Fprintf(&content, "/P <</MCID %d>> BDC BT /F1 12 Tf 10 20 Td (s) Tj ET EMC\n", i)
			elem := p.rootRef + pdf.Reference(1000+i)
			p.elems[elem] = pdf.Dict{
				"Type": pdf.Name("StructElem"), "S": pdf.Name("Span"), "P": p.pRef,
				"ActualText": pdf.String(fmt.Sprintf("bulk%d", i)), "K": pdf.Integer(i),
			}
			mcidElems[i] = elem

			annot := p.rootRef + pdf.Reference(1000+n+i)
			p.extra[annot] = pdf.Dict{
				"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Square"),
				"Rect": rect(10, 10, 30, 30), "StructParent": pdf.Integer(10 + i),
			}
			p.moreAnnots = append(p.moreAnnots, annot)
			linkKids = append(linkKids, pdf.Dict{"Type": pdf.Name("OBJR"), "Obj": annot})
			p.nums = append(p.nums, pdf.Integer(10+i), p.aRef)
		}
		// the kept text has an identifier of its own
		fmt.Fprintf(&content, "/P <</MCID %d>> BDC BT /F1 12 Tf 10 250 Td (keep) Tj ET EMC\n", n)
		p.elems[p.qRef]["K"] = pdf.Integer(n)
		mcidElems = append(mcidElems, p.qRef)
		// the element of the first removed sequence is missing
		mcidElems[0] = nil
		p.extra[p.arrRef] = mcidElems
		p.content = content.String()
		p.elems[p.aRef]["K"] = linkKids
		p.annotParent = nil
	})
	out, data := redactStruct(t, in)
	if bytes.Contains(data, []byte("bulk")) {
		t.Error("output still contains text entries of removed content")
	}
	if bytes.Contains(data, []byte("keepAT")) {
		t.Error("fallback not applied")
	}
	checkKeptOBJR(t, out)
}

// An attrEntry is an attribute object of a structure element, together with
// its revision number.
type attrEntry struct {
	dict   pdf.Dict
	rev    pdf.Integer
	hasRev bool
}

// outAttrs returns the attribute objects in the /A entry of an element of
// the output.  It fails the test if a revision number does not follow an
// attribute object.
func outAttrs(t testing.TB, r *pdf.Reader, e pdf.Dict) []attrEntry {
	t.Helper()
	a, err := pdf.Resolve(r, e["A"])
	if err != nil {
		t.Fatal(err)
	}
	items, isArray := a.(pdf.Array)
	if !isArray {
		items = pdf.Array{a}
	}
	var res []attrEntry
	afterAttr := false
	for _, item := range items {
		v, err := pdf.Resolve(r, item)
		if err != nil {
			t.Fatal(err)
		}
		switch v := v.(type) {
		case pdf.Integer:
			if !afterAttr {
				t.Fatalf("revision number %d does not follow an attribute object", v)
			}
			res[len(res)-1].rev = v
			res[len(res)-1].hasRev = true
			afterAttr = false
		case pdf.Dict:
			res = append(res, attrEntry{dict: v})
			afterAttr = true
		case *pdf.Stream:
			res = append(res, attrEntry{dict: v.Dict})
			afterAttr = true
		case nil:
		default:
			t.Fatalf("unexpected item %v in /A", v)
		}
	}
	return res
}

// attrOf returns the attribute objects of the given owner.
func attrOf(entries []attrEntry, owner pdf.Name) []attrEntry {
	var res []attrEntry
	for _, e := range entries {
		if e.dict["O"] == owner {
			res = append(res, e)
		}
	}
	return res
}

func TestStructureAttributes(t *testing.T) {
	userProps := func(value string) pdf.Dict {
		return pdf.Dict{
			"O": pdf.Name("UserProperties"),
			"P": pdf.Array{pdf.Dict{"N": pdf.String("name"), "V": pdf.String(value)}},
		}
	}
	in := structDoc(t, func(p *structParts) {
		p.root["ClassMap"] = pdf.Dict{
			"cls": pdf.Array{
				userProps("classValue"),
				pdf.Dict{
					"O": pdf.Name("Table"), "Summary": pdf.String("classSummary"),
					"Scope": pdf.Name("Row"),
				},
			},
			"other": pdf.Dict{"O": pdf.Name("Table"), "Summary": pdf.String("otherSummary")},
		}
		p.elems[p.pRef]["C"] = pdf.Name("cls")
		p.elems[p.pRef]["A"] = pdf.Array{
			userProps("secretValue"), pdf.Integer(3),
			pdf.Dict{
				"O": pdf.Name("Table"), "Summary": pdf.String("secretSummary"),
				"ColSpan": pdf.Integer(2),
			},
			pdf.Integer(4),
			pdf.Dict{"O": pdf.Name("Layout"), "Placement": pdf.Name("Block")},
			pdf.Integer(5),
		}
		p.elems[p.qRef]["C"] = pdf.Array{pdf.Name("cls"), pdf.Name("other")}
		p.elems[p.qRef]["A"] = pdf.Array{
			userProps("keepValue"),
			pdf.Dict{"O": pdf.Name("Table"), "Summary": pdf.String("keepSummary")},
		}
	})
	out, data := redactStruct(t, in)
	// the class of the stripped element loses its text, also where other
	// elements use it
	for _, s := range []string{"secretValue", "secretSummary", "classValue", "classSummary"} {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("output still contains %q", s)
		}
	}
	for _, s := range []string{"otherSummary", "keepValue", "keepSummary"} {
		if !bytes.Contains(data, []byte(s)) {
			t.Errorf("output lost %q", s)
		}
	}
	root := resolveDict(t, out, out.GetMeta().Catalog.StructTreeRoot)
	cls := outAttrs(t, out, pdf.Dict{"A": resolveDict(t, out, root["ClassMap"])["cls"]})
	if table := attrOf(cls, "Table"); len(table) != 1 || table[0].dict["Scope"] != pdf.Name("Row") {
		t.Errorf("class lost its table attributes: %v", cls)
	}

	p := outElem(t, out, 0, 0)
	if p["C"] != pdf.Name("cls") {
		t.Errorf("element lost its attribute class: %v", p["C"])
	}
	attrs := outAttrs(t, out, p)
	if len(attrOf(attrs, "UserProperties")) != 0 {
		t.Error("user properties kept")
	}
	table := attrOf(attrs, "Table")
	if len(table) != 1 {
		t.Fatalf("got %d table attribute objects, want 1", len(table))
	}
	if table[0].dict["Summary"] != nil {
		t.Error("table attributes kept /Summary")
	}
	if table[0].dict["ColSpan"] != pdf.Integer(2) || table[0].rev != 4 {
		t.Errorf("table attributes changed: %v, revision %d", table[0].dict, table[0].rev)
	}
	layout := attrOf(attrs, "Layout")
	if len(layout) != 1 {
		t.Fatalf("got %d layout attribute objects, want 1", len(layout))
	}
	if layout[0].dict["Placement"] != pdf.Name("Block") || layout[0].rev != 5 {
		t.Errorf("layout attributes changed: %v, revision %d", layout[0].dict, layout[0].rev)
	}
	if len(attrs) != 2 {
		t.Errorf("got %d attribute objects, want 2", len(attrs))
	}

	q := outElem(t, out, 0, 1)
	qAttrs := outAttrs(t, out, q)
	if len(attrOf(qAttrs, "UserProperties")) != 1 {
		t.Error("element of untouched content lost its user properties")
	}
}

// isString reports whether obj is the string s.
func isString(obj pdf.Object, s string) bool {
	v, ok := obj.(pdf.String)
	return ok && string(v) == s
}

func TestStructureAttributesSingle(t *testing.T) {
	for _, indirect := range []bool{false, true} {
		t.Run(fmt.Sprintf("indirect=%t", indirect), func(t *testing.T) {
			in := structDoc(t, func(p *structParts) {
				var a pdf.Object = pdf.Dict{
					"O": pdf.Name("UserProperties"),
					"P": pdf.Array{pdf.Dict{"N": pdf.String("n"), "V": pdf.String("secretValue")}},
				}
				if indirect {
					ref := p.rootRef + 600
					p.extra = map[pdf.Reference]pdf.Object{ref: a}
					a = ref
				}
				p.elems[p.pRef]["A"] = a
				p.elems[p.docRef]["A"] = a
			})
			out, data := redactStruct(t, in)
			if bytes.Contains(data, []byte("secretValue")) {
				t.Error("output still contains the user property")
			}
			for _, e := range []pdf.Dict{outElem(t, out, 0, 0), parentOf(t, out, outElem(t, out, 0, 0))} {
				if len(outAttrs(t, out, e)) != 0 {
					t.Errorf("attribute objects remain: %v", e["A"])
				}
			}
		})
	}
}

// TestStructureAttributesShared checks that an indirect attribute object
// which a stripped and a kept element share keeps its entries for the kept
// element.
func TestStructureAttributesShared(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		table := p.rootRef + 600
		props := p.rootRef + 601
		p.extra = map[pdf.Reference]pdf.Object{
			table: pdf.Dict{
				"O": pdf.Name("Table"), "Summary": pdf.String("sharedSummary"),
				"Scope": pdf.Name("Row"),
			},
			props: pdf.Dict{
				"O": pdf.Name("UserProperties"),
				"P": pdf.Array{pdf.Dict{"N": pdf.String("n"), "V": pdf.String("sharedValue")}},
			},
		}
		p.elems[p.pRef]["A"] = pdf.Array{table, props, pdf.Integer(1)}
		p.elems[p.qRef]["A"] = pdf.Array{table, pdf.Integer(0), props}
	})
	out, _ := redactStruct(t, in)

	pAttrs := outAttrs(t, out, outElem(t, out, 0, 0))
	if len(attrOf(pAttrs, "UserProperties")) != 0 {
		t.Error("stripped element kept the shared user properties")
	}
	pTable := attrOf(pAttrs, "Table")
	if len(pTable) != 1 || pTable[0].dict["Summary"] != nil ||
		pTable[0].dict["Scope"] != pdf.Name("Row") || pTable[0].hasRev {
		t.Errorf("wrong table attributes of the stripped element: %v", pAttrs)
	}

	qAttrs := outAttrs(t, out, outElem(t, out, 0, 1))
	if len(attrOf(qAttrs, "UserProperties")) != 1 {
		t.Error("kept element lost the shared user properties")
	}
	qTable := attrOf(qAttrs, "Table")
	if len(qTable) != 1 || !isString(qTable[0].dict["Summary"], "sharedSummary") ||
		!qTable[0].hasRev || qTable[0].rev != 0 {
		t.Errorf("wrong table attributes of the kept element: %v", qAttrs)
	}
}

// TestStructureAttributesStream checks that an attribute stream loses its
// table summary.
func TestStructureAttributesStream(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		table := p.rootRef + 600
		props := p.rootRef + 601
		p.streams = map[pdf.Reference]pdf.Dict{
			table: {
				"O": pdf.Name("Table"), "Summary": pdf.String("streamSummary"),
				"RowSpan": pdf.Integer(3),
			},
			props: {"O": pdf.Name("UserProperties"), "P": pdf.String("streamValue")},
		}
		p.elems[p.pRef]["A"] = pdf.Array{table, pdf.Integer(2), props}
	})
	out, data := redactStruct(t, in)
	for _, s := range []string{"streamSummary", "streamValue"} {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("output still contains %q", s)
		}
	}
	attrs := outAttrs(t, out, outElem(t, out, 0, 0))
	if len(attrs) != 1 || attrs[0].dict["RowSpan"] != pdf.Integer(3) || attrs[0].rev != 2 {
		t.Errorf("wrong attributes %v", attrs)
	}
}

// TestStructureAttributesFallback checks that every element loses the text
// in its attribute objects, if the element of removed content cannot be
// found.
func TestStructureAttributesFallback(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		p.nums = nil
		p.elems[p.qRef]["A"] = pdf.Dict{
			"O": pdf.Name("Table"), "Summary": pdf.String("keepSummary"),
		}
		p.elems[p.docRef]["K"] = pdf.Array{
			p.pRef, p.qRef,
			pdf.Dict{
				"Type": pdf.Name("StructElem"), "S": pdf.Name("Sect"), "P": p.docRef,
				"A": pdf.Dict{"O": pdf.Name("UserProperties"), "P": pdf.String("directValue")},
			},
		}
	})
	_, data := redactStruct(t, in)
	for _, s := range []string{"keepSummary", "directValue"} {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("output still contains %q", s)
		}
	}
}

func TestStructureAttributesMalformed(t *testing.T) {
	up := pdf.Dict{"O": pdf.Name("UserProperties"), "P": pdf.String("secretValue")}
	cases := map[string]pdf.Object{
		"name":     pdf.Name("x"),
		"null":     nil,
		"dangling": pdf.Reference(9999),
		"items": pdf.Array{
			pdf.Integer(7), nil, pdf.Name("x"), up, pdf.Integer(2), pdf.Integer(3),
			pdf.Dict{"P": pdf.String("noOwner")},
			pdf.Dict{"O": pdf.Name("Layout")}, pdf.Integer(-1),
			pdf.Reference(9999), pdf.Array{up},
		},
		"empty": pdf.Array{},
	}
	for name, a := range cases {
		t.Run(name, func(t *testing.T) {
			in := structDoc(t, func(p *structParts) {
				p.elems[p.pRef]["A"] = a
			})
			out, data := redactStruct(t, in)
			if bytes.Contains(data, []byte("secretValue")) {
				t.Error("output still contains the user property")
			}
			checkStable(t, out)
		})
	}
}

// TestStructureAttributeOwners checks that an element which loses its text
// keeps only the attribute objects of the standard owners for geometry and
// semantics, without their text attributes.
func TestStructureAttributeOwners(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		ns := p.rootRef + 600
		p.extra = map[pdf.Reference]pdf.Object{
			ns: pdf.Dict{"Type": pdf.Name("Namespace"), "NS": pdf.String("http://example.com/ns")},
		}
		p.elems[p.pRef]["A"] = pdf.Array{
			pdf.Dict{"O": pdf.Name("HTML-5.00"), "title": pdf.String("htmlTitle")},
			pdf.Dict{"O": pdf.Name("ARIA-1.1"), "aria-label": pdf.String("ariaLabel")},
			pdf.Dict{"O": pdf.Name("NSO"), "NS": ns, "label": pdf.String("nsoLabel")},
			pdf.Dict{"O": pdf.Name("Example:Owner"), "Note": pdf.String("unknownNote")},
			pdf.Dict{"O": pdf.Name("CSS-3"), "content": pdf.String("cssContent")},
			pdf.Dict{
				"O": pdf.Name("PrintField"), "Role": pdf.Name("cb"),
				"Checked": pdf.Name("on"), "Desc": pdf.String("fieldDesc"),
			},
			pdf.Dict{
				"O": pdf.Name("Table"), "Scope": pdf.Name("Column"),
				"Short": pdf.String("headerShort"),
			},
			pdf.Dict{"O": pdf.Name("List"), "ListNumbering": pdf.Name("Decimal")},
			pdf.Dict{"O": pdf.Name("Layout"), "TextAlign": pdf.Name("Center")},
			pdf.Dict{"O": pdf.Name("Artifact"), "Type": pdf.Name("Pagination")},
		}
	})
	out, data := redactStruct(t, in)
	for _, s := range []string{
		"htmlTitle", "ariaLabel", "nsoLabel", "unknownNote", "cssContent",
		"fieldDesc", "headerShort",
	} {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("output still contains %q", s)
		}
	}

	attrs := outAttrs(t, out, outElem(t, out, 0, 0))
	for _, owner := range []pdf.Name{"HTML-5.00", "ARIA-1.1", "NSO", "Example:Owner", "CSS-3"} {
		if len(attrOf(attrs, owner)) != 0 {
			t.Errorf("attribute object of owner %s kept", owner)
		}
	}
	want := map[pdf.Name]pdf.Dict{
		"PrintField": {"O": pdf.Name("PrintField"), "Role": pdf.Name("cb"), "Checked": pdf.Name("on")},
		"Table":      {"O": pdf.Name("Table"), "Scope": pdf.Name("Column")},
		"List":       {"O": pdf.Name("List"), "ListNumbering": pdf.Name("Decimal")},
		"Layout":     {"O": pdf.Name("Layout"), "TextAlign": pdf.Name("Center")},
		"Artifact":   {"O": pdf.Name("Artifact"), "Type": pdf.Name("Pagination")},
	}
	for owner, w := range want {
		got := attrOf(attrs, owner)
		if len(got) != 1 {
			t.Errorf("got %d attribute objects of owner %s, want 1", len(got), owner)
			continue
		}
		if !maps.Equal(got[0].dict, w) {
			t.Errorf("attributes of owner %s: got %v, want %v", owner, got[0].dict, w)
		}
	}
}

// TestStructureUnknownKeys checks that a stripped element and its kept
// attribute objects keep only the entries the specification defines, while
// an element of untouched content keeps all its entries.
func TestStructureUnknownKeys(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		maps.Copy(p.elems[p.pRef], pdf.Dict{
			"ID":   pdf.String("elemID"),
			"Lang": pdf.String("en-GB"),
			"R":    pdf.Integer(2),
			"Note": pdf.String("secretNote"),
			"A": pdf.Array{
				pdf.Dict{
					"O": pdf.Name("Table"), "RowSpan": pdf.Integer(2),
					"Headers": pdf.Array{pdf.String("h1")},
					"Comment": pdf.String("secretComment"),
				},
				pdf.Integer(1),
			},
		})
		maps.Copy(p.elems[p.qRef], pdf.Dict{
			"Note": pdf.String("keepNote"),
			"A": pdf.Dict{
				"O": pdf.Name("Table"), "Comment": pdf.String("keepComment"),
				"Summary": pdf.String("keepSummary"),
			},
		})
	})
	out, data := redactStruct(t, in)
	for _, s := range []string{"secretNote", "secretComment"} {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("output still contains %q", s)
		}
	}

	p := outElem(t, out, 0, 0)
	for _, k := range []pdf.Name{"Type", "S", "P", "Pg", "K", "ID", "Lang", "R", "A"} {
		if p[k] == nil {
			t.Errorf("stripped element lost /%s", k)
		}
	}
	if p["S"] != pdf.Name("P") || p["R"] != pdf.Integer(2) || !isString(p["Lang"], "en-GB") {
		t.Errorf("standard entries changed: %v", p)
	}
	attrs := outAttrs(t, out, p)
	if len(attrs) != 1 || attrs[0].rev != 1 {
		t.Fatalf("wrong attribute objects %v", attrs)
	}
	if len(attrs[0].dict) != 3 || attrs[0].dict["RowSpan"] != pdf.Integer(2) ||
		attrs[0].dict["Headers"] == nil {
		t.Errorf("wrong table attributes %v", attrs[0].dict)
	}

	q := outElem(t, out, 0, 1)
	if !isString(q["Note"], "keepNote") {
		t.Error("element of untouched content lost its extra entry")
	}
	qAttrs := outAttrs(t, out, q)
	if len(qAttrs) != 1 || !isString(qAttrs[0].dict["Comment"], "keepComment") ||
		!isString(qAttrs[0].dict["Summary"], "keepSummary") {
		t.Errorf("attributes of untouched content changed: %v", qAttrs)
	}
}

// TestStructureFallbackIndirectContentItem checks that the fallback keeps an
// indirect object reference dictionary among the kids of an element.
func TestStructureFallbackIndirectContentItem(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		p.nums = pdf.Array{pdf.Integer(7), pdf.Array{p.pRef}}
		p.extra = map[pdf.Reference]pdf.Object{
			p.arrRef: pdf.Dict{"Type": pdf.Name("OBJR"), "Obj": p.keptRef},
		}
		p.elems[p.aRef]["K"] = p.arrRef
	})
	out, data := redactStruct(t, in)
	for _, s := range structTexts {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("output still contains %q", s)
		}
	}
	doc := parentOf(t, out, outElem(t, out, 7, 0))
	kids, err := pdf.Resolve(out, doc["K"])
	if err != nil {
		t.Fatal(err)
	}
	link, err := pdf.Resolve(out, kids.(pdf.Array)[2])
	if err != nil {
		t.Fatal(err)
	}
	objr, err := pdf.Resolve(out, link.(pdf.Dict)["K"])
	if err != nil {
		t.Fatal(err)
	}
	obj, err := pdf.Resolve(out, objr.(pdf.Dict)["Obj"])
	if err != nil {
		t.Fatal(err)
	}
	if objr.(pdf.Dict)["Type"] != pdf.Name("OBJR") || obj == nil {
		t.Errorf("content item changed: %v", objr)
	}
}

// TestStructureFallbackMalformedKids checks that the fallback treats a
// malformed kid as a structure element, so that its text is removed.
func TestStructureFallbackMalformedKids(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		p.nums = pdf.Array{pdf.Integer(7), pdf.Array{p.pRef}}
		noS, badType, foreign := p.alloc(), p.alloc(), p.alloc()
		p.extra = map[pdf.Reference]pdf.Object{
			noS:     pdf.Dict{"ActualText": pdf.String("malformedNoS")},
			badType: pdf.Dict{"Type": pdf.Name("Foo"), "S": pdf.Name("Span"), "Alt": pdf.String("malformedType")},
			foreign: pdf.Dict{"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Text"),
				"Rect": pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(1), pdf.Integer(1)},
				"T":    pdf.String("annotTitle")},
		}
		p.elems[p.aRef]["K"] = pdf.Array{noS, badType, foreign}
	})
	out, data := redactStruct(t, in)
	for _, s := range []string{"malformedNoS", "malformedType"} {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("output still contains %q", s)
		}
	}
	checkStable(t, out)
}

func TestStructureFallbackTreeElements(t *testing.T) {
	// the elements are reached only through the parent tree and the ID tree
	in := structDoc(t, func(p *structParts) {
		orphan, byID := p.alloc(), p.alloc()
		p.extra[orphan] = pdf.Dict{
			"Type": pdf.Name("StructElem"), "S": pdf.Name("P"), "P": p.docRef,
			"ActualText": pdf.String("orphanAT"),
		}
		p.extra[byID] = pdf.Dict{
			"Type": pdf.Name("StructElem"), "S": pdf.Name("P"), "P": p.docRef,
			"ID": pdf.String("e1"), "Alt": pdf.String("idAlt"),
		}
		// key 0 of the page is missing, which forces the fallback
		p.nums = pdf.Array{pdf.Integer(7), pdf.Array{orphan}}
		p.root["IDTree"] = pdf.Dict{"Names": pdf.Array{pdf.String("e1"), byID}}
	})
	_, data := redactStruct(t, in)
	for _, s := range append([]string{"orphanAT", "idAlt"}, structTexts...) {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("output still contains %q", s)
		}
	}
}

func TestStructureFallbackDeepKids(t *testing.T) {
	// a deep chain of indirect arrays of kids
	in := structDoc(t, func(p *structParts) {
		p.nums = nil
		var next pdf.Object = pdf.Dict{
			"Type": pdf.Name("StructElem"), "S": pdf.Name("P"), "P": p.docRef,
			"ActualText": pdf.String("deepAT"),
		}
		for range 5000 {
			ref := p.alloc()
			p.extra[ref] = pdf.Array{next}
			next = ref
		}
		p.elems[p.docRef]["K"] = pdf.Array{p.pRef, p.qRef, next}
	})
	_, data := redactStruct(t, in)
	for _, s := range append([]string{"deepAT"}, structTexts...) {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("output still contains %q", s)
		}
	}
}

func TestStructureCopyKeyDirectRoot(t *testing.T) {
	// the redacted copy of the form gets a new key, also where the root is a
	// direct object and the parent tree is indirect
	in := structDoc(t, func(p *structParts) {
		p.form = pdf.Dict{"StructParents": pdf.Integer(5)}
		p.directRoot = true
		p.extra[p.arrRef] = pdf.Dict{"Nums": p.nums}
		p.root["ParentTree"] = p.arrRef
		p.nums = nil
	})
	out, _ := redactStruct(t, in)
	keys := streamKeys(t, out, "Form", "StructParents")
	if len(keys) != 1 || keys[0] == 5 {
		t.Fatalf("got keys %v, want one new key", keys)
	}
	checkStripped(t, "element of the form", outElem(t, out, int(keys[0]), 0))

	root, err := pdf.Resolve(out, out.GetMeta().Catalog.StructTreeRoot)
	if err != nil {
		t.Fatal(err)
	}
	if next, _ := root.(pdf.Dict)["ParentTreeNextKey"].(pdf.Integer); next <= keys[0] {
		t.Errorf("/ParentTreeNextKey is %d, key %d", next, keys[0])
	}
}

// checkTexts checks that the output lacks the strings gone and keeps the
// strings kept.
func checkTexts(t testing.TB, data []byte, gone, kept []string) {
	t.Helper()
	for _, s := range gone {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("output still contains %q", s)
		}
	}
	for _, s := range kept {
		if !bytes.Contains(data, []byte(s)) {
			t.Errorf("output lost %q", s)
		}
	}
}

// An element whose /K names removed marked content loses its text, also
// where the parent tree names another element for the content.
func TestStructureContentInK(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		x := p.alloc()
		p.elems[x] = pdf.Dict{
			"Type": pdf.Name("StructElem"), "S": pdf.Name("Span"), "P": p.docRef,
			"Pg": p.elems[p.pRef]["Pg"], "K": pdf.Integer(0),
			"ActualText": pdf.String("claimAT"),
		}
		doc := p.elems[p.docRef]
		doc["K"] = append(doc["K"].(pdf.Array), x)
	})
	_, data := redactStruct(t, in)
	checkTexts(t, data, []string{"claimAT", "secretAT"}, []string{"keepAT"})
}

// The ancestors of an element are those through which /K reaches it, also
// where /P skips one of them.
func TestStructureAncestorsThroughK(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		div := p.alloc()
		p.elems[div] = pdf.Dict{
			"Type": pdf.Name("StructElem"), "S": pdf.Name("Div"), "P": p.docRef,
			"K": p.pRef, "ActualText": pdf.String("divAT"),
		}
		doc := p.elems[p.docRef]
		kids := slices.Clone(doc["K"].(pdf.Array))
		kids[0] = div
		doc["K"] = kids
	})
	_, data := redactStruct(t, in)
	checkTexts(t, data, []string{"divAT", "secretAT"}, []string{"keepAT"})
}

// Where the parent tree has a key more than once, the elements of all its
// entries lose their text.
func TestStructureDuplicateKey(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		// reached only through the parent tree
		x := p.alloc()
		p.elems[x] = pdf.Dict{
			"Type": pdf.Name("StructElem"), "S": pdf.Name("P"), "P": p.docRef,
			"ActualText": pdf.String("dupAT"),
		}
		p.nums = append(pdf.Array{pdf.Integer(0), pdf.Array{x, p.qRef}}, p.nums...)
	})
	_, data := redactStruct(t, in)
	checkTexts(t, data, []string{"dupAT", "secretAT"}, []string{"keepAT"})
}

// A deleted annotation without /StructParent is found through its object
// reference.
func TestStructureDeletedAnnotWithoutKey(t *testing.T) {
	in := structDoc(t, func(p *structParts) { p.annotParent = nil })
	out, data := redactStruct(t, in)
	checkTexts(t, data, []string{"linkAlt"}, []string{"keepAT"})
	checkKeptOBJR(t, out)
}

// A deep structure tree is processed without the fallback.
func TestStructureDeepTree(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		// p sits at the bottom of a chain of elements
		parent := p.docRef
		var top pdf.Reference
		for i := range 5000 {
			ref := p.alloc()
			p.elems[ref] = pdf.Dict{
				"Type": pdf.Name("StructElem"), "S": pdf.Name("Div"), "P": parent,
			}
			if i == 0 {
				top = ref
				p.elems[ref]["ActualText"] = pdf.String("topAT")
			} else {
				p.elems[parent]["K"] = ref
			}
			parent = ref
		}
		p.elems[parent]["K"] = p.pRef
		p.elems[p.pRef]["P"] = parent
		doc := p.elems[p.docRef]
		kids := slices.Clone(doc["K"].(pdf.Array))
		kids[0] = top
		doc["K"] = kids
	})
	_, data := redactStruct(t, in)
	checkTexts(t, data, []string{"topAT", "secretAT", "docalt"}, []string{"keepAT"})
}

// A reference cycle in the structure tree is read as null.
func TestStructureReferenceCycle(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		x, y := p.alloc(), p.alloc()
		p.extra[x] = y
		p.extra[y] = x
		doc := p.elems[p.docRef]
		doc["K"] = append(doc["K"].(pdf.Array), x)
		p.elems[p.qRef]["C"] = x
	})
	out, data := redactStruct(t, in)
	checkTexts(t, data, []string{"secretAT"}, []string{"keepAT"})
	checkStable(t, out)
}

// A parent tree with intermediate nodes is read, and written again with the
// keys of the redacted copies.
func TestStructureParentTreeKids(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		p.form = pdf.Dict{"StructParents": pdf.Integer(5)}
		leaf1, leaf2 := p.alloc(), p.alloc()
		p.extra[leaf1] = pdf.Dict{
			"Limits": pdf.Array{pdf.Integer(0), pdf.Integer(1)},
			"Nums":   p.nums[:4],
		}
		p.extra[leaf2] = pdf.Dict{
			"Limits": pdf.Array{pdf.Integer(5), pdf.Integer(9)},
			"Nums":   p.nums[4:],
		}
		p.nums = nil
		p.root["ParentTree"] = pdf.Dict{"Kids": pdf.Array{leaf1, leaf2}}
	})
	out, data := redactStruct(t, in)
	checkTexts(t, data, []string{"secretAT", "formAT", "linkAlt"}, []string{"keepAT"})
	checkStripped(t, "element of the text", outElem(t, out, 0, 0))
	keys := streamKeys(t, out, "Form", "StructParents")
	if len(keys) != 1 || keys[0] == 5 {
		t.Fatalf("got keys %v, want one new key", keys)
	}
	checkStripped(t, "element of the form", outElem(t, out, int(keys[0]), 0))
	checkKeptOBJR(t, out)
}

// A parent tree whose keys are out of order, or repeated, is written again
// in order.
func TestStructureParentTreeUnsorted(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		p.form = pdf.Dict{"StructParents": pdf.Integer(5)}
		p.nums = pdf.Array{
			pdf.Integer(9), p.gRef,
			pdf.Integer(5), pdf.Array{p.fRef},
			pdf.Integer(1), p.aRef,
			pdf.Integer(0), pdf.Array{p.pRef, p.qRef},
			pdf.Integer(1), p.aRef,
		}
	})
	out, data := redactStruct(t, in)
	checkTexts(t, data, []string{"secretAT", "formAT"}, []string{"keepAT"})

	root := resolveDict(t, out, out.GetMeta().Catalog.StructTreeRoot)
	pt, err := numtree.ExtractFromFile(out, root["ParentTree"])
	if err != nil {
		t.Fatal(err)
	}
	var got []pdf.Integer
	for key := range pt.All() {
		got = append(got, key)
	}
	if !slices.IsSorted(got) || len(slices.Compact(slices.Clone(got))) != len(got) {
		t.Errorf("keys %v are not increasing", got)
	}
	keys := streamKeys(t, out, "Form", "StructParents")
	if len(keys) != 1 || !slices.Contains(got, keys[0]) {
		t.Fatalf("form key %v is not in the parent tree %v", keys, got)
	}
	checkStripped(t, "element of the form", outElem(t, out, int(keys[0]), 0))
	if q := outElem(t, out, 0, 1); q["ActualText"] == nil {
		t.Error("element of untouched content lost ActualText")
	}
}

// contentItems returns the content items in /K of an element of the output
// which have the given type.
func contentItems(t testing.TB, r *pdf.Reader, e pdf.Dict, tp pdf.Name) []pdf.Dict {
	t.Helper()
	k, err := pdf.Resolve(r, e["K"])
	if err != nil {
		t.Fatal(err)
	}
	items, isArray := k.(pdf.Array)
	if !isArray {
		items = pdf.Array{k}
	}
	var res []pdf.Dict
	for _, item := range items {
		v, err := pdf.Resolve(r, item)
		if err != nil {
			t.Fatal(err)
		}
		if d, ok := v.(pdf.Dict); ok && d["Type"] == tp {
			res = append(res, d)
		}
	}
	return res
}

// formRefs returns the references to the form XObjects of r whose content
// contains text.
func formRefs(t testing.TB, r *pdf.Reader, text string) []pdf.Reference {
	t.Helper()
	var res []pdf.Reference
	for i := uint32(1); i < 200; i++ {
		ref := pdf.NewReference(i, 0)
		obj, err := r.Get(ref, true)
		if err != nil {
			continue
		}
		stm, ok := obj.(*pdf.Stream)
		if !ok || stm.Dict["Subtype"] != pdf.Name("Form") {
			continue
		}
		data, err := pdf.ReadAll(r, nil, stm, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte(text)) {
			res = append(res, ref)
		}
	}
	return res
}

// copiesDoc builds a document with two pages which both paint a form with
// the extra entries form and the content data.  The region of the first
// page covers its lower third.  The structure tree root has the kids and the
// parent tree entries which setup returns, for the form fm and the root.
func copiesDoc(t *testing.T, form pdf.Dict, data string, setup func(b *nestBuilder, fm, root pdf.Reference) (pdf.Array, pdf.Array)) *pdf.Reader {
	return nestDoc(t, []pdf.Dict{{"Rect": rect(0, 0, 300, 100)}}, func(b *nestBuilder) []nestPage {
		d := pdf.Dict{"Resources": helv()}
		maps.Copy(d, form)
		fm := b.form(d, data)
		rootRef := b.w.Alloc()
		kids, nums := setup(b, fm, rootRef)
		root := pdf.Dict{"Type": pdf.Name("StructTreeRoot"), "K": kids}
		if nums != nil {
			root["ParentTree"] = pdf.Dict{"Nums": nums}
		}
		if err := b.w.Put(rootRef, root); err != nil {
			t.Fatal(err)
		}
		b.w.GetMeta().Catalog.StructTreeRoot = rootRef
		res := pdf.Dict{"XObject": pdf.Dict{"Fm": fm}}
		return []nestPage{
			{content: "/Fm Do", res: res},
			{content: "/Fm Do", res: res},
		}
	})
}

const (
	copiesSecret = "BT /F1 12 Tf 20 20 Td (Secret) Tj ET"
	copiesKeep   = "BT /F1 12 Tf 20 250 Td (Keep) Tj ET"
)

// The marked-content references to a form which is partly redacted refer to
// its copies: the redacted copy and the unchanged copy on the page without
// regions.  The parent tree entry of every copy leads back to the element.
func TestStructureMCRCopies(t *testing.T) {
	data := "/P <</MCID 0>> BDC " + copiesSecret + " EMC /P <</MCID 1>> BDC " + copiesKeep + " EMC"
	in := copiesDoc(t, pdf.Dict{"StructParents": pdf.Integer(5)}, data, func(b *nestBuilder, fm, root pdf.Reference) (pdf.Array, pdf.Array) {
		e0 := b.put(pdf.Dict{
			"Type": pdf.Name("StructElem"), "S": pdf.Name("P"), "P": root,
			"K":          pdf.Dict{"Type": pdf.Name("MCR"), "Stm": fm, "MCID": pdf.Integer(0)},
			"ActualText": pdf.String("lostAT"),
		})
		e1 := b.put(pdf.Dict{
			"Type": pdf.Name("StructElem"), "S": pdf.Name("P"), "P": root,
			"K":          pdf.Dict{"Type": pdf.Name("MCR"), "Stm": fm, "MCID": pdf.Integer(1)},
			"ActualText": pdf.String("keepAT"),
		})
		return pdf.Array{e0, e1}, pdf.Array{pdf.Integer(5), pdf.Array{e0, e1}}
	})
	out, data2 := writeRaw(t, in)
	checkTexts(t, data2, []string{"lostAT"}, []string{"keepAT"})

	forms := formRefs(t, out, "Keep")
	if len(forms) != 2 {
		t.Fatalf("got %d forms, want 2", len(forms))
	}
	for idx := range 2 {
		var elemRef pdf.Reference
		seen := map[pdf.Reference]bool{}
		for _, ref := range forms {
			key := resolveDict(t, out, ref)["StructParents"].(pdf.Integer)
			r := outElemRef(t, out, int(key), idx)
			if elemRef != 0 && r != elemRef {
				t.Errorf("the copies of the form lead to different elements")
			}
			elemRef = r
		}
		for _, mcr := range contentItems(t, out, resolveDict(t, out, elemRef), "MCR") {
			stm, _ := mcr["Stm"].(pdf.Reference)
			if mcr["MCID"] != pdf.Integer(idx) {
				t.Errorf("got MCID %v, want %d", mcr["MCID"], idx)
			}
			seen[stm] = true
		}
		for _, ref := range forms {
			if !seen[ref] {
				t.Errorf("element %d has no content item for form %v", idx, ref)
			}
		}
	}
}

// The object references to a form which is partly redacted refer to its
// copies, and the element loses its text.
func TestStructureOBJRCopies(t *testing.T) {
	for _, withKey := range []bool{true, false} {
		form := pdf.Dict{}
		if withKey {
			form["StructParent"] = pdf.Integer(9)
		}
		in := copiesDoc(t, form, copiesSecret+" "+copiesKeep, func(b *nestBuilder, fm, root pdf.Reference) (pdf.Array, pdf.Array) {
			fig := b.put(pdf.Dict{
				"Type": pdf.Name("StructElem"), "S": pdf.Name("Figure"), "P": root,
				"K":   pdf.Dict{"Type": pdf.Name("OBJR"), "Obj": fm},
				"Alt": pdf.String("figAlt"),
			})
			other := b.put(pdf.Dict{
				"Type": pdf.Name("StructElem"), "S": pdf.Name("P"), "P": root,
				"Alt": pdf.String("otherAlt"),
			})
			nums := pdf.Array{}
			if withKey {
				nums = pdf.Array{pdf.Integer(9), fig}
			}
			return pdf.Array{fig, other}, nums
		})
		out, data := writeRaw(t, in)
		checkTexts(t, data, []string{"figAlt"}, []string{"otherAlt"})

		root := resolveDict(t, out, out.GetMeta().Catalog.StructTreeRoot)
		kids, err := pdf.Resolve(out, root["K"])
		if err != nil {
			t.Fatal(err)
		}
		figRef := kids.(pdf.Array)[0].(pdf.Reference)
		seen := map[pdf.Reference]bool{}
		for _, objr := range contentItems(t, out, resolveDict(t, out, figRef), "OBJR") {
			obj, _ := objr["Obj"].(pdf.Reference)
			seen[obj] = true
		}
		forms := formRefs(t, out, "Keep")
		if len(forms) != 2 {
			t.Fatalf("got %d forms, want 2", len(forms))
		}
		for _, ref := range forms {
			if !seen[ref] {
				t.Errorf("figure has no content item for form %v", ref)
			}
			if !withKey {
				continue
			}
			key := resolveDict(t, out, ref)["StructParent"].(pdf.Integer)
			if got := outElemRef(t, out, int(key), -1); got != figRef {
				t.Errorf("form %v leads to %v, want the figure %v", ref, got, figRef)
			}
		}
	}
}

// Direct elements, marked-content references to the page and shared arrays
// of kids are found by the walk from the root.
func TestStructureWalkKinds(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		page := p.elems[p.pRef]["Pg"]
		p.annotParent = nil
		p.root["K"] = pdf.Array{p.docRef, pdf.Dict{
			"Type": pdf.Name("StructElem"), "S": pdf.Name("Sect"), "Pg": page,
			"Alt": pdf.String("sectAlt"),
			"K":   pdf.Dict{"Type": pdf.Name("OBJR"), "Obj": p.annotRef},
		}}
		elem := func(extra pdf.Dict) pdf.Reference {
			ref := p.alloc()
			p.elems[ref] = pdf.Dict{"Type": pdf.Name("StructElem"), "S": pdf.Name("P"), "P": p.docRef}
			maps.Copy(p.elems[ref], extra)
			return ref
		}
		l2 := elem(pdf.Dict{
			"Alt": pdf.String("l2Alt"),
			"K":   pdf.Dict{"Type": pdf.Name("OBJR"), "Obj": p.annotRef},
		})
		m := elem(pdf.Dict{
			"ActualText": pdf.String("mcrAT"),
			"K":          pdf.Dict{"Type": pdf.Name("MCR"), "Pg": page, "MCID": pdf.Integer(0)},
		})
		z := elem(pdf.Dict{"Pg": page, "K": pdf.Integer(0), "ActualText": pdf.String("zAT")})
		p.extra[p.arrRef] = pdf.Array{z}
		x := elem(pdf.Dict{"Alt": pdf.String("xAlt"), "K": p.arrRef})
		y := elem(pdf.Dict{"Alt": pdf.String("yAlt"), "K": p.arrRef})
		doc := p.elems[p.docRef]
		doc["K"] = append(doc["K"].(pdf.Array), l2, m, x, y)
	})
	out, data := redactStruct(t, in)
	checkTexts(t, data,
		[]string{"sectAlt", "l2Alt", "mcrAT", "zAT", "xAlt", "yAlt", "secretAT"},
		[]string{"keepAT"})
	checkStable(t, out)
}

// The attribute classes of an element which loses its text lose their text
// too.
func TestStructureClassMap(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		p.root["ClassMap"] = pdf.Dict{
			"sum":   pdf.Dict{"O": pdf.Name("Table"), "Summary": pdf.String("SECRETSUM")},
			"plain": pdf.Dict{"O": pdf.Name("Layout"), "Placement": pdf.Name("Block")},
			"user": pdf.Dict{
				"O": pdf.Name("UserProperties"),
				"P": pdf.Array{pdf.Dict{"N": pdf.String("n"), "V": pdf.String("SECRETUSER")}},
			},
			"other": pdf.Dict{"O": pdf.Name("Table"), "Summary": pdf.String("otherSum")},
		}
		p.elems[p.pRef]["C"] = pdf.Array{
			pdf.Name("sum"), pdf.Integer(1), pdf.Name("missing"), pdf.Name("plain"), pdf.Name("user"),
		}
		p.elems[p.qRef]["C"] = pdf.Name("other")
	})
	out, data := redactStruct(t, in)
	checkTexts(t, data, []string{"SECRETSUM", "SECRETUSER"}, []string{"otherSum", "keepAT"})

	root := resolveDict(t, out, out.GetMeta().Catalog.StructTreeRoot)
	cm := resolveDict(t, out, root["ClassMap"])
	if cm["user"] != nil {
		t.Error("class without kept attributes is still defined")
	}
	if plain := resolveDict(t, out, cm["plain"]); plain["Placement"] != pdf.Name("Block") {
		t.Errorf("class without text changed: %v", plain)
	}
	if sum := resolveDict(t, out, cm["sum"]); sum["O"] != pdf.Name("Table") {
		t.Errorf("class lost its owner: %v", sum)
	}
}

// The kids of a direct structure tree root are changed in the root.
func TestStructureDirectRootWalk(t *testing.T) {
	in := structDoc(t, func(p *structParts) {
		// no parent tree entry is needed
		p.content = "BT /F1 12 Tf 10 20 Td (secret) Tj ET\n"
		p.annotParent = nil
		p.directRoot = true
		p.root["K"] = pdf.Array{p.docRef, pdf.Dict{
			"Type": pdf.Name("StructElem"), "S": pdf.Name("Sect"),
			"Alt": pdf.String("sectAlt"),
			"K":   pdf.Dict{"Type": pdf.Name("OBJR"), "Obj": p.annotRef},
		}}
	})
	out, data := redactStruct(t, in)
	checkTexts(t, data, []string{"sectAlt", "linkAlt"}, []string{"keepAT"})
	checkStable(t, out)
}
