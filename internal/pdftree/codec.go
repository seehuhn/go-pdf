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

package pdftree

import (
	"cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/limits"
)

// codec describes how a tree's keys are represented in a PDF file.
//
// The implementations [NameCodec] and [NumCodec] are exported so the nametree
// and numtree facade packages can name them as type arguments.  The methods are
// unexported: only this package calls them.
type codec[K cmp.Ordered] interface {
	// leafKey is the dictionary key holding a leaf node's key-value pairs:
	// "Names" for name trees, "Nums" for number trees.
	leafKey() pdf.Name

	// maxDepth caps the /Kids nesting depth when reading a malformed tree.
	maxDepth() int

	// encode converts a key to its PDF object representation.
	encode(K) pdf.Object

	// decode reads a key from its PDF object representation.
	decode(pdf.Cursor, pdf.Object) (K, error)
}

// NameCodec is the codec for PDF name trees.  Keys are strings stored as PDF
// string objects and ordered lexicographically (PDF 32000-2 section 7.9.6).
type NameCodec struct{}

func (NameCodec) leafKey() pdf.Name { return "Names" }
func (NameCodec) maxDepth() int     { return limits.MaxNameTreeDepth }

func (NameCodec) encode(key pdf.Name) pdf.Object { return pdf.String(key) }

func (NameCodec) decode(c pdf.Cursor, obj pdf.Object) (pdf.Name, error) {
	s, err := c.String(obj)
	return pdf.Name(s), err
}

// NumCodec is the codec for PDF number trees.  Keys are integers stored as PDF
// integer objects and ordered numerically (PDF 32000-2 section 7.9.7).
type NumCodec struct{}

func (NumCodec) leafKey() pdf.Name { return "Nums" }
func (NumCodec) maxDepth() int     { return limits.MaxNumberTreeDepth }

func (NumCodec) encode(key pdf.Integer) pdf.Object { return key }

func (NumCodec) decode(c pdf.Cursor, obj pdf.Object) (pdf.Integer, error) {
	return c.Integer(obj)
}

var (
	_ codec[pdf.Name]    = NameCodec{}
	_ codec[pdf.Integer] = NumCodec{}
)
