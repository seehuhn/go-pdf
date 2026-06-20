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
	"iter"
	"slices"

	"seehuhn.de/go/pdf"
)

// InMemory represents a tree held entirely in memory.
type InMemory[K cmp.Ordered, C codec[K]] struct {
	Data map[K]pdf.Object
}

// ExtractInMemory reads a tree from a PDF document into memory.
// If root is nil, it returns nil.
func ExtractInMemory[K cmp.Ordered, C codec[K]](r pdf.Getter, root pdf.Object) (*InMemory[K, C], error) {
	if root == nil {
		return nil, nil
	}

	c := pdf.NewCursor(r)
	node, err := c.Dict(root)
	if node == nil {
		return nil, err
	}

	tree := &InMemory[K, C]{
		Data: make(map[K]pdf.Object),
	}

	seen := map[pdf.Reference]bool{}
	if ref, ok := root.(pdf.Reference); ok {
		seen[ref] = true
	}
	extractFromNode[K, C](c, node, seen, tree.Data, 0)

	return tree, nil
}

func extractFromNode[K cmp.Ordered, C codec[K]](c pdf.Cursor, node pdf.Dict, seen map[pdf.Reference]bool, data map[K]pdf.Object, depth int) {
	var kc C

	// skip subtrees deeper than the cap; over-deep input is treated as
	// malformed and silently truncated, leaving a partial map
	if depth >= kc.maxDepth() {
		return
	}

	// leaf node
	if entries, ok := node[kc.leafKey()]; ok {
		arr, err := c.Array(entries)
		if err != nil {
			return
		}

		// extract key-value pairs
		for i := 0; i+1 < len(arr); i += 2 {
			key, err := kc.decode(c, arr[i])
			if err != nil {
				continue
			}
			data[key] = arr[i+1]
		}
		return
	}

	// intermediate node with Kids
	if kids, ok := node["Kids"]; ok {
		arr, err := c.Array(kids)
		if err != nil {
			return
		}

		for _, kid := range arr {
			if ref, isRef := kid.(pdf.Reference); isRef {
				if seen[ref] {
					continue
				}
				seen[ref] = true
			}
			childNode, err := c.Dict(kid)
			if err != nil {
				continue
			}
			extractFromNode[K, C](c, childNode, seen, data, depth+1)
		}
	}
}

func (t *InMemory[K, C]) Lookup(key K) (pdf.Object, error) {
	if t == nil || t.Data == nil {
		return nil, ErrKeyNotFound
	}

	value, ok := t.Data[key]
	if !ok {
		return nil, ErrKeyNotFound
	}
	return value, nil
}

func (t *InMemory[K, C]) All() iter.Seq2[K, pdf.Object] {
	return func(yield func(K, pdf.Object) bool) {
		if t == nil || t.Data == nil {
			return
		}

		// collect keys and sort them
		keys := make([]K, 0, len(t.Data))
		for key := range t.Data {
			keys = append(keys, key)
		}
		slices.Sort(keys)

		// yield in sorted order
		for _, key := range keys {
			if !yield(key, t.Data[key]) {
				return
			}
		}
	}
}

// Embed adds the tree to a PDF file.
func (t *InMemory[K, C]) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	ref, err := Write[K, C](rm.Out(), t.All())
	return ref, err
}
