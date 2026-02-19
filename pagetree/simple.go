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

package pagetree

import (
	"errors"
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
)

// NumPages returns the number of pages in the document.
//
// The value is obtained from the /Count attribute of the root page tree node.
// If the PDF file is malformed, the value may not be accurate.
func NumPages(r pdf.Getter) (int, error) {
	catalog := r.GetMeta().Catalog
	pageTreeNode, err := pdf.GetDict(r, catalog.Pages)
	if err != nil {
		return 0, err
	}

	count, err := pdf.GetInteger(r, pageTreeNode["Count"])
	if err != nil {
		return 0, err
	}

	if count < 0 || count > math.MaxInt32 {
		return 0, errInvalidPageTree
	}

	return int(count), nil
}

// GetPage returns the page dictionary for a given page number.
// Page numbers start at 0.
// Inheritable attributes are copied from the parent nodes.
// The /Parent attribute is removed from the returned dictionary.
func GetPage(r pdf.Getter, pageNo int) (pdf.Reference, pdf.Dict, error) {
	if pageNo < 0 {
		numPages, err := NumPages(r)
		if err == nil {
			return 0, nil, fmt.Errorf("page not found (valid page numbers are 0 to %d)", numPages-1)
		}
		return 0, nil, errors.New("page not found")
	}

	meta := r.GetMeta()
	inherited := pdf.Dict{}
	inheritable := getInheritable(meta.Version)

	skip := pdf.Integer(pageNo)

	catalog := meta.Catalog
	kids := pdf.Array{catalog.Pages}

	seen := map[pdf.Object]bool{}
	for len(kids) > 0 {
		obj := kids[0]
		kids = kids[1:]

		// load the page tree node
		ref, ok := obj.(pdf.Reference)
		if ok {
			if seen[ref] {
				return 0, nil, errInvalidPageTree
			}
			seen[ref] = true
		}
		pageTreeNode, err := pdf.GetDict(r, obj)
		if err != nil {
			return 0, nil, err
		}

		// traverse the tree
		tp, err := pdf.GetName(r, pageTreeNode["Type"])
		if err != nil {
			return 0, nil, err
		}
		switch tp {
		case "Page":
			if skip > 0 {
				skip--
				break
			}

			for _, name := range inheritable {
				if _, ok := pageTreeNode[name]; !ok {
					if val, ok := inherited[name]; ok {
						pageTreeNode[name] = val
					}
				}
			}
			delete(pageTreeNode, "Parent")
			return ref, pageTreeNode, nil

		case "Pages":
			count, err := pdf.GetInteger(r, pageTreeNode["Count"])
			if err != nil {
				return 0, nil, err
			}
			if count < 0 {
				return 0, nil, errInvalidPageTree
			} else if skip < count {
				for _, name := range inheritable {
					if tmp, ok := pageTreeNode[name]; ok {
						inherited[name] = tmp
					}
				}

				kids, err = pdf.GetArray(r, pageTreeNode["Kids"])
				if err != nil {
					return 0, nil, err
				}
			} else {
				// skip to next kid
				skip -= count
			}

		default:
			return 0, nil, errInvalidPageTree
		}
	}

	numPages, err := NumPages(r)
	if err == nil {
		return 0, nil, fmt.Errorf("page not found (valid page numbers are 0 to %d)", numPages-1)
	}
	return 0, nil, errors.New("page not found")
}

var errInvalidPageTree = errors.New("invalid page tree")
