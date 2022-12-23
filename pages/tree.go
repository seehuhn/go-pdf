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

package pages

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
	tail     []*nodeInfo

	outRefs    []*pdf.Reference
	outObjects []pdf.Object

	isClosed bool
}

// NewTree creates a new page tree which adds pages to the PDF document w.
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
		nodes = t.merge(nodes, child.tail)
	}
	nodes = t.merge(nodes, t.tail)
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
			nodes = t.makeInternalNode(nodes, start, len(nodes))
		}

		if t.parent == nil && len(nodes) > 0 { // be careful in case there are no pages
			nodes[0].dictInfo = t.wrapIfNeeded(nodes[0].dictInfo)
		}

		if t.attr != nil {
			mergeAttributes(nodes[0].dict, t.attr)
		}
	}

	if t.parent == nil {
		t.tail = nil

		if len(nodes) == 0 {
			return nil, errors.New("no pages in document")
		}
		rootRef := nodes[0].ref
		t.outRefs = append(t.outRefs, nodes[0].ref)
		t.outObjects = append(t.outObjects, nodes[0].dict)
		return rootRef, t.flush()
	}

	t.parent.outRefs = append(t.parent.outRefs, t.outRefs...)
	t.parent.outObjects = append(t.parent.outObjects, t.outObjects...)
	t.outRefs = nil
	t.outObjects = nil
	if len(t.parent.outObjects) > objStreamChunkSize {
		err = t.parent.flush()
		if err != nil {
			return nil, err
		}
	}

	t.tail = nodes
	var rootRef *pdf.Reference
	if len(nodes) == 1 {
		rootRef = nodes[0].dictInfo.ref
	}
	return rootRef, nil
}

// AppendPage adds a new page to the page tree.
func (t *Tree) AppendPage(pageDict pdf.Dict) (*pdf.Reference, error) {
	if t.isClosed {
		return nil, errors.New("page tree is closed")
	}

	pageRef := t.w.Alloc()
	node := &nodeInfo{
		dictInfo: &dictInfo{
			dict: pageDict,
			ref:  pageRef,
		},
		count: 1,
		depth: 0,
	}
	t.tail = append(t.tail, node)

	for {
		n := len(t.tail)
		if n < maxDegree || t.tail[n-1].depth < t.tail[n-maxDegree].depth {
			break
		}
		if n > maxDegree && t.tail[n-maxDegree].depth+1 != t.tail[n-maxDegree-1].depth {
			panic("missed a collapse") // TODO(voss): remove
		}
		t.tail = t.makeInternalNode(t.tail, n-maxDegree, n)
	}

	if len(t.outObjects) > objStreamChunkSize {
		err := t.flush()
		if err != nil {
			return nil, err
		}
	}

	return pageRef, nil
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

func (t *Tree) flush() error {
	if len(t.outObjects) == 0 {
		return nil
	}

	_, err := t.w.WriteCompressed(t.outRefs, t.outObjects...)
	if err != nil {
		return err
	}

	t.outRefs = t.outRefs[:0]
	t.outObjects = t.outObjects[:0]
	return nil
}
