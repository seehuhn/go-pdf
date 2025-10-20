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
	"errors"
	"fmt"
	"math"
	"reflect"
)

// Encoder represents a PDF object which is tied to a specific PDF file.
type Encoder interface {
	// Encode converts the Go representation of the object into the PDF
	// representation.
	Encode(rm *ResourceManager) (Native, error)
}

// Embedder represents a PDF resource (a font, image, pattern, etc.) which has
// not yet been associated with a specific PDF file.
//
// There are two additional constraints a type must satisfy to be used as an
// Embedder, in addition to implementing the Embedder interface:
//  1. The type must be comparable, so that it can be used as a key in a map.
//  2. The type must be independent of any PDF file.  For example, it must
//     not contain pdf.Reference values, neither directly nor indirectly.
type Embedder interface {
	// Embed converts the Go representation of the object into a PDF object,
	// corresponding to the PDF version of the output file.
	//
	// The first return value is the PDF representation of the object.
	// If the object is embedded in the PDF file, this may be a reference.
	Embed(e *EmbedHelper) (Native, error)
}

type EmbedHelper struct {
	rm      *ResourceManager
	copiers map[*Extractor]*Copier
	refs    []Reference
}

func (e *EmbedHelper) Alloc() Reference {
	return e.rm.Out.Alloc()
}

func (e *EmbedHelper) AllocSelf() Reference {
	k := len(e.refs)
	if e.refs[k-1] == 0 {
		e.refs[k-1] = e.rm.Out.Alloc()
	}
	return e.refs[k-1]
}

func (e *EmbedHelper) Out() *Writer {
	return e.rm.Out
}

// TODO(voss): remove this once it is no longer needed.
//
// Deprecated: this will go away.
func (e *EmbedHelper) GetRM() *ResourceManager {
	return e.rm
}

func (e *EmbedHelper) Defer(fn func(*EmbedHelper) error) {
	e.rm.deferred = append(e.rm.deferred, fn)
}

func (e *EmbedHelper) Embed(r Embedder) (Native, error) {
	return e.EmbedAt(0, r)
}

func (e *EmbedHelper) EmbedAt(ref Reference, r Embedder) (Native, error) {
	if existing, ok := e.rm.embedded[r]; ok {
		return existing, nil
	}
	if e.rm.isClosed {
		return nil, errors.New("resource manager is already closed")
	}

	k := len(e.refs)
	e.refs = append(e.refs, ref)
	val, err := r.Embed(e)
	ref = e.refs[k]
	e.refs = e.refs[:k]
	if err != nil {
		return nil, fmt.Errorf("failed to embed resource: %w", err)
	}

	if ref != 0 && val != ref {
		panic("wrong reference")
	}

	e.rm.embedded[r] = val

	return val, nil
}

func (e *EmbedHelper) CopierFrom(x *Extractor) *Copier {
	if c, ok := e.copiers[x]; ok {
		return c
	}
	c := NewCopier(e.rm.Out, x.R)
	e.copiers[x] = c
	return c
}

func EmbedHelperEmbedFunc[T any](e *EmbedHelper, f func(*EmbedHelper, T) (Native, error), obj T) (Native, error) {
	if existing, ok := e.rm.embedded[obj]; ok {
		return existing, nil
	}
	if e.rm.isClosed {
		return nil, errors.New("resource manager is already closed")
	}

	val, err := f(e, obj)
	if err != nil {
		return nil, fmt.Errorf("failed to embed resource: %w", err)
	}

	e.rm.embedded[obj] = val

	return val, nil
}

// ResourceManager helps to avoid duplicate resources in a PDF file.
// It is used to embed object implementing the [Embedder] interface.
// Each such object is embedded only once.
//
// Use the [ResourceManagerEmbed] function to embed resources.
//
// The ResourceManager must be closed with the [Close] method before the PDF
// file is closed.
type ResourceManager struct {
	Out      *Writer
	embedded map[any]Native
	deferred []func(*EmbedHelper) error
	isClosed bool
}

// NewResourceManager creates a new ResourceManager.
func NewResourceManager(w *Writer) *ResourceManager {
	return &ResourceManager{
		Out:      w,
		embedded: make(map[any]Native),
	}
}

// Embed embeds a resource in the PDF file.
//
// If the resource is already present in the file, the existing resource is
// returned.
func (rm *ResourceManager) Embed(r Embedder) (Native, error) {
	cc := make(map[*Extractor]*Copier)
	e := &EmbedHelper{rm: rm, copiers: cc}
	return e.Embed(r)
}

// ResourceManagerEmbedFunc embeds a resource using a function-driven approach.
//
// This function provides an alternative to the Embedder interface when you need
// to embed objects that don't implement the Embedder interface, or when you need
// more control over the embedding process.  The type T must be "comparable".
//
// If the resource is already present in the file, the existing resource is
// returned without calling the embed function again.
//
// The function f must return the PDF representation of the embedded object.
func ResourceManagerEmbedFunc[T any](rm *ResourceManager, f func(*EmbedHelper, T) (Native, error), obj T) (Native, error) {
	e := &EmbedHelper{rm: rm}
	return EmbedHelperEmbedFunc(e, f, obj)
}

// Close runs all defered calls registered with [EmbedHelper.Defer].
//
// After Close has been called, the resource manager can no longer be used.
func (rm *ResourceManager) Close() error {
	if rm.isClosed {
		return nil
	}

	for len(rm.deferred) > 0 {
		fn := rm.deferred[0]
		k := copy(rm.deferred, rm.deferred[1:])
		rm.deferred = rm.deferred[:k]

		e := &EmbedHelper{rm: rm}
		if err := fn(e); err != nil {
			return err
		}
	}

	rm.isClosed = true
	return nil
}

// CycleChecker detects circular references in PDF object structures to prevent
// infinite recursion during object traversal. It maintains a set of visited
// references and returns an error when a cycle is detected.
//
// CycleChecker is particularly useful when reading complex PDF structures like
// nested functions, patterns, or other objects that may reference each other.
type CycleChecker struct {
	seen map[Reference]bool
}

// NewCycleChecker creates a new CycleChecker with an empty set of seen references.
func NewCycleChecker() *CycleChecker {
	return &CycleChecker{seen: make(map[Reference]bool)}
}

// Check examines the given PDF object for circular references. If the object
// is not a reference (i.e., it's a direct value), Check returns nil immediately.
// If the object is a reference that has already been seen by this CycleChecker,
// Check returns ErrCycle0. Otherwise, Check marks the reference as seen and
// returns nil.
//
// This method should be called before recursively processing any PDF object
// that might contain references to other objects.
func (s *CycleChecker) Check(obj Object) error {
	ref, ok := obj.(Reference)
	if !ok {
		return nil
	}
	if s.seen[ref] {
		return &MalformedFileError{Err: ErrCycle}
	}
	s.seen[ref] = true
	return nil
}

var ErrCycle = errors.New("cycle in recursive structure")

// Extractor caches extracted PDF objects to ensure that extracting the same
// reference multiple times returns the same Go object. It also detects cycles
// in PDF object structures to prevent infinite recursion.
type Extractor struct {
	R          Getter
	IsIndirect bool
	cache      map[extractorKey]any
	path       map[Reference]bool
}

type extractorKey struct {
	ref Reference
	tp  reflect.Type
}

// NewExtractor creates a new Extractor using the given Getter to read PDF
// objects.
func NewExtractor(r Getter) *Extractor {
	return &Extractor{
		R:     r,
		cache: make(map[extractorKey]any),
		path:  make(map[Reference]bool),
	}
}

func ExtractorGet[T any](x *Extractor, obj Object, extract func(*Extractor, Object) (T, error)) (T, error) {
	var zero T
	tp := reflect.TypeFor[T]()

	var refs []Reference
	for {
		ref, ok := obj.(Reference)
		if !ok {
			break
		}
		key := extractorKey{ref: ref, tp: tp}

		if v, ok := x.cache[key]; ok {
			return v.(T), nil
		}

		// check for cycle
		if x.path[ref] {
			return zero, &MalformedFileError{
				Err: ErrCycle,
				Loc: []string{"object " + ref.String()},
			}
		}

		refs = append(refs, ref)
		x.path[ref] = true

		var err error
		obj, err = x.R.Get(ref, true)
		if err != nil {
			return zero, err
		}
	}

	x.IsIndirect = len(refs) > 0
	res, err := extract(x, obj)

	// cleanup path
	for _, ref := range refs {
		delete(x.path, ref)
	}

	if err != nil {
		return zero, err
	}

	// cache under all refs
	for _, ref := range refs {
		key := extractorKey{ref: ref, tp: tp}
		x.cache[key] = res
	}

	return res, nil
}

func ExtractorGetOptional[T Embedder](x *Extractor, obj Object, extract func(*Extractor, Object) (T, error)) (T, error) {
	return Optional(ExtractorGet(x, obj, extract))
}

// Resolve resolves references to indirect objects with cycle detection.
//
// If obj is a [Reference], the function reads the corresponding object from
// the file and returns the result. If obj is not a [Reference], it is
// returned unchanged. The function recursively follows chains of references
// until it resolves to a non-reference object.
//
// If a reference loop is encountered, the function returns an error of type
// [MalformedFileError].
func (x *Extractor) Resolve(obj Object) (Native, error) {
	if obj == nil {
		return nil, nil
	}

	ref, isReference := obj.(Reference)
	if !isReference {
		return obj.AsPDF(0), nil
	}

	var refs []Reference
	var result Native
	var err error

	for {
		if x.path[ref] {
			err = &MalformedFileError{
				Err: ErrCycle,
				Loc: []string{"object " + ref.String()},
			}
			break
		}

		refs = append(refs, ref)
		x.path[ref] = true

		next, getErr := x.R.Get(ref, true)
		if getErr != nil {
			err = getErr
			break
		}

		if ref, isReference = next.(Reference); !isReference {
			result = next
			break
		}
	}

	for _, r := range refs {
		delete(x.path, r)
	}

	return result, err
}

func extractorResolveAndCast[T Native](x *Extractor, obj Object) (T, error) {
	var zero T
	resolved, err := x.Resolve(obj)
	if err != nil {
		return zero, err
	}

	if resolved == nil {
		return zero, nil
	}

	result, ok := resolved.(T)
	if !ok {
		return zero, &MalformedFileError{
			Err: fmt.Errorf("expected %T but got %T", zero, resolved),
		}
	}

	return result, nil
}

// GetArray resolves any indirect reference and returns the object as an Array.
// If obj is nil, the function returns nil, nil.
func (x *Extractor) GetArray(obj Object) (Array, error) {
	return extractorResolveAndCast[Array](x, obj)
}

// GetBoolean resolves any indirect reference and returns the object as a Boolean.
// If obj is nil, the function returns false, nil.
func (x *Extractor) GetBoolean(obj Object) (Boolean, error) {
	return extractorResolveAndCast[Boolean](x, obj)
}

// GetDict resolves any indirect reference and returns the object as a Dict.
// If obj is nil, the function returns nil, nil.
func (x *Extractor) GetDict(obj Object) (Dict, error) {
	return extractorResolveAndCast[Dict](x, obj)
}

// GetDictTyped resolves any indirect reference and checks that the resulting
// object is a dictionary. The function also checks that the "Type" entry of
// the dictionary, if set, is equal to the given type.
//
// If the object is nil, the function returns nil, nil.
func (x *Extractor) GetDictTyped(obj Object, tp Name) (Dict, error) {
	dict, err := x.GetDict(obj)
	if dict == nil || err != nil {
		return nil, err
	}

	haveType, err := x.GetName(dict["Type"])
	if err != nil {
		return nil, err
	}
	if haveType != tp && haveType != "" {
		return nil, &MalformedFileError{
			Err: fmt.Errorf("expected dict type %q, got %q", tp, haveType),
		}
	}

	return dict, nil
}

// GetName resolves any indirect reference and returns the object as a Name.
// If obj is nil, the function returns "", nil.
func (x *Extractor) GetName(obj Object) (Name, error) {
	return extractorResolveAndCast[Name](x, obj)
}

// GetReal resolves any indirect reference and returns the object as a Real.
// If obj is nil, the function returns 0, nil.
func (x *Extractor) GetReal(obj Object) (Real, error) {
	return extractorResolveAndCast[Real](x, obj)
}

// GetStream resolves any indirect reference and returns the object as a Stream.
// If obj is nil, the function returns nil, nil.
func (x *Extractor) GetStream(obj Object) (*Stream, error) {
	return extractorResolveAndCast[*Stream](x, obj)
}

// GetString resolves any indirect reference and returns the object as a String.
// If obj is nil, the function returns "", nil.
func (x *Extractor) GetString(obj Object) (String, error) {
	return extractorResolveAndCast[String](x, obj)
}

// GetInteger resolves any indirect reference and returns the object as an Integer.
// If obj is nil, the function returns 0, nil.
// Integers are returned as is.
// Floating point values are silently rounded to the nearest integer.
// All other object types result in an error.
func (x *Extractor) GetInteger(obj Object) (Integer, error) {
	resolved, err := x.Resolve(obj)
	if resolved == nil {
		return 0, err
	}

	switch val := resolved.(type) {
	case Integer:
		return val, nil
	case Real:
		return Integer(math.Round(float64(val))), nil
	default:
		return 0, &MalformedFileError{
			Err: fmt.Errorf("expected Integer but got %T", resolved),
		}
	}
}

// GetNumber resolves any indirect reference and returns the object as a float64.
// If obj is nil, the function returns 0, nil.
// Both Integer and Real values are converted to float64.
// All other object types result in an error.
func (x *Extractor) GetNumber(obj Object) (float64, error) {
	resolved, err := x.Resolve(obj)
	if resolved == nil {
		return 0, err
	}

	switch val := resolved.(type) {
	case Integer:
		return float64(val), nil
	case Real:
		return float64(val), nil
	default:
		return 0, &MalformedFileError{
			Err: fmt.Errorf("expected Number but got %T", resolved),
		}
	}
}
