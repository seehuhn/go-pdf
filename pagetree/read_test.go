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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/page"
	"seehuhn.de/go/pdf/pagetree"
)

func TestFindPages(t *testing.T) {
	doc, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	numPages := 234
	pageRefsIn := make([]pdf.Reference, numPages)
	rm := pdf.NewResourceManager(doc)
	tree := pagetree.NewWriter(doc, rm)
	for i := 0; i < numPages; i++ {
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

func TestIterator(t *testing.T) {
	data, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

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
