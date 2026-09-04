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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/page"
	"seehuhn.de/go/pdf/pagetree"
)

func TestBalance(t *testing.T) {
	// write a test file
	buf := &bytes.Buffer{}
	out, err := pdf.NewWriter(buf, pdf.V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}
	rm := pdf.NewResourceManager(out)
	tree := pagetree.NewWriter(out, rm)
	for range 16 * 16 { // maxDegree = 16 -> this should give depth 2
		p := &page.Page{
			MediaBox: document.A4,
		}
		err := tree.AppendPage(p)
		if err != nil {
			t.Fatal(err)
		}
	}
	ref, err := tree.Close()
	if err != nil {
		t.Fatal(err)
	}
	out.GetMeta().Catalog.Pages = ref
	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = out.Close()
	if err != nil {
		t.Fatal(err)
	}
	testData := buf.Bytes()

	// read back the file and inspect the page tree
	readBuf := bytes.NewReader(testData)
	in, err := pdf.NewReader(readBuf, int64(len(testData)), nil)
	if err != nil {
		t.Fatal(err)
	}
	var walk func(pages pdf.Object, depth int) error
	walk = func(obj pdf.Object, depth int) error {
		node, err := pdf.NewCursor(in).Dict(obj)
		if err != nil {
			return err
		}
		switch node["Type"].(pdf.Name) {
		case "Pages":
			kids := node["Kids"].(pdf.Array)
			for _, kid := range kids {
				err = walk(kid, depth+1)
				if err != nil {
					return err
				}
			}
		case "Page":
			if depth != 2 {
				return fmt.Errorf("page at depth %d", depth)
			}
		}

		return nil
	}
	err = walk(in.GetMeta().Catalog.Pages, 0)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAppendPageWithoutMediaBox(t *testing.T) {
	// The page tree writer builds every parent node itself and never
	// supplies a MediaBox, so a page without one cannot be written.
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	tree := pagetree.NewWriter(w, rm)

	err := tree.AppendPage(&page.Page{Resources: &content.Resources{}})
	if err == nil {
		t.Error("AppendPage accepted a page without MediaBox")
	}

	err = tree.AppendPageDict(w.Alloc(), pdf.Dict{
		"Type":      pdf.Name("Page"),
		"Resources": pdf.Dict{},
	})
	if err == nil {
		t.Error("AppendPageDict accepted a page without MediaBox")
	}
}
