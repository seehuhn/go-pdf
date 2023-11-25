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
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pagetree"
)

func TestFindPages(t *testing.T) {
	doc := pdf.NewData(pdf.V1_7)

	numPages := 234
	pageRefsIn := make([]pdf.Reference, numPages)
	tree := pagetree.NewWriter(doc)
	for i := 0; i < numPages; i++ {
		pageRefsIn[i] = doc.Alloc()
		pageDict := pdf.Dict{
			"Type": pdf.Name("Page"),
		}
		doc.Put(pageRefsIn[i], pageDict)
		err := tree.AppendPageRef(pageRefsIn[i], pageDict)
		if err != nil {
			t.Fatal(err)
		}
	}
	treeRef, err := tree.Close()
	if err != nil {
		t.Fatal(err)
	}
	doc.GetMeta().Catalog.Pages = treeRef

	pageRefsOut, err := pagetree.FindPages(doc)
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(pageRefsIn, pageRefsOut); d != "" {
		t.Fatalf("unexpected pageRefs (-want +got):\n%s", d)
	}
}
