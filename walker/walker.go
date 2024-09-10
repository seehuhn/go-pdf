// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

// Package walker provides functionality to iterate over all objects in a PDF
// file.
package walker

import (
	"iter"

	"seehuhn.de/go/pdf"
)

// A Walker can iterate over all objects in a PDF file.
//
// The iteration traverses the file recursively, starting from the
// document information dictionary, the document catalog, and the trailer
// dictionary, in this order.  Each object is visited exactly once.
//
// This only visits objects in the PDF container.  It does not include the
// contents of PDF content streams.
type Walker struct {
	pdf.Getter

	// Err holds the first error encountered during traversal.
	// The traversal stops immediately when an error is encountered.
	// Users should check this field after traversal.
	Err error
}

// New creates a new Walker instance from a pdf.Getter.
func New(r pdf.Getter) *Walker {
	return &Walker{Getter: r}
}

// PreOrder returns an iterator that traverses the PDF document structure in pre-order.
// For each object, it yields the object's reference (if any) and the object itself.
// If the object is not an indirect object, the reference is nil.
//
// In pre-order traversal, each node is visited before its children. This means
// that for complex nested structures like dictionaries or arrays, the container
// object is yielded before its contents.
//
// The iterator cannot be used concurrently.
func (w *Walker) PreOrder() iter.Seq2[pdf.Reference, pdf.Native] {
	return func(yield func(pdf.Reference, pdf.Native) bool) {
		w.walk(yield, true)
	}
}

// PostOrder returns an iterator that traverses the PDF document structure in post-order.
// For each object, it yields the object's reference (if any) and the object itself.
// If the object is not an indirect object, the reference is nil.
//
// In post-order traversal, each node is visited after its children. This means
// that for complex nested structures like dictionaries or arrays, the contents
// are yielded before the container object itself.
//
// The iterator cannot be used concurrently.
func (w *Walker) PostOrder() iter.Seq2[pdf.Reference, pdf.Native] {
	return func(yield func(pdf.Reference, pdf.Native) bool) {
		w.walk(yield, false)
	}
}

func (w *Walker) walk(yield func(pdf.Reference, pdf.Native) bool, preOrder bool) {
	w.Err = nil
	visited := make(map[pdf.Reference]struct{})

	meta := w.GetMeta()
	if !w.walkObject(pdf.AsDict(meta.Info), yield, preOrder, visited) {
		return
	}
	if !w.walkObject(pdf.AsDict(meta.Catalog), yield, preOrder, visited) {
		return
	}
	if !w.walkObject(meta.Trailer, yield, preOrder, visited) {
		return
	}
}

func (w *Walker) walkObject(obj pdf.Native, yield func(pdf.Reference, pdf.Native) bool, preOrder bool, visited map[pdf.Reference]struct{}) bool {
	if obj == nil {
		return true
	}

	// resolve references
	ref, isReference := obj.(pdf.Reference)
	if isReference {
		if _, alreadyVisited := visited[ref]; alreadyVisited || ref == 0 {
			return true
		}
		visited[ref] = struct{}{}

		resolved, err := w.Get(ref, true)
		if err != nil {
			w.Err = err
			return false
		}
		obj = resolved
	}

	// for pre-order traversal, yield the object before visiting its children
	if preOrder {
		cont := yield(ref, obj)
		if !cont {
			return false
		}
	}

	// iterate over children
	switch v := obj.(type) {
	case pdf.Array:
		for _, item := range v {
			cont := w.walkObject(item.AsPDF(0), yield, preOrder, visited)
			if !cont {
				return false
			}
		}
	case pdf.Dict:
		keys := v.SortedKeys()
		for _, k := range keys {
			cont := w.walkObject(v[k].AsPDF(0), yield, preOrder, visited)
			if !cont {
				return false
			}
		}
	case *pdf.Stream:
		cont := w.walkObject(v.Dict, yield, preOrder, visited)
		if !cont {
			return false
		}
	}

	// for post-order traversal, yield the object after visiting its children
	if !preOrder {
		cont := yield(ref, obj)
		if !cont {
			return false
		}
	}

	return true
}

func (w *Walker) IndirectObjects() iter.Seq2[pdf.Reference, pdf.Native] {
	return func(yield func(pdf.Reference, pdf.Native) bool) {
		for ref, obj := range w.PreOrder() {
			if ref == 0 {
				continue
			}
			if !yield(ref, obj) {
				return
			}
		}
	}
}
