// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package pagetree_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/page"
	"seehuhn.de/go/pdf/pagetree"
)

func TestFindPages(t *testing.T) {
	doc, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)

	numPages := 234
	pageRefsIn := make([]pdf.Reference, numPages)
	rm := pdf.NewResourceManager(doc)
	tree := pagetree.NewWriter(doc, rm)
	for i := range numPages {
		pageRefsIn[i] = doc.Alloc()
		p := &page.Page{
			MediaBox: &pdf.Rectangle{URx: 100, URy: 100},
		}
		err := tree.AppendPageRef(pageRefsIn[i], p)
		if err != nil {
			t.Fatal(err)
		}
	}
	treeRef, err := tree.Close()
	if err != nil {
		t.Fatal(err)
	}
	doc.GetMeta().Catalog.Pages = treeRef
	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	pageRefsOut, err := pagetree.FindPages(doc)
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(pageRefsIn, pageRefsOut); d != "" {
		t.Fatalf("unexpected pageRefs (-want +got):\n%s", d)
	}
}

func TestFindPagesInvalidKid(t *testing.T) {
	// Test that invalid kids get 0 placeholder at the correct position.
	// A malformed PDF might have non-reference entries in Kids array.
	data, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)

	// Create three page objects
	page0Ref := data.Alloc()
	page1Ref := data.Alloc()
	page2Ref := data.Alloc()

	for _, ref := range []pdf.Reference{page0Ref, page1Ref, page2Ref} {
		data.Put(ref, pdf.Dict{
			"Type": pdf.Name("Page"),
		})
	}

	// Create root with kids [page0, invalidEntry, page2]
	// The invalid entry is an inline dict instead of a reference
	rootRef := data.Alloc()
	root := pdf.Dict{
		"Type":  pdf.Name("Pages"),
		"Count": pdf.Integer(3),
		"Kids": pdf.Array{
			page0Ref,
			pdf.Dict{"Type": pdf.Name("Page")}, // invalid: not a reference
			page2Ref,
		},
	}
	data.Put(rootRef, root)
	data.GetMeta().Catalog.Pages = rootRef

	pages, err := pagetree.FindPages(data)
	if err != nil {
		t.Fatal(err)
	}

	// Invalid kids are silently skipped (permissive reader)
	expected := []pdf.Reference{page0Ref, page2Ref}
	if d := cmp.Diff(expected, pages); d != "" {
		t.Errorf("unexpected pages (-want +got):\n%s", d)
	}
}

func TestIterator(t *testing.T) {
	data, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)

	n := 10
	refs := make([]pdf.Reference, n)
	for i := range refs {
		refs[i] = data.Alloc()
	}
	dicts := make([]pdf.Dict, n)
	for i := range dicts {
		dicts[i] = pdf.Dict{
			"Type": pdf.Name("Page"),
		}
		if i == 1 {
			dicts[i]["Resources"] = pdf.Name("Q")
		}
		data.Put(refs[i], dicts[i])
	}

	internal00Ref := data.Alloc()
	internal00 := pdf.Dict{
		"Type":  pdf.Name("Pages"),
		"Count": pdf.Integer(3),
		"Kids":  pdf.Array{refs[1], refs[2]},
	}
	data.Put(internal00Ref, internal00)

	internal0Ref := data.Alloc()
	internal0 := pdf.Dict{
		"Type":      pdf.Name("Pages"),
		"Count":     pdf.Integer(3),
		"Kids":      pdf.Array{internal00Ref, refs[3]},
		"Resources": pdf.Name("P"),
	}
	data.Put(internal0Ref, internal0)

	internal10Ref := data.Alloc()
	internal10 := pdf.Dict{
		"Type":     pdf.Name("Pages"),
		"Count":    pdf.Integer(2),
		"Kids":     pdf.Array{refs[4], refs[5]},
		"MediaBox": pdf.Name("A"),
	}
	data.Put(internal10Ref, internal10)

	internal11Ref := data.Alloc()
	internal11 := pdf.Dict{
		"Type":     pdf.Name("Pages"),
		"Count":    pdf.Integer(3),
		"Kids":     pdf.Array{refs[7], refs[8], refs[9]},
		"MediaBox": pdf.Name("B"),
		"Rotate":   pdf.Integer(180),
	}
	data.Put(internal11Ref, internal11)

	internal1Ref := data.Alloc()
	internal1 := pdf.Dict{
		"Type":   pdf.Name("Pages"),
		"Count":  pdf.Integer(7),
		"Kids":   pdf.Array{internal10Ref, refs[6], internal11Ref},
		"Rotate": pdf.Integer(90),
	}
	data.Put(internal1Ref, internal1)

	rootRef := data.Alloc()
	root := pdf.Dict{
		"Type":  pdf.Name("Pages"),
		"Count": pdf.Integer(n),
		"Kids":  pdf.Array{refs[0], internal0Ref, internal1Ref},
	}
	data.Put(rootRef, root)
	data.GetMeta().Catalog.Pages = rootRef

	expectedResource := []pdf.Object{
		nil, pdf.Name("Q"), pdf.Name("P"), pdf.Name("P"), nil, nil, nil, nil, nil, nil,
	}
	expectedRotate := []pdf.Object{
		nil, nil, nil, nil, pdf.Integer(90), pdf.Integer(90), pdf.Integer(90), pdf.Integer(180), pdf.Integer(180), pdf.Integer(180),
	}

	var gotReferences []pdf.Reference
	var gotResources []pdf.Object
	var gotRotate []pdf.Object
	for ref, dict := range pagetree.NewIterator(data).All() {
		gotReferences = append(gotReferences, ref)
		gotResources = append(gotResources, dict["Resources"])
		gotRotate = append(gotRotate, dict["Rotate"])
	}

	if d := cmp.Diff(refs, gotReferences); d != "" {
		t.Fatalf("unexpected references (-want +got):\n%s", d)
	}
	if d := cmp.Diff(expectedResource, gotResources); d != "" {
		t.Fatalf("unexpected resources (-want +got):\n%s", d)
	}
	if d := cmp.Diff(expectedRotate, gotRotate); d != "" {
		fmt.Println(gotRotate)
		t.Fatalf("unexpected rotations (-want +got):\n%s", d)
	}
}

// walkers lists the two ways of obtaining a page view, so that tests can
// check both for the same behaviour.
var walkers = []struct {
	name string
	get  func(r pdf.Getter) (pdf.Dict, error)
}{
	{"Iterator", func(r pdf.Getter) (pdf.Dict, error) {
		it := pagetree.NewIterator(r)
		var got pdf.Dict
		n := 0
		for _, dict := range it.All() {
			got = dict
			n++
		}
		if it.Err != nil {
			return nil, it.Err
		}
		if n != 1 {
			return nil, fmt.Errorf("got %d pages, want 1", n)
		}
		return got, nil
	}},
	{"GetPage", func(r pdf.Getter) (pdf.Dict, error) {
		_, dict, err := pagetree.GetPage(r, 0)
		return dict, err
	}},
}

func TestInheritUnusableBox(t *testing.T) {
	rootBox := pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(595), pdf.Integer(842)}
	ownBox := pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(100), pdf.Integer(200)}
	letter := pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(612), pdf.Integer(792)}
	for _, tc := range []struct {
		name    string
		box     pdf.Object // the page's own MediaBox, nil for none
		rootBox pdf.Object // the root's MediaBox, nil for none
		want    pdf.Object
	}{
		{"missing", nil, rootBox, rootBox},
		{"null element", pdf.Array{pdf.Integer(1), nil, pdf.Integer(2), pdf.Integer(3)}, rootBox, rootBox},
		{"wrong length", pdf.Array{pdf.Integer(1), pdf.Integer(2), pdf.Integer(3)}, rootBox, rootBox},
		{"zero area", pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(0), pdf.Integer(100)}, rootBox, rootBox},
		{"usable", ownBox, rootBox, ownBox},
		{"none anywhere", nil, nil, letter},
		{"unusable everywhere", pdf.Array{pdf.Integer(1), pdf.Integer(2), pdf.Integer(3)}, nil, letter},
	} {
		for _, walker := range walkers {
			t.Run(tc.name+"/"+walker.name, func(t *testing.T) {
				data, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
				rootRef := data.Alloc()
				midRef := data.Alloc()
				pageRef := data.Alloc()

				page := pdf.Dict{"Type": pdf.Name("Page"), "Parent": midRef}
				if tc.box != nil {
					page["MediaBox"] = tc.box
				}
				data.Put(pageRef, page)
				// the intermediate node's box is unusable and must be skipped
				data.Put(midRef, pdf.Dict{
					"Type":     pdf.Name("Pages"),
					"Parent":   rootRef,
					"Count":    pdf.Integer(1),
					"Kids":     pdf.Array{pageRef},
					"MediaBox": pdf.Array{pdf.Integer(1), nil, pdf.Integer(2), pdf.Integer(3)},
				})
				root := pdf.Dict{
					"Type":  pdf.Name("Pages"),
					"Count": pdf.Integer(1),
					"Kids":  pdf.Array{midRef},
				}
				if tc.rootBox != nil {
					root["MediaBox"] = tc.rootBox
				}
				data.Put(rootRef, root)
				data.GetMeta().Catalog.Pages = rootRef

				dict, err := walker.get(data)
				if err != nil {
					t.Fatal(err)
				}
				if d := cmp.Diff(tc.want, dict["MediaBox"]); d != "" {
					t.Errorf("unexpected MediaBox (-want +got):\n%s", d)
				}
				if _, ok := dict["Parent"]; ok {
					t.Error("view still has a Parent entry")
				}
			})
		}
	}
}

func TestInheritNullBox(t *testing.T) {
	// An intermediate node with "/MediaBox null" must not hide the root's
	// box.  The writer drops nil dictionary entries, so the null value is
	// patched into the file bytes after writing.
	rootBox := pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(595), pdf.Integer(842)}
	for _, walker := range walkers {
		t.Run(walker.name, func(t *testing.T) {
			w, f := memfile.NewPDFWriter(t, pdf.V1_4, nil)
			rootRef := w.Alloc()
			midRef := w.Alloc()
			pageRef := w.Alloc()
			w.Put(pageRef, pdf.Dict{"Type": pdf.Name("Page"), "Parent": midRef})
			w.Put(midRef, pdf.Dict{
				"Type":     pdf.Name("Pages"),
				"Parent":   rootRef,
				"Count":    pdf.Integer(1),
				"Kids":     pdf.Array{pageRef},
				"MediaBox": pdf.Name("NULLME"),
			})
			w.Put(rootRef, pdf.Dict{
				"Type":     pdf.Name("Pages"),
				"Count":    pdf.Integer(1),
				"Kids":     pdf.Array{midRef},
				"MediaBox": rootBox,
			})
			w.GetMeta().Catalog.Pages = rootRef
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			// same length, so that the cross-reference offsets stay valid
			data := bytes.Replace(f.Data, []byte("/NULLME"), []byte(" null  "), 1)
			if bytes.Equal(data, f.Data) {
				t.Fatal("marker not found in file")
			}
			r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)), nil)
			if err != nil {
				t.Fatal(err)
			}

			dict, err := walker.get(r)
			if err != nil {
				t.Fatal(err)
			}
			if d := cmp.Diff(rootBox, dict["MediaBox"]); d != "" {
				t.Errorf("unexpected MediaBox (-want +got):\n%s", d)
			}
		})
	}
}
