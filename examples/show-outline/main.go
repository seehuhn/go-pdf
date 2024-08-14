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

package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/outline"
	"seehuhn.de/go/pdf/pagetree"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: show-outline FILENAME")
	}
	fname := os.Args[1]

	doc, err := pdf.Open(fname, nil)
	if err != nil {
		log.Fatal(err)
	}

	tree, err := outline.Read(doc)
	if err != nil {
		log.Fatal(err)
	} else if tree == nil {
		fmt.Println("no document outline")
		return
	}

	// construct a map of page dictionary objects to page numbers
	pageNumbers := map[pdf.Reference]int{}
	pageNo := 0
	for ref := range pagetree.NewIterator(doc).All() {
		pageNumbers[ref] = pageNo
		pageNo++
	}

	// print the tree
	err = showTree(doc, pageNumbers, tree, "")
	if err != nil {
		log.Fatal(err)
	}

	err = doc.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func showTree(doc pdf.Getter, pageNumbers map[pdf.Reference]int, tree *outline.Tree, indent string) error {
	var pageLabel string
	s, err := pdf.GetName(doc, tree.Action["S"])
	if err != nil {
		return err
	}
	if s == "GoTo" {
		dest, err := pdf.Resolve(doc, tree.Action["D"])
		if err != nil {
			return err
		}
		switch dest := dest.(type) {
		case pdf.Array:
			if len(dest) > 0 {
				ref, ok := dest[0].(pdf.Reference)
				if ok {
					if n, ok := pageNumbers[ref]; ok {
						pageLabel = fmt.Sprintf(" page %d", n+1)
					}
				}
			}
		}
	}

	line := indent + tree.Title
	if pageLabel != "" {
		rep := max(70-len(line), 3)
		line += "  " + strings.Repeat(".", rep) + pageLabel
	}
	fmt.Println(line)
	for _, child := range tree.Children {
		err = showTree(doc, pageNumbers, child, indent+"  ")
		if err != nil {
			return err
		}
	}
	return nil
}
