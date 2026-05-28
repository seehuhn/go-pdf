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

import (
	"bytes"
	"errors"
	"io"
)

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
		dict, err := c.copyStreamDict(x.Dict)
		if err != nil {
			return nil, err
		}
		res := &Stream{
			Dict:   dict,
			data:   x.data,
			start:  x.start,
			length: x.length,
		}
		// Decide whether the source's on-disk bytes need decryption
		// before being installed in the destination.  For /Identity
		// (and unencrypted sources) the bytes are already plaintext
		// and can be reused in place; for the default StmF recipe we
		// must strip the source encryption so the destination writer
		// can apply its own; non-Identity /Crypt CFs are not yet
		// implemented.
		recipe, err := streamCryptRecipe(c.r, x)
		if err != nil {
			return nil, err
		}
		switch recipe {
		case cryptNone, cryptIdentity:
			// reuse x.data verbatim
		case cryptDefault:
			rc, err := RawStreamReader(c.r, x)
			if err != nil {
				return nil, err
			}
			raw, err := io.ReadAll(rc)
			if closeErr := rc.Close(); err == nil {
				err = closeErr
			}
			if err != nil {
				return nil, err
			}
			res.data = bytes.NewReader(raw)
			res.start = 0
			res.length = int64(len(raw))
			// remove stale Length so the writer recomputes it
			delete(res.Dict, "Length")
		case cryptUnsupportedCF:
			return nil, errors.New(
				"copying streams with non-Identity /Crypt filter is not yet supported")
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

// copyStreamDict copies a stream's dictionary, inlining /Filter and
// /DecodeParms so that the destination dict carries direct values
// rather than references into the destination file.  This keeps the
// filter chain self-describing for downstream consumers (notably the
// cheap /Crypt probe in [Writer.OpenStream]) and matches the canonical
// PDF form used by all writers in the library.
func (c *Copier) copyStreamDict(src Dict) (Dict, error) {
	res, err := c.CopyDict(src)
	if err != nil {
		return nil, err
	}
	for _, key := range []Name{"Filter", "DecodeParms"} {
		val, ok := src[key]
		if !ok {
			continue
		}
		inlined, err := inlineFilterRefs(c.r, val)
		if err != nil {
			return nil, err
		}
		repl, err := c.Copy(inlined)
		if err != nil {
			return nil, err
		}
		res[key] = repl
	}
	return res, nil
}

// inlineFilterRefs resolves indirect references in a /Filter or
// /DecodeParms entry at the top level and, for arrays, at the element
// level.  The returned value contains only direct entries at those
// levels; nested references inside dictionary entries are left alone
// (the caller's [Copier.Copy] handles them).
func inlineFilterRefs(r Getter, val Object) (Native, error) {
	resolved, err := Resolve(r, val)
	if err != nil {
		return nil, err
	}
	arr, ok := resolved.(Array)
	if !ok {
		return resolved, nil
	}
	out := make(Array, len(arr))
	for i, v := range arr {
		elem, err := Resolve(r, v)
		if err != nil {
			return nil, err
		}
		out[i] = elem
	}
	return out, nil
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
	if IsReadError(err) {
		return 0, err
	}
	// a reference to a malformed or undefined source object resolves to
	// null (PDF 2.0, 7.3.10); leave val nil and copy null in its place
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
