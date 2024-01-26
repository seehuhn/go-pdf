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
	"maps"

	"seehuhn.de/go/pdf"
)

// FindPages returns a list of all pages in the document.
// The returned list contains the references to the page dictionaries.
func FindPages(r pdf.Getter) ([]pdf.Reference, error) {
	meta := r.GetMeta()
	catalog := meta.Catalog
	if catalog.Pages == 0 {
		return nil, errInvalidPageTree
	}

	var res []pdf.Reference
	todo := []pdf.Reference{catalog.Pages}
	seen := map[pdf.Reference]bool{
		catalog.Pages: true,
	}
	for len(todo) > 0 {
		k := len(todo) - 1
		ref := todo[k]
		todo = todo[:k]

		node, err := pdf.GetDict(r, ref)
		if err != nil {
			return nil, err
		}
		tp, err := pdf.GetName(r, node["Type"])
		if err != nil {
			return nil, err
		}
		switch tp {
		case "Page":
			res = append(res, ref)
		case "Pages":
			kids, err := pdf.GetArray(r, node["Kids"])
			if err != nil {
				return nil, err
			}
			for i := len(kids) - 1; i >= 0; i-- {
				kid := kids[i]
				if kidRef, ok := kid.(pdf.Reference); ok && !seen[kidRef] {
					todo = append(todo, kidRef)
					seen[kidRef] = true
				} else {
					res = append(res, 0)
				}
			}
		}
	}

	return res, nil
}

type Iterator struct {
	Err error
	r   pdf.Getter
}

func NewIterator(r pdf.Getter) *Iterator {
	return &Iterator{r: r}
}

// All returns a function which iterates over all pages in the document.
// The arguments are the reference to the page dictionary and the page
// dictionary itself.
//
// The function must return true if the iteration should continue, or false if it
// should stop.
//
// TODO(voss): change this to iterate over (page number, page dictionary) pairs?
func (i *Iterator) All() func(yield func(pdf.Reference, pdf.Dict) bool) bool {
	yield := func(yield func(pdf.Reference, pdf.Dict) bool) bool {
		if i.Err != nil {
			return false
		}

		r := i.r
		meta := r.GetMeta()
		root := meta.Catalog.Pages
		if root == 0 {
			return true
		}

		type frame struct {
			todo      []pdf.Reference
			inherited pdf.Dict
		}
		var stack []*frame
		todo := []pdf.Reference{root}
		inherited := pdf.Dict{}
		inheritable := getInheritable(meta.Version)

		seen := map[pdf.Reference]bool{
			root: true,
		}
		for len(todo) > 0 || len(stack) > 0 {
			if len(todo) == 0 {
				k := len(stack) - 1
				frame := stack[k]
				stack = stack[:k]
				todo = frame.todo
				inherited = frame.inherited
			}

			k := len(todo) - 1
			ref := todo[k]
			todo = todo[:k]

			node, err := pdf.GetDict(r, ref)
			if err != nil {
				if pdf.IsMalformed(err) {
					continue
				}
				i.Err = err
				return false
			}
			tp, err := pdf.GetName(r, node["Type"])
			if err != nil {
				if pdf.IsMalformed(err) {
					continue
				}
				i.Err = err
				return false
			}
			switch tp {
			case "Page":
				for _, name := range inheritable {
					_, isPresent := node[name]
					if val, canInherit := inherited[name]; !isPresent && canInherit {
						node[name] = val
					}
				}
				delete(node, "Parent")
				cont := yield(ref, node)
				if !cont {
					return false
				}

			case "Pages":
				kids, err := pdf.GetArray(r, node["Kids"])
				if err != nil {
					if pdf.IsMalformed(err) {
						continue
					}
					i.Err = err
					return false
				}

				hasInheritables := false
				for _, name := range inheritable {
					if _, isPresent := node[name]; isPresent {
						hasInheritables = true
						break
					}
				}
				if hasInheritables {
					if len(todo) > 0 {
						stack = append(stack, &frame{
							todo:      todo,
							inherited: maps.Clone(inherited),
						})
						todo = nil
					}
					for _, name := range inheritable {
						if tmp, ok := node[name]; ok {
							inherited[name] = tmp
						}
					}
				}

				for i := len(kids) - 1; i >= 0; i-- {
					kid := kids[i]
					if kidRef, ok := kid.(pdf.Reference); ok && !seen[kidRef] {
						todo = append(todo, kidRef)
						seen[kidRef] = true
					}
				}
			}
		}
		return true
	}
	return yield
}

func getInheritable(v pdf.Version) []pdf.Name {
	if v < pdf.V1_3 {
		return inheritedOld
	}
	return inheritableNew
}

var (
	inheritableNew = []pdf.Name{"Resources", "MediaBox", "CropBox", "Rotate"}       // Since PDF 1.3
	inheritedOld   = []pdf.Name{"Resources", "MediaBox", "CropBox", "Rotate", "AA"} // Before PDF 1.3
)
