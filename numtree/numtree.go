// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

type Tree interface {
	// Get returns the value for a given key.  If the key is not
	// found, Get returns ErrKeyNotFound.
	Get(key pdf.Integer) (pdf.Object, error)

	// First returns the smallest key in the tree.  If the tree is
	// empty, First returns ErrKeyNotFound.
	First() (pdf.Integer, error)

	// Next returns the smallest key in the tree which is strictly larger
	// than the given key.  If there is no such key, Next returns
	// ErrKeyNotFound.
	Next(after pdf.Integer) (pdf.Integer, error)

	// Prev returns the largest key in the tree which is smaller than or equal
	// to the given key.  If there is no such key, Prev returns ErrKeyNotFound.
	Prev(before pdf.Integer) (pdf.Integer, error)
}

var (
	ErrKeyNotFound = errors.New("key not found")
)
