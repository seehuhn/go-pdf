// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package pdftree

import (
	"cmp"
	"errors"

	"seehuhn.de/go/pdf"
)

// ErrKeyNotFound is returned by Lookup when the key is absent from the tree.
var ErrKeyNotFound = errors.New("key not found")

// Size returns the number of entries in the tree, without reading the entire
// tree into memory.
func Size[K cmp.Ordered, C codec[K]](r pdf.Getter, root pdf.Object) (int, error) {
	tree, err := ExtractFromFile[K, C](r, root)
	if err != nil {
		return 0, err
	}
	count := 0
	for range tree.All() {
		count++
	}
	return count, nil
}
