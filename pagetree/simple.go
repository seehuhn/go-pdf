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

// GetPage returns the page tree node for a given page number.
// Page numbers start at 0.
// Inheritable attributes are copied from the parent nodes.
// The /Parent attribute is removed from the returned dictionary.
func GetPage(r pdf.Getter, pageNo int) (pdf.Dict, error) {
	if pageNo < 0 {
		return nil, errors.New("invalid page number")
	}

	meta := r.GetMeta()
	inherited := pdf.Dict{}
	inheritable := getInheritable(meta.Version)

	skip := pdf.Integer(pageNo)

	catalog := meta.Catalog
	kids := pdf.Array{catalog.Pages}

	seen := map[pdf.Object]bool{}
	for len(kids) > 0 {
		ref := kids[0]
		kids = kids[1:]

		// load the page tree node
		if r, ok := ref.(pdf.Reference); ok {
			if seen[r] {
				return nil, errInvalidPageTree
			}
			seen[r] = true
		}
		pageTreeNode, err := pdf.GetDict(r, ref)
		if err != nil {
			return nil, err
		}

		// traverse the tree
		tp, err := pdf.GetName(r, pageTreeNode["Type"])
		if err != nil {
			return nil, err
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
			delete(pageTreeNode, "Parent") // TODO(voss): why are we doing this?
			return pageTreeNode, nil

		case "Pages":
			count, err := pdf.GetInteger(r, pageTreeNode["Count"])
			if err != nil {
				return nil, err
			}
			if count < 0 {
				return nil, errInvalidPageTree
			} else if skip < count {
				for _, name := range inheritable {
					if tmp, ok := pageTreeNode[name]; ok {
						inherited[name] = tmp
					}
				}

				kids, err = pdf.GetArray(r, pageTreeNode["Kids"])
				if err != nil {
					return nil, err
				}
			} else {
				// skip to next kid
				skip -= count
			}

		default:
			return nil, errInvalidPageTree
		}
	}

	numPages, err := NumPages(r)
	if err == nil {
		return nil, fmt.Errorf("page not found (valid page numbers are 0 to %d)", numPages-1)
	}
	return nil, errors.New("page not found")
}

var errInvalidPageTree = errors.New("invalid page tree")
