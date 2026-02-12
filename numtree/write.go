// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package numtree

import (
	"errors"
	"iter"

	"seehuhn.de/go/pdf"
)

// maxChildren is the maximum number of children we allow in a node while writing a number tree.
const maxChildren = 64

type entry struct {
	key   pdf.Integer
	value pdf.Object
}

type nodeInfo struct {
	ref       pdf.Reference
	depth     int
	minKey    pdf.Integer
	maxKey    pdf.Integer
	nodeCount int // number of child nodes (for intermediate) or entries (for leaf)
}

type numberTreeWriter struct {
	w           *pdf.Writer
	tail        []*nodeInfo // completed nodes at various depths
	pendingLeaf []entry     // accumulating entries for current leaf
	lastKey     pdf.Integer // for sort order validation
	hasEntries  bool        // track if we've seen any entries
}

// Write creates a number tree in the PDF file.
// The iterator `data` provides the key-value pairs.  The keys must be
// returned in sorted order, and must not contain duplicates.
// The return value is the reference to the number tree root.
func Write(w *pdf.Writer, data iter.Seq2[pdf.Integer, pdf.Object]) (pdf.Reference, error) {
	writer := &numberTreeWriter{
		w: w,
	}

	for key, value := range data {
		err := writer.addEntry(key, value)
		if err != nil {
			return 0, err
		}
	}

	return writer.finish()
}

func (w *numberTreeWriter) addEntry(key pdf.Integer, value pdf.Object) error {
	// validate sort order
	if w.hasEntries && key <= w.lastKey {
		return errors.New("keys must be in sorted order")
	}
	w.lastKey = key
	w.hasEntries = true

	// add to pending leaf
	w.pendingLeaf = append(w.pendingLeaf, entry{key: key, value: value})

	// if leaf is full, complete it
	if len(w.pendingLeaf) >= maxChildren {
		err := w.completePendingLeaf()
		if err != nil {
			return err
		}
	}

	return nil
}

func (w *numberTreeWriter) completePendingLeaf() error {
	if len(w.pendingLeaf) == 0 {
		return nil
	}

	// create leaf node
	ref := w.w.Alloc()

	var nums []pdf.Object
	for _, e := range w.pendingLeaf {
		nums = append(nums, e.key)
		nums = append(nums, e.value)
	}

	node := pdf.Dict{
		"Nums": pdf.Array(nums),
		"Limits": pdf.Array{
			w.pendingLeaf[0].key,
			w.pendingLeaf[len(w.pendingLeaf)-1].key,
		},
	}

	err := w.w.Put(ref, node)
	if err != nil {
		return err
	}

	// add to tail
	nodeInfo := &nodeInfo{
		ref:       ref,
		depth:     0,
		minKey:    w.pendingLeaf[0].key,
		maxKey:    w.pendingLeaf[len(w.pendingLeaf)-1].key,
		nodeCount: len(w.pendingLeaf),
	}
	w.tail = append(w.tail, nodeInfo)
	w.pendingLeaf = nil

	// apply merging logic (like pagetree)
	return w.mergeTail()
}

func (w *numberTreeWriter) mergeTail() error {
	for {
		n := len(w.tail)
		if n < maxChildren || w.tail[n-1].depth != w.tail[n-maxChildren].depth {
			break
		}
		err := w.mergeNodes(n-maxChildren, n)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *numberTreeWriter) mergeNodes(start, end int) error {
	if start >= end {
		return nil
	}

	children := w.tail[start:end]
	ref := w.w.Alloc()

	// build Kids array
	var kids []pdf.Object
	for _, child := range children {
		kids = append(kids, child.ref)
	}

	// create intermediate node
	node := pdf.Dict{
		"Kids": pdf.Array(kids),
		"Limits": pdf.Array{
			children[0].minKey,
			children[len(children)-1].maxKey,
		},
	}

	err := w.w.Put(ref, node)
	if err != nil {
		return err
	}

	// replace children with merged node
	merged := &nodeInfo{
		ref:       ref,
		depth:     children[0].depth + 1,
		minKey:    children[0].minKey,
		maxKey:    children[len(children)-1].maxKey,
		nodeCount: len(children),
	}

	// replace the range with merged node
	w.tail = append(w.tail[:start], append([]*nodeInfo{merged}, w.tail[end:]...)...)

	return nil
}

func (w *numberTreeWriter) finish() (pdf.Reference, error) {
	// complete any pending leaf
	if len(w.pendingLeaf) > 0 {
		// special case: if this is the only leaf, make it a root with Nums (no Limits)
		if len(w.tail) == 0 {
			return w.writeRootWithNums()
		}
		err := w.completePendingLeaf()
		if err != nil {
			return 0, err
		}
	}

	// handle empty tree
	if len(w.tail) == 0 {
		return 0, nil
	}

	// special case: single leaf becomes root with Nums (no Limits)
	if len(w.tail) == 1 && w.tail[0].depth == 0 {
		return w.writeRootFromSingleLeaf(w.tail[0])
	}

	// collapse to single root
	err := w.collapse()
	if err != nil {
		return 0, err
	}

	if len(w.tail) != 1 {
		return 0, errors.New("failed to collapse to single root")
	}

	root := w.tail[0]

	// root with Kids should have no Limits per PDF spec
	if root.depth > 0 {
		return w.writeRootWithKids(root.ref)
	}

	return root.ref, nil
}

func (w *numberTreeWriter) writeRootWithNums() (pdf.Reference, error) {
	ref := w.w.Alloc()

	var nums []pdf.Object
	for _, e := range w.pendingLeaf {
		nums = append(nums, e.key)
		nums = append(nums, e.value)
	}

	// root node with Nums has no Limits
	node := pdf.Dict{
		"Nums": pdf.Array(nums),
	}

	err := w.w.Put(ref, node)
	return ref, err
}

func (w *numberTreeWriter) writeRootFromSingleLeaf(leaf *nodeInfo) (pdf.Reference, error) {
	// we need to rewrite the leaf as a root (without Limits)
	// since we can't modify already written objects, create new root
	ref := w.w.Alloc()
	node := pdf.Dict{
		"Kids": pdf.Array{leaf.ref},
	}
	err := w.w.Put(ref, node)
	return ref, err
}

func (w *numberTreeWriter) writeRootWithKids(kidRef pdf.Reference) (pdf.Reference, error) {
	ref := w.w.Alloc()
	node := pdf.Dict{
		"Kids": pdf.Array{kidRef},
	}
	err := w.w.Put(ref, node)
	return ref, err
}

func (w *numberTreeWriter) collapse() error {
	for len(w.tail) > 1 {
		start := max(len(w.tail)-maxChildren, 0)
		// find start of same depth group
		for start > 0 && w.tail[start-1].depth == w.tail[start].depth {
			start--
		}
		// find end of same depth group
		end := start + 1
		for end < len(w.tail) && w.tail[end].depth == w.tail[start].depth {
			end++
		}
		err := w.mergeNodes(start, end)
		if err != nil {
			return err
		}
	}
	return nil
}
