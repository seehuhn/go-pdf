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

// Package pages implements PDF page trees.
package pages

import (
	"errors"
	"fmt"

	"seehuhn.de/go/pdf"
)

// Tree represents a PDF page tree.
type Tree struct {
	Out *pdf.Writer

	parent *Tree
	attr   *InheritableAttributes

	// Children contains potentially incomplete subtrees of the current
	// tree, in page order.
	children []*Tree

	// Tail contains completed, partial subtrees of the current tree, in
	// page order.  The depth of the subtrees is weakly decreasing, and for
	// every depth there are at most maxDegree-1 subtrees of this depth.
	tail []*nodeInfo

	// OutObjects contains completed objects which still need to be written to
	// the PDF file.  OutRefs contains the PDF references for these objects.
	outObjects []pdf.Object
	outRefs    []*pdf.Reference

	isClosed bool
}

func (t *Tree) checkInvariants() {
	// TODO(voss): once things have settled, move this function into the test
	// suite.

	for _, child := range t.children {
		child.checkInvariants()
		if child.parent != t {
			panic("child.parent != t")
		}
	}

	var curDepth, numAtDepth int
	first := true
	for i, node := range t.tail {
		invalid := false
		if first || node.depth < curDepth {
			curDepth = node.depth
			numAtDepth = 1
		} else if node.depth > curDepth {
			invalid = true
		} else {
			numAtDepth++
			if numAtDepth > maxDegree {
				invalid = true
			}
		}
		if invalid {
			var dd []int
			for j := 0; j <= i; j++ {
				dd = append(dd, t.tail[j].depth)
			}
			panic(fmt.Sprintf("invalid depth seq %d", dd))
		}
	}

	if len(t.outObjects) != len(t.outRefs) {
		panic("len(outObjects) != len(outRefs)")
	}
}

// InstallTree installs a page tree as the root of the PDF document.
// The tree is automatically closed when the PDF document is closed,
// and a reference to the root node is written to the \Pages entry
// of the PDF document catalog (overwriting any previous value).
func InstallTree(w *pdf.Writer, attr *InheritableAttributes) *Tree {
	t := NewTree(w, attr)
	w.OnClose(func(w *pdf.Writer) error {
		ref, err := t.Close()
		if err != nil {
			return err
		}
		w.Catalog.Pages = ref
		return nil
	})
	return t
}

// NewTree creates a new page tree which adds pages to the PDF document w.
func NewTree(w *pdf.Writer, attr *InheritableAttributes) *Tree {
	t := &Tree{
		Out:  w,
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
	{
		var nodes []*nodeInfo
		var err error
		for _, child := range t.children {
			_, err = child.Close()
			if err != nil {
				return nil, err
			}
			nodes = t.merge(nodes, child.tail)
		}
		t.children = nil
		t.tail = t.merge(nodes, t.tail)
	}

	t.checkInvariants()

	if t.attr != nil || t.parent == nil {
		// reduce to one node, if needed
		for len(t.tail) > 1 {
			start := len(t.tail) - maxDegree
			if start < 0 {
				start = 0
			}
			for start > 0 && t.tail[start-1].depth == t.tail[start].depth {
				start++
			}
			t.tail = t.mergeNodes(t.tail, start, len(t.tail))
		}
	}

	if t.parent == nil && len(t.tail) > 0 { // be careful in case there are no pages
		t.tail[0].dictInfo = t.wrapIfNeeded(t.tail[0].dictInfo)
	}

	if t.attr != nil && len(t.tail) > 0 {
		mergeAttributes(t.tail[0].dict, t.attr)
	}

	if t.parent == nil {
		if len(t.tail) == 0 {
			return nil, errors.New("no pages in document")
		}
		rootRef := t.tail[0].ref
		t.outRefs = append(t.outRefs, t.tail[0].ref)
		t.outObjects = append(t.outObjects, t.tail[0].dict)
		return rootRef, t.flush()
	}

	t.parent.outRefs = append(t.parent.outRefs, t.outRefs...)
	t.parent.outObjects = append(t.parent.outObjects, t.outObjects...)
	t.outRefs = nil
	t.outObjects = nil
	if len(t.parent.outObjects) > objStreamChunkSize {
		err := t.parent.flush()
		if err != nil {
			return nil, err
		}
	}

	var rootRef *pdf.Reference
	if len(t.tail) == 1 {
		rootRef = t.tail[0].dictInfo.ref
	}
	return rootRef, nil
}

// AppendPage adds a new page to the page tree.
func (t *Tree) AppendPage(pageDict pdf.Dict) (*pdf.Reference, error) {
	if t.isClosed {
		return nil, errors.New("page tree is closed")
	}

	pageRef := t.Out.Alloc()
	node := &nodeInfo{
		dictInfo: &dictInfo{
			dict: pageDict,
			ref:  pageRef,
		},
		pageCount: 1,
		depth:     0,
	}
	t.tail = append(t.tail, node)

	for {
		n := len(t.tail)
		if n < maxDegree || t.tail[n-1].depth != t.tail[n-maxDegree].depth {
			break
		}
		t.tail = t.mergeNodes(t.tail, n-maxDegree, n)
	}
	t.checkInvariants()

	if len(t.outObjects) >= objStreamChunkSize {
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
func (t *Tree) NewSubTree(attr *InheritableAttributes) (*Tree, error) {
	if t.isClosed {
		return nil, errors.New("page tree is closed")
	}

	if len(t.tail) > 0 {
		before := &Tree{
			parent: t,
			Out:    t.Out,
			tail:   t.tail,
		}
		t.children = append(t.children, before)
		t.tail = nil
	}
	subTree := &Tree{
		parent: t,
		Out:    t.Out,
		attr:   attr,
	}
	t.children = append(t.children, subTree)
	return subTree, nil
}

// Flush writes a batch finished objects to the output file.
func (t *Tree) flush() error {
	if len(t.outObjects) == 0 {
		return nil
	}

	_, err := t.Out.WriteCompressed(t.outRefs, t.outObjects...)
	if err != nil {
		return err
	}

	t.outObjects = t.outObjects[:0]
	t.outRefs = t.outRefs[:0]
	return nil
}
