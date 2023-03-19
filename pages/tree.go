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

	// absPageNo is the page number (0 based, from the start of the document)
	// of the next page to be added to the tree.  It is incremented
	// automatically when a page is added.
	absPageNo *futureInt

	// NumPagesCb is a list of callbacks which are called when the subtree
	// is closed, to report the total number of pages in the subtree.
	numPagesCb []func(int)

	// NextPageAbsCb is a list of callbacks which are called when the next page
	// is added to the tree, to report the absolute page number of the new page.
	nextPageAbsCb []func(int)
}

// InstallTree installs a page tree as the root of the PDF document.
// The tree is automatically closed when the PDF document is closed,
// and a reference to the root node is written to the \Pages entry
// of the PDF document catalog (overwriting any previous value).
//
// TODO(voss): remove?
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
		Out:       w,
		attr:      attr,
		absPageNo: &futureInt{},
	}
	return t
}

// Close closes the current tree and all subtrees.
// After a tree is closed, no more pages can be added to it.
// If the tree is the root of a page tree, the complete tree is written
// to the PDF file and a reference to the root node is returned.
// Otherwise, the returned reference is nil.
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
			if !child.isClosed {
				_, err = child.Close()
				if err != nil {
					return nil, err
				}
			}
			nodes = t.merge(nodes, child.tail)
		}
		t.children = nil
		t.tail = t.merge(nodes, t.tail)
	}

	t.checkInvariants()

	if len(t.numPagesCb) != 0 {
		numPages := 0
		for _, node := range t.tail {
			numPages += int(node.pageCount)
		}
		for _, fn := range t.numPagesCb {
			fn(numPages)
		}
	}
	for _, cb := range t.nextPageAbsCb {
		cb(-1)
	}

	if t.IsRoot() {
		t.collapse()
		if len(t.tail) == 0 {
			return nil, errors.New("no pages in document")
		}
		rootNode := t.tail[0]
		t.tail = nil

		// the root node cannot be a leaf
		rootNode.dictInfo = t.wrapIfLeaf(rootNode.dictInfo)

		if t.attr != nil {
			mergeAttributes(rootNode.dict, t.attr)
		}

		t.outRefs = append(t.outRefs, rootNode.ref)
		t.outObjects = append(t.outObjects, rootNode.dict)
		return rootNode.ref, t.flush()
	}

	// If we reach this point, we are in a subtree.

	if t.attr != nil && len(t.tail) > 0 {
		t.collapse()
		mergeAttributes(t.tail[0].dict, t.attr)
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
	return nil, nil
}

// AppendPage adds a new page to the page tree.
func (t *Tree) AppendPage(pageDict pdf.Dict, pageRef *pdf.Reference) (*pdf.Reference, error) {
	if t.isClosed {
		return nil, errors.New("page tree is closed")
	}

	if pageRef == nil {
		pageRef = t.Out.Alloc()
	}
	node := &nodeInfo{
		dictInfo: &dictInfo{
			dict: pageDict,
			ref:  pageRef,
		},
		pageCount: 1,
		depth:     0,
	}
	t.tail = append(t.tail, node)

	for _, fn := range t.nextPageAbsCb {
		t.absPageNo.WhenAvailable(fn)
	}
	t.nextPageAbsCb = t.nextPageAbsCb[:0]

	// increment the page numbers
	t.absPageNo = t.absPageNo.Inc()

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
		parent:    t,
		Out:       t.Out,
		attr:      attr,
		absPageNo: t.absPageNo,
	}
	t.absPageNo = &futureInt{numMissing: 2}
	subTree.absPageNo.WhenAvailable(t.absPageNo.AddMissing)
	subTree.numPagesCb = append(subTree.numPagesCb, t.absPageNo.AddMissing)

	t.children = append(t.children, subTree)
	return subTree, nil
}

// NextPageNumberAbs registers a callback that will be called when the next
// page number is known.  Page numbers are relative to the start of the
// document, starting at 0.
//
// The callback will be called with -1 if the page tree is closed before
// another page is added.
func (t *Tree) NextPageNumberAbs(cb func(int)) {
	if t.isClosed {
		// there will be no next page
		cb(-1)
		return
	}

	t.nextPageAbsCb = append(t.nextPageAbsCb, cb)
}

// wrapIfLeaf ensures that the given dictionary is a /Pages object.
// A wrapper /Pages object is created if necessary.
func (t *Tree) wrapIfLeaf(info *dictInfo) *dictInfo {
	if info.dict["Type"] == pdf.Name("Pages") {
		return info
	}

	wrapperRef := t.Out.Alloc()
	info.dict["Parent"] = wrapperRef
	t.outRefs = append(t.outRefs, info.ref)
	t.outObjects = append(t.outObjects, info.dict)

	wrapper := pdf.Dict{
		"Type":  pdf.Name("Pages"),
		"Count": pdf.Integer(1),
		"Kids":  pdf.Array{info.ref},
	}

	return &dictInfo{dict: wrapper, ref: wrapperRef}
}

// Collapse reduces the tail to (at most) one node.
func (t *Tree) collapse() {
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

func (t *Tree) IsRoot() bool {
	return t.parent == nil
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
