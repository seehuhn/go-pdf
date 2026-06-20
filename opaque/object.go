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

package opaque

import (
	"errors"

	"seehuhn.de/go/pdf"
)

// Object holds an arbitrary PDF value that the library does not need to
// interpret.  It is constructed in one of two modes:
//
//   - [Extract] binds the value to its source extractor; on [Object.Embed],
//     references inside the value are translated to the destination file
//     through [pdf.Copier].
//   - [Direct] wraps an in-memory PDF value, which is written verbatim on
//     Embed.  The caller is responsible for ensuring the value contains
//     no indirect objects.
//
// Object implements [pdf.Embedder].  Two [Object.Embed] calls with the
// same *Object pointer produce a single copy of the value in the
// destination file: the resource manager's Embedder cache returns the
// same [pdf.Native] for every call, so the underlying [pdf.Copier]
// runs at most once per write.
type Object struct {
	src *pdf.Extractor // nil for in-memory values
	obj pdf.Object
}

// Extract wraps an opaque PDF value read from a file.  The extractor is
// stashed so that references inside obj can be translated to the
// destination file when [Object.Embed] is called.  obj must remain
// valid for as long as Object is used.
func Extract(x *pdf.Extractor, obj pdf.Object) *Object {
	return &Object{src: x, obj: obj}
}

// Direct wraps an in-memory PDF value.  obj must contain no indirect objects;
// on [Object.Embed] the value is written verbatim without reference
// translation.
func Direct(obj pdf.Object) *Object {
	return &Object{obj: obj}
}

// Embed implements [pdf.Embedder].
//
// For an Object built via [Extract], references in the wrapped value
// are translated to the destination file via [pdf.Copier].  For an
// Object built via [Direct], the value is written verbatim.
func (o *Object) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	native := o.obj.AsPDF(e.Out().GetOptions())
	if o.src == nil {
		return native, nil
	}
	return e.CopierFrom(o.src).Copy(native)
}

// Equal reports whether o and other denote the same PDF value.  For
// Extract-built objects, references are resolved through the source
// extractor before comparison; for Direct-built objects, the wrapped
// value is compared directly.  The comparison is structural except
// for streams and placeholders, which are compared by pointer
// identity (inherited from [pdf.Equal]).
func (o *Object) Equal(other *Object) bool {
	if o == nil || other == nil {
		return o == other
	}
	a, errA := o.resolve()
	b, errB := other.resolve()
	if errA != nil || errB != nil {
		return false
	}
	return pdf.Equal(a, b)
}

// resolve flattens all references inside the wrapped value, using the
// source extractor when present.
func (o *Object) resolve() (pdf.Native, error) {
	if o.src == nil {
		return o.obj.AsPDF(0), nil
	}
	return deepResolve(pdf.CursorAt(o.src, nil), 0, o.obj)
}

// maxDeepResolveDepth caps the recursion depth of [deepResolve],
// rejecting adversarially deep object trees before they exhaust the Go
// call stack.
const maxDeepResolveDepth = 256

// deepResolve recursively resolves all references in a PDF object tree.
// References are resolved through the extractor, and the values within
// Dicts and Arrays are resolved recursively.  The returned object
// contains no References.
//
// path threads cycle detection across the recursion so that reference
// loops that cross structural boundaries (e.g. dict A holds a reference
// to dict B which holds a reference back to A) are detected.  depth
// bounds the recursion against adversarial nesting.
func deepResolve(c pdf.Cursor, depth int, obj pdf.Object) (pdf.Native, error) {
	if obj == nil {
		return nil, nil
	}
	if depth >= maxDeepResolveDepth {
		return nil, &pdf.MalformedFileError{Err: errors.New("nesting depth exceeded")}
	}
	native := obj.AsPDF(0)
	if ref, ok := native.(pdf.Reference); ok {
		resolved, err := c.Resolve(ref)
		if err != nil {
			return nil, err
		}
		// extend the path with ref so nested resolves detect back-references
		c = pdf.CursorAt(c.Extractor(), &pdf.CycleCheck{Ref: ref, Parent: c.Path()})
		native = resolved
	}
	switch v := native.(type) {
	case pdf.Dict:
		res := make(pdf.Dict, len(v))
		for k, val := range v {
			r, err := deepResolve(c, depth+1, val)
			if err != nil {
				return nil, err
			}
			res[k] = r
		}
		return res, nil
	case pdf.Array:
		res := make(pdf.Array, len(v))
		for i, val := range v {
			r, err := deepResolve(c, depth+1, val)
			if err != nil {
				return nil, err
			}
			res[i] = r
		}
		return res, nil
	default:
		return native, nil
	}
}

// AsDirectDict checks whether the wrapped value is a PDF dictionary,
// which is a direct object and does not contain any indirect objects.
// If the wrapped value has this form, the dictionary is returned.
// Otherwise, nil is returned.
func (o *Object) AsDirectDict() pdf.Dict {
	dict, ok := o.obj.(pdf.Dict)
	if !ok || !pdf.IsDirect(dict) {
		return nil
	}
	return dict
}

// ObjectAs runs the given typed extractor against the value wrapped
// by o, using o's source extractor for cache coherency and the
// supplied path for cycle detection.  It returns an error if o was
// built via [Direct] (no source available); that case is a programming
// bug, not malformed PDF input.
//
// Once Go supports generic methods, this will become Object.As.
func ObjectAs[T any](
	o *Object,
	path *pdf.CycleCheck,
	extract func(pdf.Cursor, pdf.Object, bool) (T, error),
) (T, error) {
	if o.src == nil {
		var zero T
		return zero, errors.New("opaque: ObjectAs requires an Object built via Extract")
	}
	return pdf.Decode(pdf.CursorAt(o.src, path), o.obj, extract)
}
