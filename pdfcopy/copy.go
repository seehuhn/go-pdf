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

package pdfcopy

import (
	"seehuhn.de/go/pdf"
)

// A Copier is used to copy objects from one PDF file to another. The Copier
// keeps track of the objects that have already been copied and ensures that
// each object is copied only once.
//
// Indirect objects are allocated in the target file as needed, and references
// are translated accordingly.
type Copier struct {
	trans map[pdf.Reference]pdf.Reference
	r     pdf.Getter
	w     pdf.Putter
}

// NewCopier creates a new Copier.
func NewCopier(w pdf.Putter, r pdf.Getter) *Copier {
	c := &Copier{
		trans: make(map[pdf.Reference]pdf.Reference),
		w:     w,
		r:     r,
	}
	return c
}

// Copy copies an object from the source file to the target file, recursively.
//
// The returned object is guaranteed to be the same type as the input object,
func (c *Copier) Copy(obj pdf.Native) (pdf.Native, error) {
	switch x := obj.(type) {
	case pdf.Dict:
		return c.CopyDict(x)
	case pdf.Array:
		return c.CopyArray(x)
	case *pdf.Stream:
		dict, err := c.CopyDict(x.Dict)
		if err != nil {
			return nil, err
		}
		res := &pdf.Stream{
			Dict: dict,
			R:    x.R,
		}
		return res, nil
	case pdf.Reference:
		return c.CopyReference(x)
	default:
		return obj, nil
	}
}

// CopyDict copies a dictionary from the source file to the target file,
func (c *Copier) CopyDict(obj pdf.Dict) (pdf.Dict, error) {
	res := pdf.Dict{}
	for key, val := range obj {
		repl, err := c.Copy(val.AsPDF(c.w.GetOptions()))
		if err != nil {
			return nil, err
		}
		res[key] = repl
	}

	return res, nil
}

// CopyArray copies an array from the source file to the target file,
func (c *Copier) CopyArray(obj pdf.Array) (pdf.Array, error) {
	var res pdf.Array
	for _, val := range obj {
		repl, err := c.Copy(val.AsPDF(c.w.GetOptions()))
		if err != nil {
			return nil, err
		}
		res = append(res, repl)
	}
	return res, nil
}

// CopyReference copies a reference from the source file to the target file,
//
// This method shortens chains of indirect references, the returned reference
// always points to a direct object.
func (c *Copier) CopyReference(obj pdf.Reference) (pdf.Reference, error) {
	newRef, ok := c.trans[obj]
	if ok {
		return newRef, nil
	}
	newRef = c.w.Alloc()
	c.trans[obj] = newRef

	val, err := pdf.Resolve(c.r, obj)
	if err != nil {
		return 0, err
	}
	trans, err := c.Copy(val)
	if err != nil {
		return 0, err
	}
	err = c.w.Put(newRef, trans)
	if err != nil {
		return 0, err
	}

	return newRef, nil
}

// Redirect replaces an indirect object in the old file with one in the new file.
func (c *Copier) Redirect(origRef, newRef pdf.Reference) {
	c.trans[origRef] = newRef
}
