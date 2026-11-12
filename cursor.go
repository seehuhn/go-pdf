// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
// objects with [Cursor.Decode], which threads the path and shares the cache.
type Cursor struct {
	x    *Extractor
	path *CycleCheck
}

// NewCursor returns a root cursor that reads from r.
//
// If r is a *Writer, the returned Cursor's Extractor must not be used from
// more than one goroutine: decoding records provenance on the Writer, which
// is not safe for concurrent use.
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

func (c Cursor) cast[T Native](obj Object) (T, error) {
	resolved, err := c.resolve(obj)
	if err != nil {
		var zero T
		return zero, err
	}
	return as[T](resolved)
}

// Integer resolves any indirect reference and returns the object as an Integer.
// Real values are rounded to the nearest integer; a nil object or any other
// type yields a [MalformedFileError].
func (c Cursor) Integer(obj Object) (Integer, error) {
	resolved, err := c.resolve(obj)
	if err != nil {
		return 0, err
	}
	return asInteger(resolved)
}

// Number resolves any indirect reference and returns the object as a float64.
// A nil object or a non-numeric type yields a [MalformedFileError].
func (c Cursor) Number(obj Object) (float64, error) {
	resolved, err := c.resolve(obj)
	if err != nil {
		return 0, err
	}
	switch x := resolved.(type) {
	case Integer:
		return float64(x), nil
	case Real:
		return float64(x), nil
	case nil:
		return 0, &MalformedFileError{Err: errNoNumber}
	default:
		return 0, &MalformedFileError{
			Err: fmt.Errorf("expected Number but got %T", resolved),
		}
	}
}

// Array resolves any indirect reference and returns the object as an Array.
// A nil object yields nil.
func (c Cursor) Array(obj Object) (Array, error) { return c.cast[Array](obj) }

// Boolean resolves any indirect reference and returns the object as a Boolean.
// A nil object yields false.
func (c Cursor) Boolean(obj Object) (Boolean, error) { return c.cast[Boolean](obj) }

// OptionalBoolean resolves any indirect reference and returns the value of
// an optional boolean entry.  The entry counts as absent (ok is false) if obj
// is nil, resolves to null, is not a Boolean, or cannot be read because the
// file is malformed.  Only errors other than [MalformedFileError] are
// returned.
//
// Use this instead of [Cursor.Boolean] where an absent entry must be told
// apart from false, for example when the default value is true.
func (c Cursor) OptionalBoolean(obj Object) (v, ok bool, err error) {
	resolved, err := Optional(c.resolve(obj))
	if err != nil {
		return false, false, err
	}
	b, ok := resolved.(Boolean)
	return bool(b), ok, nil
}

// Dict resolves any indirect reference and returns the object as a Dict.
// A nil object yields nil.
func (c Cursor) Dict(obj Object) (Dict, error) { return c.cast[Dict](obj) }

// Name resolves any indirect reference and returns the object as a Name.
// A nil object yields the empty Name.
func (c Cursor) Name(obj Object) (Name, error) { return c.cast[Name](obj) }

// Stream resolves any indirect reference and returns the object as a stream.
// A nil object yields nil.
func (c Cursor) Stream(obj Object) (*Stream, error) { return c.cast[*Stream](obj) }

// String resolves any indirect reference and returns the object as a String.
// A nil object yields the empty String.
func (c Cursor) String(obj Object) (String, error) { return c.cast[String](obj) }

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
// wantType. A missing "Type" entry is accepted, as is one which is not a name:
// such an entry says nothing about the type, and the context the dictionary
// was reached through already does.
func (c Cursor) CheckDictType(dict Dict, wantType Name) error {
	haveType, err := Optional(c.Name(dict["Type"]))
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
		return nil, &MalformedFileError{Err: errNoRectangle}
	}
	values, err := c.FloatArray(a)
	if err != nil {
		return nil, err
	}
	if len(values) != 4 {
		return nil, &MalformedFileError{Err: errNoRectangle}
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
// the given function.  The decoded value is cached for the object the
// references lead to, so decoding any reference to the same object again
// returns the same Go value.
//
// The decode function receives a Cursor positioned at the resolved object, the
// resolved object itself, and a flag that is true when obj was a direct
// (non-reference) object.
//
// If c reads from a *Writer, Decode records provenance on it; an Extractor
// whose Getter is a *Writer must not be used from more than one goroutine.
func (c Cursor) Decode[T any](obj Object, decode func(Cursor, Object, bool) (T, error)) (T, error) {
	var zero T
	x := c.x
	path := c.path
	tp := reflect.TypeFor[T]()

	// The cache is keyed by the reference of the object itself, i.e. the
	// last reference of a chain such as "1 0 obj 2 0 R endobj".  Keying by
	// every reference followed would let decoders that start at different
	// points of a chain publish different values for one object.
	var last Reference
	followed := false
	for {
		ref, ok := obj.(Reference)
		if !ok {
			break
		}
		// a hit means that ref names the object itself, as only those are cached
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
		last, followed = ref, true

		obj, err = x.R.Get(ref, true)
		if err != nil {
			return zero, err
		}
	}

	res, err := decode(Cursor{x: x, path: path}, obj, !followed)
	if err != nil {
		return zero, err
	}

	// adopt a concurrent decoder's result on a race, so that callers share
	// one value (see cacheStoreOrLoad)
	if followed {
		res, _ = x.cacheStoreOrLoad(extractorKey{ref: last, tp: tp}, res).(T)
		// a value decoded through a Writer stands for the object it was
		// read from
		if w, ok := x.R.(*Writer); ok {
			w.recordOrigin(res, last)
		}
	}

	return res, nil
}

// DecodeOptional is like [Cursor.Decode] but treats a nil result as acceptable rather
// than as an error.
func (c Cursor) DecodeOptional[T any](obj Object, decode func(Cursor, Object, bool) (T, error)) (T, error) {
	return Optional(c.Decode(obj, decode))
}

// DecodeExclusive is like [Cursor.Decode] but runs the decode function at most
// once per object: concurrent callers for the same object wait for the first
// to finish and share its result, even if they reach the object through
// different references.
//
// Use this ONLY for decodes that cannot participate in a cross-goroutine
// reference cycle — that is, a document "sink" such as the annotation list of
// a page, whose decode never waits on an object that might be waiting on it.
// For anything that can be mutually referential, use [Cursor.Decode], whose
// load-or-store publishing never waits and so cannot deadlock.
func (c Cursor) DecodeExclusive[T any](obj Object, decode func(Cursor, Object, bool) (T, error)) (T, error) {
	ref, ok := obj.(Reference)
	if !ok {
		return decode(c, obj, true) // direct object: not shared
	}
	key := extractorKey{ref: ref, tp: reflect.TypeFor[T]()}
	if v, ok := c.x.cacheGet(key); ok {
		// nil interface result
		r, _ := v.(T)
		return r, nil
	}

	// single-flight on the object itself, as the cache does
	_, path, err := resolvePath(c.x.R, c.path, ref, true)
	if err != nil {
		var zero T
		return zero, err
	}
	key.ref = path.Ref
	return exclusive(c.x, key, func() (T, error) {
		return Cursor{x: c.x, path: path.Parent}.Decode(path.Ref, decode)
	})
}

// Once returns the result of f, running f at most once per extractor and
// result type T.  Concurrent callers wait for the first and share its
// result.  A call which fails is not cached, so the next caller runs f again.
//
// Once holds a document-wide value which no object reference identifies, for
// example the document's interactive form, whose dictionary may be stored
// directly in the document catalog.  The result type is the key, so T should
// be a type private to the caller.  As for [Cursor.DecodeExclusive], f must
// not wait on a decode that might be waiting on it.
func (x *Extractor) Once[T any](f func(Cursor) (T, error)) (T, error) {
	// object number 0 never names an object, so no decode uses this key
	key := extractorKey{tp: reflect.TypeFor[T]()}
	return exclusive(x, key, func() (T, error) {
		res, err := f(CursorAt(x, nil))
		if err == nil {
			x.mu.Lock()
			x.cache[key] = res
			x.mu.Unlock()
		}
		return res, err
	})
}

// exclusive returns the value cached under key, or runs decode to obtain it.
// Concurrent callers for the same key wait for the first to finish and share
// its result.  The decode function is responsible for caching its result.
func exclusive[T any](x *Extractor, key extractorKey, decode func() (T, error)) (T, error) {
	var zero T

	x.mu.Lock()
	if v, ok := x.cache[key]; ok {
		x.mu.Unlock()
		// nil interface result
		r, _ := v.(T)
		return r, nil
	}
	if p, ok := x.wip[key]; ok {
		x.mu.Unlock()
		<-p.done
		if p.err != nil {
			return zero, p.err
		}
		// nil interface result
		r, _ := p.val.(T)
		return r, nil
	}
	p := &pending{done: make(chan struct{})}
	x.wip[key] = p
	x.mu.Unlock()

	res, err := decode()

	x.mu.Lock()
	p.val, p.err = res, err
	delete(x.wip, key)
	x.mu.Unlock()
	close(p.done)

	return res, err
}
