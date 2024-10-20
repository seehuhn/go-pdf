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
// For each object, it yields a path to the object and the object itself.
// For indirect objects, the same path is yielded twice: once for the reference
// and once for the resolved object.
//
// In pre-order traversal, each node is visited before its children. This means
// that for complex nested structures like dictionaries or arrays, the container
// object is yielded before its contents.
//
// Iterators cannot be used concurrently.
func (w *Walker) PreOrder() iter.Seq2[[]pdf.Object, pdf.Native] {
	return func(yield func([]pdf.Object, pdf.Native) bool) {
		w.walk(yield, true)
	}
}

// PostOrder returns an iterator that traverses the PDF document structure in post-order.
// For each object, it yields a path to the object and the object itself.
// For indirect objects, the same path is yielded twice: once for the reference
// and once for the resolved object.
//
// In post-order traversal, each node is visited after its children. This means
// that for complex nested structures like dictionaries or arrays, the contents
// are yielded before the container object itself.
//
// Iterators cannot be used concurrently.
func (w *Walker) PostOrder() iter.Seq2[[]pdf.Object, pdf.Native] {
	return func(yield func([]pdf.Object, pdf.Native) bool) {
		w.walk(yield, false)
	}
}

type data struct {
	r        pdf.Getter
	yield    func([]pdf.Object, pdf.Native) bool
	path     []pdf.Object
	preOrder bool
	visited  map[pdf.Reference]struct{}
	err      error
}

func (w *Walker) walk(yield func([]pdf.Object, pdf.Native) bool, preOrder bool) {
	meta := w.GetMeta()

	d := &data{
		r:        w.Getter,
		yield:    yield,
		preOrder: preOrder,
		visited:  make(map[pdf.Reference]struct{}),
	}

	if ref, ok := meta.Trailer["Info"].(pdf.Reference); ok {
		d.visited[ref] = struct{}{}
	}
	if ref, ok := meta.Trailer["Root"].(pdf.Reference); ok {
		d.visited[ref] = struct{}{}
	}

	d.path = append(d.path[:0], pdf.Name("info"))
	d.walkObject(pdf.AsDict(meta.Info))
	d.path = append(d.path[:0], pdf.Name("catalog"))
	d.walkObject(pdf.AsDict(meta.Catalog))
	d.path = append(d.path[:0], pdf.Name("trailer"))
	d.walkObject(meta.Trailer)

	w.Err = d.err
}

// walkObject traverses the object obj recursively.
// It returns false if the traversal should be aborted.
func (d *data) walkObject(obj pdf.Native) bool {
	if d.err != nil {
		return false
	}
	if obj == nil {
		return true
	}

	k := len(d.path)

	// resolve references
	ref, isReference := obj.(pdf.Reference)
	if isReference {
		cont := d.yield(d.path[:k], ref)
		if !cont {
			return false
		}

		if _, alreadyVisited := d.visited[ref]; alreadyVisited || ref == 0 {
			return true
		}
		d.visited[ref] = struct{}{}

		resolved, err := d.r.Get(ref, true)
		if err != nil {
			d.err = err
			return false
		}

		if stm, isStream := resolved.(*pdf.Stream); isStream {
			// Because the Length depends on whether or not the stream is
			// encrypted or not, it is not safe to unconditionally re-use when
			// writing the object to another PDF file. To allow easy copying
			// for PDF file contents using the Walker, we remove the Length key
			// from the stream dictionary here, to trigger automatic
			// recalculation of the appropriate Length for any output file.
			delete(stm.Dict, "Length")
		}

		obj = resolved
	}

	// for pre-order traversal, yield the object before visiting its children
	if d.preOrder {
		cont := d.yield(d.path[:k], obj)
		if !cont {
			return false
		}
	}

	// iterate over children
	switch v := obj.(type) {
	case pdf.Array:
		for i, item := range v {
			d.path = append(d.path[:k], pdf.Integer(i))
			cont := d.walkObject(native(item))
			if !cont {
				return false
			}
		}
	case pdf.Dict:
		keys := v.SortedKeys()
		for _, key := range keys {
			native := native(v[key])
			d.path = append(d.path[:k], key)
			cont := d.walkObject(native)
			if !cont {
				return false
			}
		}
	case *pdf.Stream:
		cont := d.walkObject(v.Dict)
		if !cont {
			return false
		}
	}

	// for post-order traversal, yield the object after visiting its children
	if !d.preOrder {
		cont := d.yield(d.path[:k], obj)
		if !cont {
			return false
		}
	}

	return true
}

func native(obj pdf.Object) pdf.Native {
	if obj == nil {
		return nil
	}
	return obj.AsPDF(0)
}
