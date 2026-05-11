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
	"errors"

	"seehuhn.de/go/pdf"
)

// Size returns the number of entries in the number tree,
// without reading the entire tree into memory.
func Size(r pdf.Getter, root pdf.Object) (int, error) {
	tree, err := ExtractFromFile(r, root)
	if err != nil {
		return 0, err
	}
	count := 0
	for range tree.All() {
		count++
	}
	return count, nil
}

var ErrKeyNotFound = errors.New("key not found")
