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

package nametree

import (
	"errors"
	"iter"
	"slices"
	"strings"

	"seehuhn.de/go/pdf"
)

// maxChildren is the maximum number of children we allow in a node while writing a name tree.
const maxChildren = 64

type entry struct {
	key   pdf.Name
	value pdf.Object
}

type nodeInfo struct {
	ref       pdf.Reference
	depth     int
	minKey    string
	maxKey    string
	nodeCount int // number of child nodes (for intermediate) or entries (for leaf)
}

type nameTreeWriter struct {
	w           *pdf.Writer
	tail        []*nodeInfo // completed nodes at various depths
	pendingLeaf []entry     // accumulating entries for current leaf
	lastKey     pdf.Name    // for sort order validation
	hasEntries  bool        // track if we've seen any entries
}

// Write creates a name tree in the PDF file.
// The iterator `data` provides the key-value pairs.  The keys must be
// returned in sorted order, and must not contain duplicates.
// The return value is the reference to the name tree root.
func Write(w *pdf.Writer, data iter.Seq2[pdf.Name, pdf.Object]) (pdf.Reference, error) {
	writer := &nameTreeWriter{
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

func (w *nameTreeWriter) addEntry(key pdf.Name, value pdf.Object) error {
	// validate sort order
	if w.hasEntries && string(key) <= string(w.lastKey) {
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

func (w *nameTreeWriter) completePendingLeaf() error {
	if len(w.pendingLeaf) == 0 {
		return nil
	}

	// create leaf node
	ref := w.w.Alloc()

	var names []pdf.Object
	for _, e := range w.pendingLeaf {
		names = append(names, pdf.String(e.key))
		names = append(names, e.value)
	}

	node := pdf.Dict{
		"Names": pdf.Array(names),
		"Limits": pdf.Array{
			pdf.String(w.pendingLeaf[0].key),
			pdf.String(w.pendingLeaf[len(w.pendingLeaf)-1].key),
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
		minKey:    string(w.pendingLeaf[0].key),
		maxKey:    string(w.pendingLeaf[len(w.pendingLeaf)-1].key),
		nodeCount: len(w.pendingLeaf),
	}
	w.tail = append(w.tail, nodeInfo)
	w.pendingLeaf = nil

	// apply merging logic (like pagetree)
	return w.mergeTail()
}

func (w *nameTreeWriter) mergeTail() error {
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

func (w *nameTreeWriter) mergeNodes(start, end int) error {
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
			pdf.String(children[0].minKey),
			pdf.String(children[len(children)-1].maxKey),
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

func (w *nameTreeWriter) finish() (pdf.Reference, error) {
	// complete any pending leaf
	if len(w.pendingLeaf) > 0 {
		// special case: if this is the only leaf, make it a root with Names (no Limits)
		if len(w.tail) == 0 {
			return w.writeRootWithNames()
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

	// special case: single leaf becomes root with Names (no Limits)
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

func (w *nameTreeWriter) writeRootWithNames() (pdf.Reference, error) {
	ref := w.w.Alloc()

	var names []pdf.Object
	for _, e := range w.pendingLeaf {
		names = append(names, pdf.String(e.key))
		names = append(names, e.value)
	}

	// root node with Names has no Limits
	node := pdf.Dict{
		"Names": pdf.Array(names),
	}

	err := w.w.Put(ref, node)
	return ref, err
}

func (w *nameTreeWriter) writeRootFromSingleLeaf(leaf *nodeInfo) (pdf.Reference, error) {
	// we need to rewrite the leaf as a root (without Limits)
	// since we can't modify already written objects, create new root
	ref := w.w.Alloc()
	node := pdf.Dict{
		"Kids": pdf.Array{leaf.ref},
	}
	err := w.w.Put(ref, node)
	return ref, err
}

func (w *nameTreeWriter) writeRootWithKids(kidRef pdf.Reference) (pdf.Reference, error) {
	ref := w.w.Alloc()
	node := pdf.Dict{
		"Kids": pdf.Array{kidRef},
	}
	err := w.w.Put(ref, node)
	return ref, err
}

func (w *nameTreeWriter) collapse() error {
	for len(w.tail) > 1 {
		start := len(w.tail) - maxChildren
		if start < 0 {
			start = 0
		}
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

// WriteMap creates a name tree from a map of key-value pairs.
// The keys are sorted lexicographically before writing.
// The return value is the reference to the name tree root.
func WriteMap(w *pdf.Writer, data map[pdf.Name]pdf.Object) (pdf.Reference, error) {
	// collect and sort keys
	keys := make([]pdf.Name, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b pdf.Name) int {
		return strings.Compare(string(a), string(b))
	})

	// create iterator over sorted entries
	iter := func(yield func(pdf.Name, pdf.Object) bool) {
		for _, k := range keys {
			if !yield(k, data[k]) {
				return
			}
		}
	}

	return Write(w, iter)
}
