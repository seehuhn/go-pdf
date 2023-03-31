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
	"seehuhn.de/go/pdf/pagetree"
)

func TestBalance(t *testing.T) {
	// write a test file
	buf := &bytes.Buffer{}
	out, err := pdf.NewWriter(buf, nil)
	if err != nil {
		t.Fatal(err)
	}
	tree := pagetree.NewWriter(out, &pagetree.InheritableAttributes{
		MediaBox: pagetree.A4,
	})
	for i := 0; i < 16*16; i++ { // maxDegree = 16 -> this should give depth 2
		dict := pdf.Dict{
			"Type": pdf.Name("Page"),
		}
		_, err := tree.AppendPage(dict, 0)
		if err != nil {
			t.Fatal(err)
		}
	}
	ref, err := tree.Close()
	if err != nil {
		t.Fatal(err)
	}
	out.Catalog.Pages = ref
	err = out.Close()
	if err != nil {
		t.Fatal(err)
	}
	testData := buf.Bytes()

	// read back the file and inspect the page tree
	readBuf := bytes.NewReader(testData)
	in, err := pdf.NewReader(readBuf, nil)
	if err != nil {
		t.Fatal(err)
	}
	var walk func(pages pdf.Object, depth int) error
	walk = func(obj pdf.Object, depth int) error {
		node, err := in.GetDict(obj)
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
	err = walk(in.Catalog.Pages, 0)
	if err != nil {
		t.Fatal(err)
	}
}
