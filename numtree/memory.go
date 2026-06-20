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
	"iter"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/limits"
)

// PDF 2.0 sections: 7.9.7

// InMemory represents a number tree held entirely in memory.
type InMemory struct {
	Data map[pdf.Integer]pdf.Object
}

var _ pdf.NumberTree = (*InMemory)(nil)

// ExtractInMemory reads a number tree from a PDF document into memory.
// If obj is nil, it returns nil.
func ExtractInMemory(r pdf.Getter, root pdf.Object) (*InMemory, error) {
	if root == nil {
		return nil, nil
	}

	c := pdf.NewCursor(r)
	node, err := c.Dict(root)
	if node == nil {
		return nil, err
	}

	tree := &InMemory{
		Data: make(map[pdf.Integer]pdf.Object),
	}

	seen := map[pdf.Reference]bool{}
	if ref, ok := root.(pdf.Reference); ok {
		seen[ref] = true
	}
	extractFromNode(c, node, seen, tree.Data, 0)

	return tree, nil
}

func extractFromNode(c pdf.Cursor, node pdf.Dict, seen map[pdf.Reference]bool, data map[pdf.Integer]pdf.Object, depth int) {
	// skip subtrees deeper than the cap; over-deep input is treated as
	// malformed and silently truncated, leaving a partial map
	if depth >= limits.MaxNumberTreeDepth {
		return
	}

	// leaf node with Nums
	if nums, ok := node["Nums"]; ok {
		arr, err := c.Array(nums)
		if err != nil {
			return
		}

		// extract key-value pairs from Nums array
		for i := 0; i+1 < len(arr); i += 2 {
			keyObj, err := c.Integer(arr[i])
			if err != nil {
				continue
			}
			data[keyObj] = arr[i+1]
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
			extractFromNode(c, childNode, seen, data, depth+1)
		}
	}
}

func (t *InMemory) Lookup(key pdf.Integer) (pdf.Object, error) {
	if t == nil || t.Data == nil {
		return nil, ErrKeyNotFound
	}

	value, ok := t.Data[key]
	if !ok {
		return nil, ErrKeyNotFound
	}
	return value, nil
}

func (t *InMemory) All() iter.Seq2[pdf.Integer, pdf.Object] {
	return func(yield func(pdf.Integer, pdf.Object) bool) {
		if t == nil || t.Data == nil {
			return
		}

		// collect keys and sort them numerically
		keys := make([]pdf.Integer, 0, len(t.Data))
		for key := range t.Data {
			keys = append(keys, key)
		}

		// sort numerically
		slices.Sort(keys)

		// yield in sorted order
		for _, key := range keys {
			if !yield(key, t.Data[key]) {
				return
			}
		}
	}
}

// Embed adds the number tree to a PDF file.
func (t *InMemory) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	ref, err := Write(rm.Out(), t.All())
	return ref, err
}
