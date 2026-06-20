// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package pdftree

import (
	"cmp"
	"errors"
	"iter"
	"slices"

	"seehuhn.de/go/pdf"
)

// maxChildren is the maximum number of children we allow in a node while
// writing a tree.
const maxChildren = 64

type entry[K cmp.Ordered] struct {
	key   K
	value pdf.Object
}

type nodeInfo[K cmp.Ordered] struct {
	ref       pdf.Reference
	depth     int
	minKey    K
	maxKey    K
	nodeCount int // number of child nodes (for intermediate) or entries (for leaf)
}

type treeWriter[K cmp.Ordered, C codec[K]] struct {
	w           *pdf.Writer
	tail        []*nodeInfo[K] // completed nodes at various depths
	pendingLeaf []entry[K]     // accumulating entries for current leaf
	lastKey     K              // for sort order validation
	hasEntries  bool           // track if we've seen any entries
}

// Write creates a tree in the PDF file.
// The iterator data provides the key-value pairs.  The keys must be returned in
// sorted order, and must not contain duplicates.
// The return value is the reference to the tree root.  An empty sequence
// produces the null reference rather than a tree object.
func Write[K cmp.Ordered, C codec[K]](w *pdf.Writer, data iter.Seq2[K, pdf.Object]) (pdf.Reference, error) {
	writer := &treeWriter[K, C]{
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

func (w *treeWriter[K, C]) addEntry(key K, value pdf.Object) error {
	// validate sort order
	if w.hasEntries && key <= w.lastKey {
		return errors.New("keys must be in sorted order")
	}
	w.lastKey = key
	w.hasEntries = true

	// add to pending leaf
	w.pendingLeaf = append(w.pendingLeaf, entry[K]{key: key, value: value})

	// if leaf is full, complete it
	if len(w.pendingLeaf) >= maxChildren {
		err := w.completePendingLeaf()
		if err != nil {
			return err
		}
	}

	return nil
}

func (w *treeWriter[K, C]) completePendingLeaf() error {
	if len(w.pendingLeaf) == 0 {
		return nil
	}
	var kc C

	// create leaf node
	ref := w.w.Alloc()

	var entries []pdf.Object
	for _, e := range w.pendingLeaf {
		entries = append(entries, kc.encode(e.key))
		entries = append(entries, e.value)
	}

	node := pdf.Dict{
		kc.leafKey(): pdf.Array(entries),
		"Limits": pdf.Array{
			kc.encode(w.pendingLeaf[0].key),
			kc.encode(w.pendingLeaf[len(w.pendingLeaf)-1].key),
		},
	}

	err := w.w.Put(ref, node)
	if err != nil {
		return err
	}

	// add to tail
	info := &nodeInfo[K]{
		ref:       ref,
		depth:     0,
		minKey:    w.pendingLeaf[0].key,
		maxKey:    w.pendingLeaf[len(w.pendingLeaf)-1].key,
		nodeCount: len(w.pendingLeaf),
	}
	w.tail = append(w.tail, info)
	w.pendingLeaf = nil

	// apply merging logic (like pagetree)
	return w.mergeTail()
}

func (w *treeWriter[K, C]) mergeTail() error {
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

func (w *treeWriter[K, C]) mergeNodes(start, end int) error {
	if start >= end {
		return nil
	}
	var kc C

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
			kc.encode(children[0].minKey),
			kc.encode(children[len(children)-1].maxKey),
		},
	}

	err := w.w.Put(ref, node)
	if err != nil {
		return err
	}

	// replace children with merged node
	merged := &nodeInfo[K]{
		ref:       ref,
		depth:     children[0].depth + 1,
		minKey:    children[0].minKey,
		maxKey:    children[len(children)-1].maxKey,
		nodeCount: len(children),
	}

	// replace the range with merged node
	w.tail = append(w.tail[:start], append([]*nodeInfo[K]{merged}, w.tail[end:]...)...)

	return nil
}

func (w *treeWriter[K, C]) finish() (pdf.Reference, error) {
	// complete any pending leaf
	if len(w.pendingLeaf) > 0 {
		// special case: if this is the only leaf, make it a root with the
		// key-value pairs (no Limits)
		if len(w.tail) == 0 {
			return w.writeRootWithEntries()
		}
		err := w.completePendingLeaf()
		if err != nil {
			return 0, err
		}
	}

	// empty tree: no entries means no root object
	if len(w.tail) == 0 {
		return 0, nil
	}

	// single completed leaf: wrap it in a root with /Kids (no Limits on the root)
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

func (w *treeWriter[K, C]) writeRootWithEntries() (pdf.Reference, error) {
	var kc C
	ref := w.w.Alloc()

	var entries []pdf.Object
	for _, e := range w.pendingLeaf {
		entries = append(entries, kc.encode(e.key))
		entries = append(entries, e.value)
	}

	// root node with the key-value pairs has no Limits
	node := pdf.Dict{
		kc.leafKey(): pdf.Array(entries),
	}

	err := w.w.Put(ref, node)
	return ref, err
}

func (w *treeWriter[K, C]) writeRootFromSingleLeaf(leaf *nodeInfo[K]) (pdf.Reference, error) {
	// The leaf is already written and cannot be modified, so wrap it in a
	// fresh root whose /Kids points at it.  The root carries no Limits.
	ref := w.w.Alloc()
	node := pdf.Dict{
		"Kids": pdf.Array{leaf.ref},
	}
	err := w.w.Put(ref, node)
	return ref, err
}

func (w *treeWriter[K, C]) writeRootWithKids(kidRef pdf.Reference) (pdf.Reference, error) {
	ref := w.w.Alloc()
	node := pdf.Dict{
		"Kids": pdf.Array{kidRef},
	}
	err := w.w.Put(ref, node)
	return ref, err
}

// collapse reduces the tail to a single root node.  The tail holds completed
// subtrees whose depth is non-increasing from left to right.  Each step merges
// the trailing run of equal-depth nodes (capped at maxChildren) into one node
// of the next depth.  A lone trailing node shallower than its left neighbour is
// lifted one level, so it eventually reaches the neighbour's depth and merges
// with it.  Every step either shrinks the tail or raises the trailing node
// towards its neighbour, so this terminates.
func (w *treeWriter[K, C]) collapse() error {
	for len(w.tail) > 1 {
		end := len(w.tail)
		depth := w.tail[end-1].depth
		start := end - 1
		for start > 0 && w.tail[start-1].depth == depth {
			start--
		}
		if end-start > maxChildren {
			start = end - maxChildren
		}
		err := w.mergeNodes(start, end)
		if err != nil {
			return err
		}
	}
	return nil
}

// WriteMap creates a tree from a map of key-value pairs.
// The keys are sorted before writing.
// The return value is the reference to the tree root.
func WriteMap[K cmp.Ordered, C codec[K]](w *pdf.Writer, data map[K]pdf.Object) (pdf.Reference, error) {
	// collect and sort keys
	keys := make([]K, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	// create iterator over sorted entries
	seq := func(yield func(K, pdf.Object) bool) {
		for _, k := range keys {
			if !yield(k, data[k]) {
				return
			}
		}
	}

	return Write[K, C](w, seq)
}
