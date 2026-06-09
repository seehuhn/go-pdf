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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/limits"
)

// PDF 2.0 sections: 7.9.6

// FromFile represents a name tree that allows reading values from a PDF file
// without holding the entire tree in memory.
type FromFile struct {
	r    pdf.Getter
	root pdf.Object
}

// ExtractFromFile creates a new FromFile name tree that reads from a PDF document.
// If root is nil, it returns nil.
func ExtractFromFile(r pdf.Getter, root pdf.Object) (*FromFile, error) {
	if root == nil {
		return nil, nil
	}
	return &FromFile{r: r, root: root}, nil
}

var _ pdf.NameTree = (*FromFile)(nil)

// Lookup returns the value for key, or [ErrKeyNotFound] if the key is absent.
//
// The search descends through intermediate nodes using their Limits arrays to
// avoid scanning every leaf.  Children whose Limits entry is missing or
// malformed are skipped, so in a malformed tree Lookup may return
// [ErrKeyNotFound] for keys that [FromFile.All] would yield.
func (t *FromFile) Lookup(key pdf.Name) (pdf.Object, error) {
	if t == nil {
		return nil, ErrKeyNotFound
	}

	node, err := pdf.GetDict(t.r, t.root)
	if err != nil || node == nil {
		return nil, ErrKeyNotFound
	}

	seen := map[pdf.Reference]bool{}
	if ref, ok := t.root.(pdf.Reference); ok {
		seen[ref] = true
	}
	return t.lookupInNode(node, seen, key, 0)
}

func (t *FromFile) lookupInNode(node pdf.Dict, seen map[pdf.Reference]bool, key pdf.Name, depth int) (pdf.Object, error) {
	if depth >= limits.MaxNameTreeDepth {
		return nil, &pdf.MalformedFileError{Err: errors.New("name tree nesting depth exceeded")}
	}

	// leaf node with Names
	if names, ok := node["Names"]; ok {
		arr, err := pdf.GetArray(t.r, names)
		if err != nil {
			return nil, ErrKeyNotFound
		}

		// search through Names array (key-value pairs)
		for i := 0; i+1 < len(arr); i += 2 {
			keyObj, err := pdf.GetString(t.r, arr[i])
			if err != nil {
				continue
			}
			if pdf.Name(keyObj) == key {
				return arr[i+1], nil
			}
		}
		return nil, ErrKeyNotFound
	}

	// intermediate node with Kids
	if kids, ok := node["Kids"]; ok {
		arr, err := pdf.GetArray(t.r, kids)
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
			childNode, err := pdf.GetDict(t.r, kid)
			if err != nil {
				continue
			}

			limits, ok := childNode["Limits"]
			if !ok {
				continue
			}

			limitsArr, err := pdf.GetArray(t.r, limits)
			if err != nil || len(limitsArr) != 2 {
				continue
			}

			minKey, err := pdf.GetString(t.r, limitsArr[0])
			if err != nil {
				continue
			}
			maxKey, err := pdf.GetString(t.r, limitsArr[1])
			if err != nil {
				continue
			}

			// check if key is within this child's range
			if string(key) >= string(minKey) && string(key) <= string(maxKey) {
				return t.lookupInNode(childNode, seen, key, depth+1)
			}
		}
	}

	return nil, ErrKeyNotFound
}

func (t *FromFile) All() iter.Seq2[pdf.Name, pdf.Object] {
	return func(yield func(pdf.Name, pdf.Object) bool) {
		if t == nil {
			return
		}

		node, err := pdf.GetDict(t.r, t.root)
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

func (t *FromFile) yieldFromNode(node pdf.Dict, seen map[pdf.Reference]bool, yield func(pdf.Name, pdf.Object) bool, depth int) bool {
	// skip subtrees deeper than the cap; over-deep input is treated as
	// malformed and silently truncated, the iterator continues elsewhere
	if depth >= limits.MaxNameTreeDepth {
		return true
	}

	// check if this is a leaf node with Names
	if names, ok := node["Names"]; ok {
		arr, err := pdf.GetArray(t.r, names)
		if err != nil {
			return true
		}

		// yield all key-value pairs from Names array
		for i := 0; i+1 < len(arr); i += 2 {
			keyObj, err := pdf.GetString(t.r, arr[i])
			if err != nil {
				continue
			}
			if !yield(pdf.Name(keyObj), arr[i+1]) {
				return false
			}
		}
		return true
	}

	// check if this is an intermediate node with Kids
	if kids, ok := node["Kids"]; ok {
		arr, err := pdf.GetArray(t.r, kids)
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
			childNode, err := pdf.GetDict(t.r, kid)
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

func (t *FromFile) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	ref, err := Write(rm.Out(), t.All())
	return ref, err
}
