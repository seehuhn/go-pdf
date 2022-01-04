// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package pages

import (
	"errors"

	"seehuhn.de/go/pdf"
)

const pageTreeWidth = 12

// TODO(voss): allow to add subtrees which are still growing, e.g. for the toc

// PageTree represents a PDF page tree.
type PageTree struct {
	w        *pdf.Writer
	root     *internalNode
	current  *internalNode
	defaults *DefaultAttributes
}

// NewPageTree allocates a new PageTree object.
// Use .AddPage() to add pages to the tree.
// Use .Finish() to write the tree to the file and return the root object
// for inclusion in the document catalog.
func NewPageTree(w *pdf.Writer, defaults *DefaultAttributes) *PageTree {
	root := &internalNode{
		id: w.Alloc(),
	}
	return &PageTree{
		w:        w,
		root:     root,
		current:  root,
		defaults: defaults,
	}
}

// Finish flushes all internal /Pages notes to the file and returns
// the root of the page tree.  After .Finish() has been called, the
// page tree cannot be used any more.
func (tree *PageTree) Finish() (*pdf.Reference, error) {
	current := tree.current
	for current.parent != nil {
		obj := current.toObject()
		_, err := tree.w.Write(obj, current.id)
		if err != nil {
			return nil, err
		}
		current = current.parent
	}
	tree.current = nil
	tree.root = nil

	root := current.toObject()
	if def := tree.defaults; def != nil {
		if def.Resources != nil {
			root["Resources"] = pdf.AsDict(def.Resources)
		}
		if def.MediaBox != nil {
			root["MediaBox"] = def.MediaBox
		}
		if def.CropBox != nil {
			root["CropBox"] = def.CropBox
		}
		if def.Rotate != 0 {
			root["Rotate"] = pdf.Integer(def.Rotate)
		}
	}
	return tree.w.Write(root, current.id)
}

// Ship adds a new page or subtree to the PageTree. This function is for
// special cases, where the caller constructs the page dictionary manually.
// Normally callers would use the .AddPage() method, instead.
func (tree *PageTree) Ship(page pdf.Dict) error {
	if page["Type"] != pdf.Name("Page") {
		return errors.New("wrong pdf.Dict type, expected /Page")
	}

	parent, err := tree.splitIfNeeded(tree.current)
	if err != nil {
		return err
	}
	tree.current = parent
	page["Parent"] = parent.id

	ref, err := tree.w.Write(page, nil)
	if err != nil {
		return err
	}

	inc := 1
	if cumulative, ok := page["Count"].(pdf.Integer); ok {
		inc = int(cumulative)
	}
	parent.kids = append(parent.kids, ref)
	for parent != nil {
		parent.count += inc
		parent = parent.parent
	}

	return nil
}

func (tree *PageTree) splitIfNeeded(node *internalNode) (*internalNode, error) {
	if len(node.kids) < pageTreeWidth {
		return node, nil
	}

	// Node is full: write it to disk and get a new one.

	// First check that there is a parent.
	parent := node.parent
	if parent == nil {
		// tree is full: add another level at the root
		parent = &internalNode{
			id:    tree.w.Alloc(),
			kids:  []*pdf.Reference{node.id},
			count: node.count,
		}
		node.parent = parent
		tree.root = parent
	}

	// Turn the node into a PDF object and write this to the file.
	nodeObj := node.toObject()
	_, err := tree.w.Write(nodeObj, node.id)
	if err != nil {
		return nil, err
	}

	parent, err = tree.splitIfNeeded(parent)
	if err != nil {
		return nil, err
	}
	node = &internalNode{
		id:     tree.w.Alloc(),
		parent: parent,
	}
	parent.kids = append(parent.kids, node.id)
	return node, nil
}

type internalNode struct {
	id     *pdf.Reference
	parent *internalNode
	kids   []*pdf.Reference
	count  int
}

func (pp *internalNode) toObject() pdf.Dict {
	var kids pdf.Array
	for _, ref := range pp.kids {
		kids = append(kids, ref)
	}
	nodeDict := pdf.Dict{ // page 76
		"Type":  pdf.Name("Pages"),
		"Kids":  kids,
		"Count": pdf.Integer(pp.count),
	}
	if pp.parent != nil {
		nodeDict["Parent"] = pp.parent.id
	}
	return nodeDict
}
