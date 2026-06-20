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

	"seehuhn.de/go/pdf"
)

// FromFile represents a tree that allows reading values from a PDF file without
// holding the entire tree in memory.
type FromFile[K cmp.Ordered, C codec[K]] struct {
	cur  pdf.Cursor
	root pdf.Object
}

// ExtractFromFile creates a FromFile tree that reads from a PDF document.
// If root is nil, it returns nil.
func ExtractFromFile[K cmp.Ordered, C codec[K]](r pdf.Getter, root pdf.Object) (*FromFile[K, C], error) {
	if root == nil {
		return nil, nil
	}
	return &FromFile[K, C]{cur: pdf.NewCursor(r), root: root}, nil
}

// Lookup returns the value for key, or [ErrKeyNotFound] if the key is absent.
//
// The search descends through intermediate nodes using their Limits arrays to
// avoid scanning every leaf.  Children whose Limits entry is missing or
// malformed are skipped, so in a malformed tree Lookup may return
// [ErrKeyNotFound] for keys that [FromFile.All] would yield.
func (t *FromFile[K, C]) Lookup(key K) (pdf.Object, error) {
	if t == nil {
		return nil, ErrKeyNotFound
	}

	node, err := t.cur.Dict(t.root)
	if err != nil || node == nil {
		return nil, ErrKeyNotFound
	}

	seen := map[pdf.Reference]bool{}
	if ref, ok := t.root.(pdf.Reference); ok {
		seen[ref] = true
	}
	return t.lookupInNode(node, seen, key, 0)
}

func (t *FromFile[K, C]) lookupInNode(node pdf.Dict, seen map[pdf.Reference]bool, key K, depth int) (pdf.Object, error) {
	var kc C

	if depth >= kc.maxDepth() {
		return nil, &pdf.MalformedFileError{Err: errors.New("tree nesting depth exceeded")}
	}

	// leaf node
	if entries, ok := node[kc.leafKey()]; ok {
		arr, err := t.cur.Array(entries)
		if err != nil {
			return nil, ErrKeyNotFound
		}

		// search through the key-value pairs
		for i := 0; i+1 < len(arr); i += 2 {
			k, err := kc.decode(t.cur, arr[i])
			if err != nil {
				continue
			}
			if k == key {
				return arr[i+1], nil
			}
		}
		return nil, ErrKeyNotFound
	}

	// intermediate node with Kids
	if kids, ok := node["Kids"]; ok {
		arr, err := t.cur.Array(kids)
		if err != nil {
			return nil, ErrKeyNotFound
		}

		// find the right child by checking Limits
		for _, kid := range arr {
			if ref, isRef := kid.(pdf.Reference); isRef {
				if seen[ref] {
					continue
				}
				seen[ref] = true
			}
			childNode, err := t.cur.Dict(kid)
			if err != nil {
				continue
			}

			limits, ok := childNode["Limits"]
			if !ok {
				continue
			}

			limitsArr, err := t.cur.Array(limits)
			if err != nil || len(limitsArr) != 2 {
				continue
			}

			minKey, err := kc.decode(t.cur, limitsArr[0])
			if err != nil {
				continue
			}
			maxKey, err := kc.decode(t.cur, limitsArr[1])
			if err != nil {
				continue
			}

			// check if key is within this child's range
			if key >= minKey && key <= maxKey {
				return t.lookupInNode(childNode, seen, key, depth+1)
			}
		}
	}

	return nil, ErrKeyNotFound
}

func (t *FromFile[K, C]) All() iter.Seq2[K, pdf.Object] {
	return func(yield func(K, pdf.Object) bool) {
		if t == nil {
			return
		}

		node, err := t.cur.Dict(t.root)
		if err != nil {
			return
		}

		seen := map[pdf.Reference]bool{}
		if ref, ok := t.root.(pdf.Reference); ok {
			seen[ref] = true
		}
		t.yieldFromNode(node, seen, yield, 0)
	}
}

func (t *FromFile[K, C]) yieldFromNode(node pdf.Dict, seen map[pdf.Reference]bool, yield func(K, pdf.Object) bool, depth int) bool {
	var kc C

	// skip subtrees deeper than the cap; over-deep input is treated as
	// malformed and silently truncated, the iterator continues elsewhere
	if depth >= kc.maxDepth() {
		return true
	}

	// leaf node
	if entries, ok := node[kc.leafKey()]; ok {
		arr, err := t.cur.Array(entries)
		if err != nil {
			return true
		}

		// yield all key-value pairs
		for i := 0; i+1 < len(arr); i += 2 {
			k, err := kc.decode(t.cur, arr[i])
			if err != nil {
				continue
			}
			if !yield(k, arr[i+1]) {
				return false
			}
		}
		return true
	}

	// intermediate node with Kids
	if kids, ok := node["Kids"]; ok {
		arr, err := t.cur.Array(kids)
		if err != nil {
			return true
		}

		// recursively yield from all children in order
		for _, kid := range arr {
			if ref, isRef := kid.(pdf.Reference); isRef {
				if seen[ref] {
					continue
				}
				seen[ref] = true
			}
			childNode, err := t.cur.Dict(kid)
			if err != nil {
				continue
			}

			if !t.yieldFromNode(childNode, seen, yield, depth+1) {
				return false
			}
		}
	}

	return true
}

func (t *FromFile[K, C]) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	ref, err := Write[K, C](rm.Out(), t.All())
	return ref, err
}
