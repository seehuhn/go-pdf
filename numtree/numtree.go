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

package numtree

import (
	"iter"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/pdftree"
)

// InMemory represents a number tree held entirely in memory.
type InMemory = pdftree.InMemory[pdf.Integer, pdftree.NumCodec]

// FromFile represents a number tree that allows reading values from a PDF file
// without holding the entire tree in memory.
type FromFile = pdftree.FromFile[pdf.Integer, pdftree.NumCodec]

var (
	_ pdf.NumberTree = (*InMemory)(nil)
	_ pdf.NumberTree = (*FromFile)(nil)
)

// ErrKeyNotFound is returned by Lookup when the key is absent.
var ErrKeyNotFound = pdftree.ErrKeyNotFound

// ExtractInMemory reads a number tree from a PDF document into memory.
// If root is nil, it returns nil.
func ExtractInMemory(r pdf.Getter, root pdf.Object) (*InMemory, error) {
	return pdftree.ExtractInMemory[pdf.Integer, pdftree.NumCodec](r, root)
}

// ExtractFromFile creates a new FromFile number tree that reads from a PDF document.
// If root is nil, it returns nil.
func ExtractFromFile(r pdf.Getter, root pdf.Object) (*FromFile, error) {
	return pdftree.ExtractFromFile[pdf.Integer, pdftree.NumCodec](r, root)
}

// Write creates a number tree in the PDF file.
// The iterator data provides the key-value pairs.  The keys must be returned in
// sorted order, and must not contain duplicates.
// The return value is the reference to the number tree root.  An empty sequence
// produces the null reference rather than a tree object.
func Write(w *pdf.Writer, data iter.Seq2[pdf.Integer, pdf.Object]) (pdf.Reference, error) {
	return pdftree.Write[pdf.Integer, pdftree.NumCodec](w, data)
}

// Size returns the number of entries in the number tree,
// without reading the entire tree into memory.
func Size(r pdf.Getter, root pdf.Object) (int, error) {
	return pdftree.Size[pdf.Integer, pdftree.NumCodec](r, root)
}
