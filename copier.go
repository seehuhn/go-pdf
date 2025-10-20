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

package pdf

// A Copier is used to copy objects from one PDF file to another. The Copier
// keeps track of the objects that have already been copied and ensures that
// each object is copied only once.
//
// Indirect objects are allocated in the target file as needed, and references
// are translated accordingly.
type Copier struct {
	trans map[Reference]Reference
	r     Getter
	w     *Writer
}

// NewCopier creates a new Copier.
func NewCopier(w *Writer, r Getter) *Copier {
	c := &Copier{
		trans: make(map[Reference]Reference),
		w:     w,
		r:     r,
	}
	return c
}

// Copy copies an object from the source file to the target file, recursively.
//
// The returned object is guaranteed to be the same type as the input object,
func (c *Copier) Copy(obj Native) (Native, error) {
	switch x := obj.(type) {
	case Dict:
		return c.CopyDict(x)
	case Array:
		return c.CopyArray(x)
	case *Stream:
		dict, err := c.CopyDict(x.Dict)
		if err != nil {
			return nil, err
		}
		res := &Stream{
			Dict: dict,
			R:    x.R,
		}
		return res, nil
	case Reference:
		return c.CopyReference(x)
	default:
		return obj, nil
	}
}

// CopyDict copies a dictionary from the source file to the target file,
func (c *Copier) CopyDict(obj Dict) (Dict, error) {
	res := Dict{}
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
func (c *Copier) CopyArray(obj Array) (Array, error) {
	var res Array
	for _, val := range obj {
		var repl Native
		if val != nil {
			var err error
			repl, err = c.Copy(val.AsPDF(c.w.GetOptions()))
			if err != nil {
				return nil, err
			}
		}
		res = append(res, repl)
	}
	return res, nil
}

// CopyReference copies a reference from the source file to the target file,
//
// This method shortens chains of indirect references, the returned reference
// always points to a direct object.
func (c *Copier) CopyReference(obj Reference) (Reference, error) {
	newRef, ok := c.trans[obj]
	if ok {
		return newRef, nil
	}
	newRef = c.w.Alloc()
	c.trans[obj] = newRef

	val, err := Resolve(c.r, obj)
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
func (c *Copier) Redirect(origRef, newRef Reference) {
	c.trans[origRef] = newRef
}

// CopierCopyStruct copies a struct from the source file to the target file.
// For this to work, `data` must be a pointer to a struct, and must be
// a valid argument to [AsDict].  Otherwise, the function panics.
//
// Once Go supports methods with type parameters, this function can be turned
// into a method on [Copier].
//
// Deprecated: This function will be removed.
//
// TODO(voss): remove
func CopierCopyStruct[T any](c *Copier, data *T) (*T, error) {
	oldDict := AsDict(data)
	newDict, err := c.CopyDict(oldDict)
	if err != nil {
		return nil, err
	}
	newData := new(T)
	err = DecodeDict(c.r, newData, newDict)
	if err != nil {
		return nil, err
	}
	return newData, nil
}
