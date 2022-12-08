// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package pages2

import (
	"errors"

	"seehuhn.de/go/pdf"
)

// Tree represents a PDF page tree.
type Tree struct {
	parent *Tree
	w      *pdf.Writer
	attr   *InheritableAttributes

	children []*Tree

	// tail contains all \Page and \Pages objects which still need to be
	// written to the pdf file.  The depth of nodes in the list is decreasing,
	// with at most maxDegree-1 nodes for each depth.
	tail []*nodeInfo

	isClosed bool
}

// NewTree creates a new page tree.
func NewTree(w *pdf.Writer, attr *InheritableAttributes) *Tree {
	t := &Tree{
		w:    w,
		attr: attr,
	}
	return t
}

// Close closes the current tree and all subtrees.
// After a tree is closed, no more pages can be appended to it.
// If the tree is the root of a page tree, the complete tree is written
// to the PDF file and a reference to the root node is returned.
// In case of subtrees, if the tree has attributes or consists of a single
// page, a reference to the subtree-root is returned.  Otherwise, the
// returned reference is nil.
func (t *Tree) Close() (*pdf.Reference, error) {
	if t.isClosed {
		return nil, errors.New("page tree is closed")
	}
	t.isClosed = true

	// closed trees have no children, and all nodes are in tail
	var nodes []*nodeInfo
	var err error
	for _, child := range t.children {
		_, err = child.Close()
		if err != nil {
			return nil, err
		}
		nodes, err = t.merge(nodes, child.tail)
		if err != nil {
			return nil, err
		}
	}
	nodes, err = t.merge(nodes, t.tail)
	if err != nil {
		return nil, err
	}
	t.children = nil

	if t.attr != nil || t.parent == nil {
		// reduce to one node, if needed
		for len(nodes) > 1 {
			start := len(nodes) - maxDegree
			if start < 0 {
				start = 0
			}
			for start > 0 && nodes[start-1].depth == nodes[start].depth {
				start++
			}
			var err error
			nodes, err = t.makeInternalNode(nodes, start, len(nodes))
			if err != nil {
				return nil, err
			}
		}

		if t.parent == nil && len(nodes) > 0 { // be careful in case are no pages
			nodes[0].dictInfo, err = t.wrapIfNeeded(nodes[0].dictInfo)
			if err != nil {
				return nil, err
			}
		}

		if t.attr != nil {
			mergeAttributes(nodes[0].dict, t.attr)
		}
	}

	if t.parent == nil {
		if len(nodes) == 0 {
			return nil, errors.New("no pages in document")
		}
		t.tail = nil
		return t.w.Write(nodes[0].dict, nodes[0].ref)
	}

	t.tail = nodes
	var rootRef *pdf.Reference
	if len(nodes) == 1 {
		rootRef = nodes[0].dictInfo.ref
	}
	return rootRef, nil
}

// NewSubTree creates a new Tree, which inserts pages into the document at the
// position of the current end of the parent tree.  Pages added to the parent
// tree will be inserted after the pages in the sub-tree.
func (t *Tree) NewSubTree(attr *InheritableAttributes) *Tree {
	if len(t.tail) > 0 {
		before := &Tree{
			parent: t,
			w:      t.w,
			tail:   t.tail,
		}
		t.children = append(t.children, before)
		t.tail = nil
	}
	subTree := &Tree{
		parent: t,
		w:      t.w,
		attr:   attr,
	}
	t.children = append(t.children, subTree)
	return subTree
}
