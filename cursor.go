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

package pdf

import (
	"fmt"
	"io"
	"math"
	"os"
	"reflect"

	"seehuhn.de/go/geom/matrix"
)

// A Cursor reads typed values from a PDF file.
//
// It bundles an [Extractor] (which caches decoded objects and resolves
// references) with the current reference-resolution path used to detect cycles.
// Cursors are small values passed by value: leaf values are read with the typed
// accessor methods ([Cursor.Integer], [Cursor.Dict], ...), and nested typed
// objects with [Decode], which threads the path and shares the cache.
type Cursor struct {
	x    *Extractor
	path *CycleCheck
}

// NewCursor returns a root cursor that reads from r.
func NewCursor(r Getter) Cursor {
	return Cursor{x: NewExtractor(r)}
}

// resolve follows indirect references starting from obj, threading the cursor's
// cycle-detection path.
func (c Cursor) resolve(obj Object) (Native, error) {
	n, _, err := resolvePath(c.x.R, c.path, obj, true)
	return n, err
}

// Resolve follows indirect references starting from obj and returns the
// resolved object, threading the cursor's cycle-detection path. References to a
// null object and a nil obj both yield a nil result.
func (c Cursor) Resolve(obj Object) (Native, error) {
	return c.resolve(obj)
}

// Getter returns the underlying PDF file the cursor reads from.
func (c Cursor) Getter() Getter { return c.x.R }

func cursorCast[T Native](c Cursor, obj Object) (T, error) {
	resolved, err := c.resolve(obj)
	if err != nil {
		var zero T
		return zero, err
	}
	return as[T](resolved)
}

// Integer resolves any indirect reference and returns the object as an Integer.
// A nil object yields 0. Real values are rounded to the nearest integer; other
// types yield an error.
func (c Cursor) Integer(obj Object) (Integer, error) {
	resolved, err := c.resolve(obj)
	if err != nil {
		return 0, err
	}
	return asInteger(resolved)
}

// Number resolves any indirect reference and returns the object as a float64.
// A nil object yields 0; non-numeric types yield an error.
func (c Cursor) Number(obj Object) (float64, error) {
	resolved, err := c.resolve(obj)
	if err != nil {
		return 0, err
	}
	n, err := asNumber(resolved)
	return float64(n), err
}

// Array resolves any indirect reference and returns the object as an Array.
// A nil object yields nil.
func (c Cursor) Array(obj Object) (Array, error) { return cursorCast[Array](c, obj) }

// Boolean resolves any indirect reference and returns the object as a Boolean.
// A nil object yields false.
func (c Cursor) Boolean(obj Object) (Boolean, error) { return cursorCast[Boolean](c, obj) }

// Dict resolves any indirect reference and returns the object as a Dict.
// A nil object yields nil.
func (c Cursor) Dict(obj Object) (Dict, error) { return cursorCast[Dict](c, obj) }

// Name resolves any indirect reference and returns the object as a Name.
// A nil object yields the empty Name.
func (c Cursor) Name(obj Object) (Name, error) { return cursorCast[Name](c, obj) }

// Real resolves any indirect reference and returns the object as a Real.
// A nil object yields 0.
func (c Cursor) Real(obj Object) (Real, error) { return cursorCast[Real](c, obj) }

// Stream resolves any indirect reference and returns the object as a stream.
// A nil object yields nil.
func (c Cursor) Stream(obj Object) (*Stream, error) { return cursorCast[*Stream](c, obj) }

// String resolves any indirect reference and returns the object as a String.
// A nil object yields the empty String.
func (c Cursor) String(obj Object) (String, error) { return cursorCast[String](c, obj) }

// DictTyped resolves any indirect reference and returns the object as a Dict,
// checking that its "Type" entry, if present, equals tp.
// A nil object yields nil.
func (c Cursor) DictTyped(obj Object, tp Name) (Dict, error) {
	dict, err := c.Dict(obj)
	if dict == nil || err != nil {
		return nil, err
	}
	if err := c.CheckDictType(dict, tp); err != nil {
		return nil, err
	}
	return dict, nil
}

// CheckDictType checks that the "Type" entry of dict, if present, equals
// wantType. A missing "Type" entry is accepted.
func (c Cursor) CheckDictType(dict Dict, wantType Name) error {
	haveType, err := c.Name(dict["Type"])
	if err != nil {
		return err
	}
	if haveType != wantType && haveType != "" {
		return &MalformedFileError{
			Err: fmt.Errorf("expected dict type %q, got %q", wantType, haveType),
		}
	}
	return nil
}

// FloatArray resolves any indirect reference and returns the object as a slice
// of float64 values. A nil object yields nil.
func (c Cursor) FloatArray(obj Object) ([]float64, error) {
	array, err := c.Array(obj)
	if err != nil {
		return nil, err
	}
	if array == nil {
		return nil, nil
	}
	result := make([]float64, len(array))
	for i, item := range array {
		num, err := c.Number(item)
		if err != nil {
			return nil, fmt.Errorf("array element %d: %w", i, err)
		}
		result[i] = num
	}
	return result, nil
}

// Rectangle resolves any indirect reference and returns the object as a
// rectangle. A nil object yields nil.
func (c Cursor) Rectangle(obj Object) (*Rectangle, error) {
	if rect, ok := obj.(*Rectangle); ok {
		return rect, nil
	}
	a, err := c.Array(obj)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, nil
	}
	if len(a) != 4 {
		return nil, errNoRectangle
	}
	values, err := c.FloatArray(a)
	if err != nil {
		return nil, err
	}
	if len(values) != 4 {
		return nil, errNoRectangle
	}
	return &Rectangle{
		LLx: math.Min(values[0], values[2]),
		LLy: math.Min(values[1], values[3]),
		URx: math.Max(values[0], values[2]),
		URy: math.Max(values[1], values[3]),
	}, nil
}

// Matrix resolves any indirect reference and returns the object as a 6-element
// transformation matrix.
func (c Cursor) Matrix(obj Object) (matrix.Matrix, error) {
	a, err := c.FloatArray(obj)
	if err != nil {
		return matrix.Matrix{}, Wrap(err, "Matrix")
	}
	if len(a) != 6 {
		return matrix.Matrix{}, &MalformedFileError{
			Err: fmt.Errorf("expected 6 numbers, got %d", len(a)),
		}
	}
	var m matrix.Matrix
	copy(m[:], a)
	return m, nil
}

// Date resolves any indirect reference and returns the object as a Date.
func (c Cursor) Date(obj Object) (Date, error) {
	var zero Date
	s, err := c.String(obj)
	if err != nil {
		return zero, err
	}
	return s.AsDate()
}

// TextString resolves any indirect reference and returns the object as a
// utf-8 encoded text string.
func (c Cursor) TextString(obj Object) (TextString, error) {
	s, err := c.String(obj)
	if err != nil {
		return "", err
	}
	return s.AsTextString(), nil
}

// StreamReader resolves obj to a stream and returns a reader for its decoded
// contents. A nil obj, or one that does not resolve to a stream, yields an
// error wrapping [os.ErrNotExist].
func (c Cursor) StreamReader(obj Object) (io.ReadCloser, error) {
	stm, err := c.Stream(obj)
	if err != nil {
		return nil, err
	}
	if stm == nil {
		return nil, fmt.Errorf("no stream found: %w", os.ErrNotExist)
	}
	return DecodeStream(c.x.R, c.path, stm)
}

// ReadAll resolves obj to a stream and returns its decoded contents, failing if
// the decoded size exceeds maxBytes. A nil obj yields a nil result.
func (c Cursor) ReadAll(obj Object, maxBytes int64) ([]byte, error) {
	stm, err := c.Stream(obj)
	if err != nil || stm == nil {
		return nil, err
	}
	return ReadAll(c.x.R, c.path, stm, maxBytes)
}

// Filters returns the decoded filter chain described by the stream dictionary
// dict.
func (c Cursor) Filters(dict Dict) ([]Filter, error) {
	return GetFilters(c.x.R, c.path, dict)
}

// Version returns the PDF version of the file the cursor reads from.
func (c Cursor) Version() Version {
	return GetVersion(c.x.R)
}

// Decode resolves any indirect references in obj and decodes the result with
// the given function. The decoded value is cached under every reference that
// was followed, so decoding the same reference again returns the same Go value.
//
// The decode function receives a Cursor positioned at the resolved object, the
// resolved object itself, and a flag that is true when obj was a direct
// (non-reference) object.
func Decode[T any](c Cursor, obj Object, decode func(Cursor, Object, bool) (T, error)) (T, error) {
	var zero T
	x := c.x
	path := c.path
	tp := reflect.TypeFor[T]()

	var refs []Reference
	for {
		ref, ok := obj.(Reference)
		if !ok {
			break
		}
		key := extractorKey{ref: ref, tp: tp}
		if v, ok := x.cacheGet(key); ok {
			// a cached nil interface result (T is an interface type and the
			// decoder returned nil) is an untyped nil here; the comma-ok form
			// yields the zero value instead of panicking on the assertion
			r, _ := v.(T)
			return r, nil
		}

		// follow the reference, rejecting cycles and over-deep chains
		var err error
		path, err = path.step(ref)
		if err != nil {
			return zero, err
		}
		refs = append(refs, ref)

		obj, err = x.R.Get(ref, true)
		if err != nil {
			return zero, err
		}
	}

	isDirect := len(refs) == 0
	res, err := decode(Cursor{x: x, path: path}, obj, isDirect)
	if err != nil {
		return zero, err
	}

	// publish under all refs; adopt a concurrent decoder's result on a race so
	// callers share one object (see cacheStoreOrLoad)
	if len(refs) > 0 {
		res, _ = x.cacheStoreOrLoad(refs, tp, res).(T)
	}

	return res, nil
}

// DecodeOptional is like [Decode] but treats a nil result as acceptable rather
// than as an error.
func DecodeOptional[T any](c Cursor, obj Object, decode func(Cursor, Object, bool) (T, error)) (T, error) {
	return Optional(Decode(c, obj, decode))
}

// DecodeExclusive is like [Decode] but runs the decode function at most once
// per reference: concurrent callers for the same reference wait for the first
// to finish and share its result.
//
// Use this ONLY for decodes that cannot participate in a cross-goroutine
// reference cycle — that is, a document "sink" such as the interactive form,
// whose decode never waits on an object that might be waiting on it. For
// anything that can be mutually referential, use [Decode], whose load-or-store
// publishing never waits and so cannot deadlock.
func DecodeExclusive[T any](c Cursor, obj Object, decode func(Cursor, Object, bool) (T, error)) (T, error) {
	var zero T
	x := c.x
	ref, ok := obj.(Reference)
	if !ok {
		return decode(c, obj, true) // direct object: not shared
	}
	key := extractorKey{ref: ref, tp: reflect.TypeFor[T]()}

	x.mu.Lock()
	if v, ok := x.cache[key]; ok {
		x.mu.Unlock()
		return v.(T), nil
	}
	if p, ok := x.wip[key]; ok {
		x.mu.Unlock()
		<-p.done
		if p.err != nil {
			return zero, p.err
		}
		return p.val.(T), nil
	}
	p := &pending{done: make(chan struct{})}
	x.wip[key] = p
	x.mu.Unlock()

	res, err := Decode(c, obj, decode)

	x.mu.Lock()
	p.val, p.err = res, err
	delete(x.wip, key)
	x.mu.Unlock()
	close(p.done)

	return res, err
}
