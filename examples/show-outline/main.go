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
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/destination"
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
	for _, item := range tree.Items {
		err = showItem(pageNumbers, item, "")
		if err != nil {
			log.Fatal(err)
		}
	}

	err = doc.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func showItem(pageNumbers map[pdf.Reference]int, item *outline.Item, indent string) error {
	var pageLabel string

	// get page number from destination or GoTo action
	var dest destination.Destination
	if item.Destination != nil {
		dest = item.Destination
	} else if goTo, ok := item.Action.(*action.GoTo); ok {
		dest = goTo.Dest
	}

	if dest != nil {
		if ref := getPageRef(dest); ref != 0 {
			if n, ok := pageNumbers[ref]; ok {
				pageLabel = fmt.Sprintf(" page %d", n+1)
			}
		}
	}

	line := indent + item.Title
	if pageLabel != "" {
		rep := max(70-len(line), 3)
		line += "  " + strings.Repeat(".", rep) + pageLabel
	}
	fmt.Println(line)
	for _, child := range item.Children {
		err := showItem(pageNumbers, child, indent+"  ")
		if err != nil {
			return err
		}
	}
	return nil
}

// getPageRef extracts the page reference from a destination.
func getPageRef(dest destination.Destination) pdf.Reference {
	switch d := dest.(type) {
	case *destination.XYZ:
		if ref, ok := d.Page.(pdf.Reference); ok {
			return ref
		}
	case *destination.Fit:
		if ref, ok := d.Page.(pdf.Reference); ok {
			return ref
		}
	case *destination.FitH:
		if ref, ok := d.Page.(pdf.Reference); ok {
			return ref
		}
	case *destination.FitV:
		if ref, ok := d.Page.(pdf.Reference); ok {
			return ref
		}
	case *destination.FitR:
		if ref, ok := d.Page.(pdf.Reference); ok {
			return ref
		}
	case *destination.FitB:
		if ref, ok := d.Page.(pdf.Reference); ok {
			return ref
		}
	case *destination.FitBH:
		if ref, ok := d.Page.(pdf.Reference); ok {
			return ref
		}
	case *destination.FitBV:
		if ref, ok := d.Page.(pdf.Reference); ok {
			return ref
		}
	}
	return 0
}
