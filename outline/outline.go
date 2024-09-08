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

package outline

import (
	"errors"
	"fmt"

	"seehuhn.de/go/pdf"
)

type Tree struct {
	Title    string
	Children []*Tree
	Open     bool
	Action   pdf.Dict
}

func (tree *Tree) AddChild(title string) *Tree {
	child := &Tree{
		Title: title,
	}
	tree.Children = append(tree.Children, child)
	return child
}

func Read(r *pdf.Reader) (*Tree, error) {
	root := r.GetMeta().Catalog.Outlines
	if root == 0 {
		return nil, nil
	}

	seen := map[pdf.Reference]bool{}
	tree, _, err := readNode(r, seen, root)
	if err != nil {
		return nil, err
	}
	tree.Open = false // TODO(voss): is this right?
	return tree, nil
}

func readNode(r *pdf.Reader, seen map[pdf.Reference]bool, node pdf.Reference) (*Tree, pdf.Dict, error) {
	if seen[node] {
		return nil, nil, fmt.Errorf("outline tree contains a loop")
	}
	seen[node] = true
	if len(seen) > 65536 {
		return nil, nil, errors.New("outline too large")
	}

	dict, err := pdf.GetDictTyped(r, node, "Outlines")
	if err != nil {
		return nil, nil, err
	}

	tree := &Tree{}

	title, err := pdf.GetTextString(r, dict["Title"])
	if err != nil {
		return nil, nil, pdf.Wrap(err, "/Title in outline")
	}
	tree.Title = string(title)

	count, _ := dict["Count"].(pdf.Integer)
	tree.Open = count > 0

	if dest, _ := pdf.Resolve(r, dict["Dest"]); dest != nil {
		tree.Action = pdf.Dict{
			"S": pdf.Name("GoTo"),
			"D": dest,
		}
	} else if a, _ := pdf.GetDict(r, dict["A"]); a != nil {
		tree.Action = a
	}

	ccPtr, _ := dict["First"].(pdf.Reference)
	cc, err := readChildren(r, seen, ccPtr)
	if err != nil {
		return nil, nil, err
	}
	tree.Children = cc

	return tree, dict, nil
}

func readChildren(r *pdf.Reader, seen map[pdf.Reference]bool, node pdf.Reference) ([]*Tree, error) {
	var res []*Tree
	for node != 0 {
		tree, dict, err := readNode(r, seen, node)
		if err != nil {
			return nil, err
		}

		res = append(res, tree)

		node, _ = dict["Next"].(pdf.Reference)
	}
	return res, nil
}

// Write writes the outline tree to the PDF file and installs it in the catalog.
func (tree *Tree) Write(w pdf.Putter) error {
	if tree == nil {
		return nil
	}

	ww := &writer{
		out:   w,
		root:  tree,
		count: map[*Tree]int{},
	}
	ww.getCount(tree)
	if !ww.hasOpen {
		delete(ww.count, tree)
	}

	rootRef := w.Alloc()
	rootDict := pdf.Dict{}
	err := ww.writeNode(rootRef, rootDict, tree)
	if err != nil {
		return err
	}

	err = ww.flush()
	if err != nil {
		return err
	}

	w.GetMeta().Catalog.Outlines = rootRef
	return nil
}

type writer struct {
	out     pdf.Putter
	root    *Tree
	refs    []pdf.Reference
	objs    []pdf.Object
	count   map[*Tree]int
	hasOpen bool
}

func (ww *writer) getCount(tree *Tree) int {
	if tree == nil {
		return 0
	}
	total := len(tree.Children)
	for _, child := range tree.Children {
		cCount := ww.getCount(child)
		if cCount > 0 {
			total += cCount
		}
	}
	if !tree.Open && tree != ww.root {
		total = -total
	} else {
		ww.hasOpen = true
	}
	if total != 0 && (ww.hasOpen || tree != ww.root) {
		ww.count[tree] = total
	}
	return total
}

func (ww *writer) writeNode(ref pdf.Reference, dict pdf.Dict, tree *Tree) error {
	if tree.Title != "" && tree != ww.root {
		dict["Title"] = pdf.TextString(tree.Title)
	}
	if tree.Action != nil && tree != ww.root {
		if tree.Action["S"] == pdf.Name("GoTo") {
			dict["Dest"] = tree.Action["D"]
		} else {
			dict["A"] = tree.Action
		}
	}

	var first, last pdf.Reference
	if len(tree.Children) > 0 {
		first = ww.out.Alloc()
		if len(tree.Children) > 1 {
			last = ww.out.Alloc()
		} else {
			last = first
		}
		dict["First"] = first
		dict["Last"] = last
		dict["Count"] = pdf.Integer(ww.count[tree])
	}

	ww.objs = append(ww.objs, dict)
	ww.refs = append(ww.refs, ref)
	if len(ww.objs) >= chunkSize {
		err := ww.flush()
		if err != nil {
			return err
		}
	}

	if len(tree.Children) > 0 {
		err := ww.writeChildren(ref, first, last, tree.Children)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ww *writer) writeChildren(parent, first, last pdf.Reference, cc []*Tree) error {
	ccRef := make([]pdf.Reference, len(cc))
	for i := range cc {
		if i == 0 {
			ccRef[i] = first
		} else if i == len(cc)-1 {
			ccRef[i] = last
		} else {
			ccRef[i] = ww.out.Alloc()
		}
	}

	for i, tree := range cc {
		dict := pdf.Dict{
			"Parent": parent,
		}
		if i > 0 {
			dict["Prev"] = ccRef[i-1]
		}
		if i < len(cc)-1 {
			dict["Next"] = ccRef[i+1]
		}
		err := ww.writeNode(ccRef[i], dict, tree)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ww *writer) flush() error {
	if len(ww.objs) == 0 {
		return nil
	}
	err := ww.out.WriteCompressed(ww.refs, ww.objs...)
	if err != nil {
		return pdf.Wrap(err, "document outline tree nodes")
	}
	ww.refs = ww.refs[:0]
	ww.objs = ww.objs[:0]
	return nil
}

const chunkSize = 16
