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

	"seehuhn.de/go/pdf"
)

// FromFile represents a number tree that allows reading values from a PDF file
// without holding the entire tree in memory.
type FromFile struct {
	r    pdf.Getter
	root pdf.Object
}

// ExtractFromFile creates a new FromFile number tree that reads from a PDF document.
// If root is nil, it returns nil.
func ExtractFromFile(r pdf.Getter, root pdf.Object) (*FromFile, error) {
	if root == nil {
		return nil, nil
	}
	return &FromFile{r: r, root: root}, nil
}

var _ pdf.NumberTree = (*FromFile)(nil)

func (t *FromFile) Lookup(key pdf.Integer) (pdf.Object, error) {
	if t == nil {
		return nil, ErrKeyNotFound
	}

	node, err := pdf.GetDict(t.r, t.root)
	if node == nil {
		return nil, err
	}

	return t.lookupInNode(node, key)
}

func (t *FromFile) lookupInNode(node pdf.Dict, key pdf.Integer) (pdf.Object, error) {
	// check if this is a leaf node with Nums
	if nums, ok := node["Nums"]; ok {
		arr, err := pdf.GetArray(t.r, nums)
		if err != nil {
			return nil, err
		}

		// search through Nums array (key-value pairs)
		for i := 0; i < len(arr); i += 2 {
			keyObj, err := pdf.GetInteger(t.r, arr[i])
			if err != nil {
				continue
			}
			if keyObj == key {
				return arr[i+1], nil
			}
		}
		return nil, ErrKeyNotFound
	}

	// check if this is an intermediate node with Kids
	if kids, ok := node["Kids"]; ok {
		arr, err := pdf.GetArray(t.r, kids)
		if err != nil {
			return nil, err
		}

		// find the right child by checking Limits
		for _, kid := range arr {
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

			minKey, err := pdf.GetInteger(t.r, limitsArr[0])
			if err != nil {
				continue
			}
			maxKey, err := pdf.GetInteger(t.r, limitsArr[1])
			if err != nil {
				continue
			}

			// check if key is within this child's range
			if key >= minKey && key <= maxKey {
				return t.lookupInNode(childNode, key)
			}
		}
	}

	return nil, ErrKeyNotFound
}

func (t *FromFile) All() iter.Seq2[pdf.Integer, pdf.Object] {
	return func(yield func(pdf.Integer, pdf.Object) bool) {
		if t == nil {
			return
		}

		node, err := pdf.GetDict(t.r, t.root)
		if err != nil {
			return
		}

		t.yieldFromNode(node, yield)
	}
}

func (t *FromFile) yieldFromNode(node pdf.Dict, yield func(pdf.Integer, pdf.Object) bool) bool {
	// check if this is a leaf node with Nums
	if nums, ok := node["Nums"]; ok {
		arr, err := pdf.GetArray(t.r, nums)
		if err != nil {
			return true
		}

		// yield all key-value pairs from Nums array
		for i := 0; i < len(arr); i += 2 {
			keyObj, err := pdf.GetInteger(t.r, arr[i])
			if err != nil {
				continue
			}
			if !yield(keyObj, arr[i+1]) {
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
			childNode, err := pdf.GetDict(t.r, kid)
			if err != nil {
				continue
			}

			if !t.yieldFromNode(childNode, yield) {
				return false
			}
		}
	}

	return true
}

func (t *FromFile) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	ref, err := Write(rm.Out, t.All())
	return ref, pdf.Unused{}, err
}
