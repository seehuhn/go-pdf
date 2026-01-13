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

// Package pagetree implements PDF page trees.
package pagetree

import (
	"errors"
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/page"
)

// Writer writes a page tree to a PDF file.
type Writer struct {
	Out *pdf.Writer
	RM  *pdf.ResourceManager

	isClosed bool
	err      error // sticky error from encoding

	parent *Writer

	// Children contains potentially incomplete subtrees of the current
	// tree, in page order.  This is only used until the tree is closed,
	// afterwards all nodes are in tail.
	children []*Writer

	// Tail contains completed, partial subtrees of the current tree, in
	// page order.  The depth of the subtrees is weakly decreasing, and for
	// every depth there are at most maxDegree-1 subtrees of this depth.
	tail []*nodeInfo

	// OutObjects contains completed objects which still need to be written to
	// the PDF file.  OutRefs contains the PDF references for these objects.
	outObjects []pdf.Object
	outRefs    []pdf.Reference

	// nextPageNumber is the page number (0 based, from the start of the
	// document) of the next page to be added to the tree.  This is incremented
	// automatically whenever a new page is added.
	nextPageNumber *futureInt

	// NextPageNumberCb is a list of callbacks which are called once the
	// absolute page number of the next page added is known. If the tree is
	// closed without another page having been added, the callbacks are called
	// with the argument -1.
	nextPageNumberCb []func(int)

	// NumPagesCb is a list of callbacks which are called when the subtree
	// is closed, to report the total number of pages in the subtree.
	numPagesCb []func(int)
}

// NewWriter creates a new page tree which adds pages to the PDF document w.
// The ResourceManager rm is used to encode page objects.
func NewWriter(w *pdf.Writer, rm *pdf.ResourceManager) *Writer {
	t := &Writer{
		Out:            w,
		RM:             rm,
		nextPageNumber: &futureInt{},
	}
	return t
}

// Close closes the current tree and all subtrees.
// After a tree has been closed, no more pages can be added.
//
// If the tree is the root of a page tree, the complete tree is written
// to the PDF file and a reference to the root node is returned.
// Otherwise, the returned reference is 0.
//
// TODO(voss): get rid of the Reference return value
func (w *Writer) Close() (pdf.Reference, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.isClosed {
		return 0, errors.New("page tree is closed")
	}
	w.isClosed = true

	// closed trees have no children, all nodes are in tail
	{
		var nodes []*nodeInfo
		var err error
		for _, child := range w.children {
			if !child.isClosed {
				_, err = child.Close()
				if err != nil {
					return 0, err
				}
			}
			nodes = w.merge(nodes, child.tail)
		}
		w.tail = w.merge(nodes, w.tail)
		w.children = nil
	}

	if w.err != nil {
		return 0, w.err
	}

	w.checkInvariants()

	if len(w.numPagesCb) != 0 {
		numPages := 0
		for _, node := range w.tail {
			numPages += int(node.pageCount)
		}
		for _, fn := range w.numPagesCb {
			fn(numPages)
		}
	}

	for _, cb := range w.nextPageNumberCb {
		cb(-1)
	}
	w.nextPageNumberCb = nil

	if w.parent == nil { // If we are at the root of the tree ...
		w.collapse()
		if w.err != nil {
			return 0, w.err
		}
		if len(w.tail) == 0 {
			return 0, errors.New("no pages in document")
		}
		rootNode := w.tail[0]
		w.tail = nil

		// the root node cannot be a leaf
		rootNode.dictInfo = w.wrapIfLeaf(rootNode)
		if w.err != nil {
			return 0, w.err
		}

		w.outRefs = append(w.outRefs, rootNode.ref)
		w.outObjects = append(w.outObjects, rootNode.dict)
		return rootNode.ref, w.flush()
	}

	// If we are in a subtree ...
	w.parent.outRefs = append(w.parent.outRefs, w.outRefs...)
	w.outRefs = nil
	w.parent.outObjects = append(w.parent.outObjects, w.outObjects...)
	w.outObjects = nil
	if len(w.parent.outObjects) >= objStreamChunkSize {
		// TODO(voss): strictly obey the chunk size?
		err := w.parent.flush()
		if err != nil {
			return 0, err
		}
	}
	return 0, nil
}

// AppendPage adds a new page to the page tree.
//
// This function takes ownership of the page object, and
// sets the Parent field before encoding the page to the PDF file.
func (w *Writer) AppendPage(p *page.Page) error {
	return w.AppendPageRef(w.Out.Alloc(), p)
}

// AppendPageRef adds a new page to the page tree, using the given reference
// for the page dictionary.
//
// This function takes ownership of the page object, and
// sets the Parent field before encoding the page to the PDF file.
func (w *Writer) AppendPageRef(ref pdf.Reference, p *page.Page) error {
	if w.isClosed {
		return errors.New("page tree is closed")
	}

	node := &nodeInfo{
		dictInfo: &dictInfo{
			ref: ref,
		},
		pendingPage: p,
		pageCount:   1,
		depth:       0,
	}
	w.tail = append(w.tail, node)

	for _, fn := range w.nextPageNumberCb {
		w.nextPageNumber.WhenAvailable(fn)
	}
	w.nextPageNumberCb = w.nextPageNumberCb[:0]

	// increment the page numbers
	w.nextPageNumber = w.nextPageNumber.Inc()

	for {
		n := len(w.tail)
		if n < maxDegree || w.tail[n-1].depth != w.tail[n-maxDegree].depth {
			break
		}
		w.tail = w.mergeNodes(w.tail, n-maxDegree, n)
	}
	w.checkInvariants()

	if len(w.outObjects) >= objStreamChunkSize {
		err := w.flush()
		if err != nil {
			return err
		}
	}

	return nil
}

// AppendPageDict adds a page to the page tree using a raw dictionary.
// This is useful for page copying operations where the page dictionary
// is already constructed.
//
// Unlike [AppendPageRef], this does not encode the page - it assumes
// the dict is already in the correct format.  The Parent field will be
// set automatically when the page tree is built.
func (w *Writer) AppendPageDict(ref pdf.Reference, dict pdf.Dict) error {
	if w.isClosed {
		return errors.New("page tree is closed")
	}

	node := &nodeInfo{
		dictInfo: &dictInfo{
			ref:  ref,
			dict: dict,
		},
		pageCount: 1,
		depth:     0,
	}
	w.tail = append(w.tail, node)

	for _, fn := range w.nextPageNumberCb {
		w.nextPageNumber.WhenAvailable(fn)
	}
	w.nextPageNumberCb = w.nextPageNumberCb[:0]

	w.nextPageNumber = w.nextPageNumber.Inc()

	for {
		n := len(w.tail)
		if n < maxDegree || w.tail[n-1].depth != w.tail[n-maxDegree].depth {
			break
		}
		w.tail = w.mergeNodes(w.tail, n-maxDegree, n)
	}
	w.checkInvariants()

	if len(w.outObjects) >= objStreamChunkSize {
		err := w.flush()
		if err != nil {
			return err
		}
	}

	return nil
}

// NewRange creates a new Writer that can insert pages into a PDF document at
// the current position at the time of the call. Pages added to the parent
// Writer will be inserted after the pages from the newly returned Writer.
func (w *Writer) NewRange() (*Writer, error) {
	if w.isClosed {
		return nil, errors.New("page tree is closed")
	}

	if len(w.tail) > 0 {
		before := &Writer{
			parent: w,
			Out:    w.Out,
			RM:     w.RM,
			tail:   w.tail,
		}
		// TODO(voss): should we close this child already here?
		w.children = append(w.children, before)
		w.tail = nil
	}
	subTree := &Writer{
		parent:         w,
		Out:            w.Out,
		RM:             w.RM,
		nextPageNumber: w.nextPageNumber,
	}
	w.nextPageNumber = &futureInt{numMissing: 2}
	subTree.nextPageNumber.WhenAvailable(w.nextPageNumber.Update)
	subTree.numPagesCb = append(subTree.numPagesCb, w.nextPageNumber.Update)

	w.children = append(w.children, subTree)
	return subTree, nil
}

// NextPageNumber registers a callback that will be called when the absolute
// page number of the next page to be added is known.  Page numbers are
// relative to the start of the document, starting at 0.
//
// The callback will be called with -1 if the page tree is closed before
// another page is added.
func (w *Writer) NextPageNumber(cb func(int)) {
	if w.isClosed {
		// there will be no next page
		cb(-1)
		return
	}

	w.nextPageNumberCb = append(w.nextPageNumberCb, cb)
}

// wrapIfLeaf ensures that the given node is a /Pages object.
// A wrapper /Pages object is created if necessary.
// For single-page documents, this encodes the pending page.
func (w *Writer) wrapIfLeaf(node *nodeInfo) *dictInfo {
	// Handle pending page first - we know it will be a Page needing a wrapper
	if node.pendingPage != nil {
		wrapperRef := w.Out.Alloc()
		node.pendingPage.Parent = wrapperRef
		if node.pendingPage.Resources == nil {
			node.pendingPage.Resources = &content.Resources{}
		}
		dict, err := node.pendingPage.Encode(w.RM)
		if err != nil {
			w.err = err
			return nil
		}
		node.dict = dict.(pdf.Dict)
		node.pendingPage = nil

		// Write the page and create wrapper
		w.outRefs = append(w.outRefs, node.ref)
		w.outObjects = append(w.outObjects, node.dict)

		wrapper := pdf.Dict{
			"Type":  pdf.Name("Pages"),
			"Count": pdf.Integer(1),
			"Kids":  pdf.Array{node.ref},
		}
		return &dictInfo{dict: wrapper, ref: wrapperRef}
	}

	// Already-encoded dict that's a Pages node - no wrapper needed
	if node.dict["Type"] == pdf.Name("Pages") {
		return node.dictInfo
	}

	// Already-encoded Page dict - needs wrapper
	wrapperRef := w.Out.Alloc()
	node.dict["Parent"] = wrapperRef
	w.outRefs = append(w.outRefs, node.ref)
	w.outObjects = append(w.outObjects, node.dict)

	wrapper := pdf.Dict{
		"Type":  pdf.Name("Pages"),
		"Count": pdf.Integer(1),
		"Kids":  pdf.Array{node.ref},
	}
	return &dictInfo{dict: wrapper, ref: wrapperRef}
}

// Collapse reduces the tail to (at most) one node.
func (w *Writer) collapse() {
	for len(w.tail) > 1 {
		start := len(w.tail) - maxDegree
		if start < 0 {
			start = 0
		}
		for start > 0 && w.tail[start-1].depth == w.tail[start].depth {
			start++
		}
		w.tail = w.mergeNodes(w.tail, start, len(w.tail))
	}
}

// Flush completes all pending writes to the output file.
func (w *Writer) flush() error {
	if len(w.outObjects) == 0 {
		return nil
	}

	err := w.Out.WriteCompressed(w.outRefs, w.outObjects...)
	if err != nil {
		return pdf.Wrap(err, "page tree nodes")
	}

	w.outObjects = w.outObjects[:0]
	w.outRefs = w.outRefs[:0]
	return nil
}

func (w *Writer) checkInvariants() {
	// TODO(voss): once things have settled, move this function into the test
	// suite.

	for _, child := range w.children {
		child.checkInvariants()
		if child.parent != w {
			panic("child.parent != t")
		}
	}

	var curDepth, numAtDepth int
	first := true
	for i, node := range w.tail {
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
		first = false
		if invalid {
			var dd []int
			for j := 0; j <= i; j++ {
				dd = append(dd, w.tail[j].depth)
			}
			panic(fmt.Sprintf("invalid depth seq %d", dd))
		}
	}

	if len(w.outObjects) != len(w.outRefs) {
		panic("len(outObjects) != len(outRefs)")
	}
}

const (
	maxDegree          = 16
	objStreamChunkSize = maxDegree + 1
)
