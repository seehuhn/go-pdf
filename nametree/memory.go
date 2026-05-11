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
	"iter"
	"slices"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 7.9.6

// InMemory represents a name tree held entirely in memory.
type InMemory struct {
	Data map[pdf.Name]pdf.Object
}

var _ pdf.NameTree = (*InMemory)(nil)

// ExtractInMemory reads a name tree from a PDF document into memory.
// If obj is nil, it returns nil.
func ExtractInMemory(r pdf.Getter, root pdf.Object) (*InMemory, error) {
	if root == nil {
		return nil, nil
	}

	node, err := pdf.GetDict(r, root)
	if node == nil {
		return nil, err
	}

	tree := &InMemory{
		Data: make(map[pdf.Name]pdf.Object),
	}

	seen := map[pdf.Reference]bool{}
	if ref, ok := root.(pdf.Reference); ok {
		seen[ref] = true
	}
	extractFromNode(r, node, seen, tree.Data)

	return tree, nil
}

func extractFromNode(r pdf.Getter, node pdf.Dict, seen map[pdf.Reference]bool, data map[pdf.Name]pdf.Object) {
	// leaf node with Names
	if names, ok := node["Names"]; ok {
		arr, err := pdf.GetArray(r, names)
		if err != nil {
			return
		}

		// extract key-value pairs from Names array
		for i := 0; i+1 < len(arr); i += 2 {
			keyObj, err := pdf.GetString(r, arr[i])
			if err != nil {
				continue
			}
			data[pdf.Name(keyObj)] = arr[i+1]
		}
		return
	}

	// intermediate node with Kids
	if kids, ok := node["Kids"]; ok {
		arr, err := pdf.GetArray(r, kids)
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
			childNode, err := pdf.GetDict(r, kid)
			if err != nil {
				continue
			}
			extractFromNode(r, childNode, seen, data)
		}
	}
}

func (t *InMemory) Lookup(key pdf.Name) (pdf.Object, error) {
	if t == nil || t.Data == nil {
		return nil, ErrKeyNotFound
	}

	value, ok := t.Data[key]
	if !ok {
		return nil, ErrKeyNotFound
	}
	return value, nil
}

func (t *InMemory) All() iter.Seq2[pdf.Name, pdf.Object] {
	return func(yield func(pdf.Name, pdf.Object) bool) {
		if t == nil || t.Data == nil {
			return
		}

		// collect keys and sort them
		keys := make([]pdf.Name, 0, len(t.Data))
		for key := range t.Data {
			keys = append(keys, key)
		}

		// sort lexicographically
		slices.Sort(keys)

		// yield in sorted order
		for _, key := range keys {
			if !yield(key, t.Data[key]) {
				return
			}
		}
	}
}

// Embed adds the name tree to a PDF file.
func (t *InMemory) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	ref, err := Write(rm.Out(), t.All())
	return ref, err
}
