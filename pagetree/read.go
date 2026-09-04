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
	"iter"
	"maps"
	"slices"

	"seehuhn.de/go/pdf"
)

// FindPages returns a list of all pages in the document.
// The returned list contains the references to the page dictionaries.
func FindPages(r pdf.Getter) ([]pdf.Reference, error) {
	c := pdf.NewCursor(r)
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

		node, err := pdf.Optional(c.Dict(ref))
		if err != nil {
			return nil, err
		}
		if node == nil {
			continue
		}
		tp, err := pdf.Optional(c.Name(node["Type"]))
		if err != nil {
			return nil, err
		}
		switch tp {
		case "Page":
			res = append(res, ref)
		case "Pages":
			kids, err := pdf.Optional(c.Array(node["Kids"]))
			if err != nil {
				return nil, err
			}
			if kids == nil {
				continue
			}
			for _, kid := range slices.Backward(kids) {
				if kidRef, ok := kid.(pdf.Reference); ok && !seen[kidRef] {
					todo = append(todo, kidRef)
					seen[kidRef] = true
				}
			}
		}
	}

	return res, nil
}

// Iterator iterates over the pages in a PDF document.
type Iterator struct {
	// Err holds any error encountered during iteration.
	// Check this after the loop completes.
	Err error

	r pdf.Getter
}

func NewIterator(r pdf.Getter) *Iterator {
	return &Iterator{r: r}
}

// All iterates over all pages in the document.
// Each iteration yields the page reference and a view of the page
// dictionary: inheritable attributes are copied in from parent nodes and
// the /Parent entry is removed.  The view is not suitable for writing back
// as the page object; to modify a page, read the stored dictionary via the
// reference instead.
func (i *Iterator) All() iter.Seq2[pdf.Reference, pdf.Dict] {
	yield := func(yield func(pdf.Reference, pdf.Dict) bool) {
		if i.Err != nil {
			return
		}

		r := i.r
		c := pdf.NewCursor(r)
		meta := r.GetMeta()
		root := meta.Catalog.Pages
		if root == 0 {
			return
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

			node, err := c.Dict(ref)
			if err != nil {
				if pdf.IsMalformed(err) {
					continue
				}
				i.Err = err
				return
			}
			tp, err := c.Name(node["Type"])
			if err != nil {
				if pdf.IsMalformed(err) {
					continue
				}
				i.Err = err
				return
			}
			switch tp {
			case "Page":
				if err := fillInherited(c, node, inherited, inheritable); err != nil {
					i.Err = err
					return
				}
				cont := yield(ref, node)
				if !cont {
					return
				}

			case "Pages":
				kids, err := c.Array(node["Kids"])
				if err != nil {
					if pdf.IsMalformed(err) {
						continue
					}
					i.Err = err
					return
				}

				var found []pdf.Name
				for _, name := range inheritable {
					ok, err := usable(c, name, node[name])
					if err != nil {
						i.Err = err
						return
					}
					if ok {
						found = append(found, name)
					}
				}
				if len(found) > 0 {
					if len(todo) > 0 {
						stack = append(stack, &frame{
							todo:      todo,
							inherited: maps.Clone(inherited),
						})
						todo = nil
					}
					for _, name := range found {
						inherited[name] = node[name]
					}
				}

				for _, kid := range slices.Backward(kids) {
					if kidRef, ok := kid.(pdf.Reference); ok && !seen[kidRef] {
						todo = append(todo, kidRef)
						seen[kidRef] = true
					}
				}
			}
		}
	}
	return yield
}

// fillInherited completes a page dictionary with the inheritable attributes
// in force from its ancestors, and removes the /Parent entry.  A page
// without a usable MediaBox anywhere in its ancestry is given the US
// Letter size, as in other readers.
func fillInherited(c pdf.Cursor, node, inherited pdf.Dict, inheritable []pdf.Name) error {
	for _, name := range inheritable {
		ok, err := usable(c, name, node[name])
		if err != nil {
			return err
		}
		if ok {
			continue
		}
		if val, canInherit := inherited[name]; canInherit {
			node[name] = val
		} else if name == "MediaBox" {
			node[name] = letterBox()
		}
	}
	delete(node, "Parent")
	return nil
}

// letterBox returns the MediaBox used for a page without a usable box.
func letterBox() pdf.Array {
	return pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(612), pdf.Integer(792)}
}

// usable reports whether val can serve as the value of the inheritable entry
// name.  A null value is equivalent to a missing entry.  A page box which is
// not a rectangle, or which has no area, also counts as missing, so that an
// ancestor's box can take its place.
func usable(c pdf.Cursor, name pdf.Name, val pdf.Object) (bool, error) {
	switch name {
	case "MediaBox", "CropBox":
		r, err := pdf.Optional(c.Rectangle(val))
		if err != nil {
			return false, err
		}
		return r != nil && r.Dx() > 0 && r.Dy() > 0, nil
	default:
		return val != nil, nil
	}
}

func getInheritable(v pdf.Version) []pdf.Name {
	if v < pdf.V1_3 {
		return inheritableOld
	}
	return inheritableNew
}

var (
	inheritableNew = []pdf.Name{"Resources", "MediaBox", "CropBox", "Rotate"}       // Since PDF 1.3
	inheritableOld = []pdf.Name{"Resources", "MediaBox", "CropBox", "Rotate", "AA"} // Before PDF 1.3
)
