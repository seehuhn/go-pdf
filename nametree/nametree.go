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

package nametree

import (
	"iter"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/pdftree"
)

// InMemory represents a name tree held entirely in memory.
type InMemory = pdftree.InMemory[pdf.Name, pdftree.NameCodec]

// FromFile represents a name tree that allows reading values from a PDF file
// without holding the entire tree in memory.
type FromFile = pdftree.FromFile[pdf.Name, pdftree.NameCodec]

var (
	_ pdf.NameTree = (*InMemory)(nil)
	_ pdf.NameTree = (*FromFile)(nil)
)

// ErrKeyNotFound is returned by Lookup when the key is absent.
var ErrKeyNotFound = pdftree.ErrKeyNotFound

// ExtractInMemory reads a name tree from a PDF document into memory.
// If root is nil, it returns nil.
func ExtractInMemory(r pdf.Getter, root pdf.Object) (*InMemory, error) {
	return pdftree.ExtractInMemory[pdf.Name, pdftree.NameCodec](r, root)
}

// ExtractFromFile creates a new FromFile name tree that reads from a PDF document.
// If root is nil, it returns nil.
func ExtractFromFile(r pdf.Getter, root pdf.Object) (*FromFile, error) {
	return pdftree.ExtractFromFile[pdf.Name, pdftree.NameCodec](r, root)
}

// Write creates a name tree in the PDF file.
// The iterator data provides the key-value pairs.  The keys must be returned in
// sorted order, and must not contain duplicates.
// The return value is the reference to the name tree root.  An empty sequence
// produces the null reference rather than a tree object.
func Write(w *pdf.Writer, data iter.Seq2[pdf.Name, pdf.Object]) (pdf.Reference, error) {
	return pdftree.Write[pdf.Name, pdftree.NameCodec](w, data)
}

// WriteMap creates a name tree from a map of key-value pairs.
// The keys are sorted lexicographically before writing.
// The return value is the reference to the name tree root.  An empty map
// produces the null reference rather than a tree object.
func WriteMap(w *pdf.Writer, data map[pdf.Name]pdf.Object) (pdf.Reference, error) {
	return pdftree.WriteMap[pdf.Name, pdftree.NameCodec](w, data)
}

// Size returns the number of entries in the name tree,
// without reading the entire tree into memory.
func Size(r pdf.Getter, root pdf.Object) (int, error) {
	return pdftree.Size[pdf.Name, pdftree.NameCodec](r, root)
}
