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

	"seehuhn.de/go/pdf"
)

// Size returns the number of entries in the name tree,
// without reading the entire tree into memory.
func Size(r pdf.Getter, root pdf.Object) (int, error) {
	node, err := pdf.GetDict(r, root)
	if node == nil {
		return 0, err
	}

	return sizeNode(r, node)
}

func sizeNode(r pdf.Getter, node pdf.Dict) (int, error) {
	// leaf or single-node tree with Names
	if names, ok := node["Names"]; ok {
		arr, err := pdf.GetArray(r, names)
		if err != nil {
			return 0, err
		}
		return len(arr) / 2, nil // Names array has key-value pairs
	}

	// intermediate or root node with Kids
	if kids, ok := node["Kids"]; ok {
		arr, err := pdf.GetArray(r, kids)
		if err != nil {
			return 0, err
		}

		total := 0
		for _, kid := range arr {
			childNode, err := pdf.GetDict(r, kid)
			if err != nil {
				return 0, err
			}
			childSize, err := sizeNode(r, childNode)
			if err != nil {
				return 0, err
			}
			total += childSize
		}
		return total, nil
	}

	return 0, nil
}

var ErrKeyNotFound = errors.New("key not found")
