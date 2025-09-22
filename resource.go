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
type Embedder[T any] interface {
	// Embed converts the Go representation of the object into a PDF object,
	// corresponding to the PDF version of the output file.
	//
	// The first return value is the PDF representation of the object.
	// If the object is embedded in the PDF file, this may be a reference.
	//
	// The second return value is a Go representation of the embedded object.
	// In most cases, this value is not used and T can be set to [Unused].
	Embed(e *EmbedHelper) (Native, T, error)
}

type EmbedHelper struct {
	rm   *ResourceManager
	refs []Reference
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

func EmbedHelperEmbed[T any](e *EmbedHelper, r Embedder[T]) (Native, T, error) {
	return EmbedHelperEmbedAt(e, 0, r)
}

func EmbedHelperEmbedAt[T any](e *EmbedHelper, ref Reference, r Embedder[T]) (Native, T, error) {
	var zero T

	if existing, ok := e.rm.embedded[r]; ok {
		return existing.Val, existing.Emb.(T), nil
	}
	if e.rm.isClosed {
		return nil, zero, errors.New("resource manager is already closed")
	}

	k := len(e.refs)
	e.refs = append(e.refs, ref)
	val, emb, err := r.Embed(e)
	ref = e.refs[k]
	e.refs = e.refs[:k]
	if err != nil {
		return nil, zero, fmt.Errorf("failed to embed resource: %w", err)
	}

	if ref != 0 && val != ref {
		panic("wrong reference")
	}

	e.rm.embedded[r] = embRes{Val: val, Emb: emb}

	return val, emb, nil
}

func EmbedHelperEmbedFunc[T any](e *EmbedHelper, f func(*EmbedHelper, T) (Native, error), obj T) (Native, error) {
	if existing, ok := e.rm.embedded[obj]; ok {
		return existing.Val, nil
	}
	if e.rm.isClosed {
		return nil, errors.New("resource manager is already closed")
	}

	val, err := f(e, obj)
	if err != nil {
		return nil, fmt.Errorf("failed to embed resource: %w", err)
	}

	e.rm.embedded[obj] = embRes{Val: val}

	return val, nil
}

// Unused is a placeholder type for the second return value of the
// Embedder.Embed method, for when no Go representation of the
// embedded object is required.
type Unused struct{}

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
	embedded map[any]embRes
	deferred []func(*EmbedHelper) error
	isClosed bool
}

// NewResourceManager creates a new ResourceManager.
func NewResourceManager(w *Writer) *ResourceManager {
	return &ResourceManager{
		Out:      w,
		embedded: make(map[any]embRes),
	}
}

type embRes struct {
	Val Native
	Emb any
}

// ResourceManagerEmbed embeds a resource in the PDF file.
//
// If the resource is already present in the file, the existing resource is
// returned.
//
// The embedded type, T, must be comparable.  If T implements [Finisher], the
// Finish() method will be called when the ResourceManager is closed.
//
// Once Go supports methods with type parameters, this function can be turned
// into a method on [ResourceManager].
func ResourceManagerEmbed[T any](rm *ResourceManager, r Embedder[T]) (Native, T, error) {
	e := &EmbedHelper{rm: rm}
	return EmbedHelperEmbed(e, r)
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

// Close runs the Finish methods of all embedded resources where the Go
// representation implemented the [Finisher] interface.
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
// Check returns ErrCycle. Otherwise, Check marks the reference as seen and
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

var ErrCycle = &MalformedFileError{
	Err: errors.New("cycle in recursive structure"),
}

// Extractor caches extracted PDF objects to ensure that extracting the same
// reference multiple times returns the same Go object.
type Extractor struct {
	R     Getter
	cache map[extractorKey]any
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
	}
}

func ExtractorGet[X any, T Embedder[X]](x *Extractor, obj Object, extract func(*Extractor, Object) (T, error)) (T, error) {
	var zero T
	tp := reflect.TypeFor[T]()

	// We need to keep the information whether the original object was a
	// reference, in order to correctly set any SingleUse fields.
	origObj := obj

	var refs []Reference
	count := 0
	for {
		ref, ok := obj.(Reference)
		if !ok {
			break
		}
		key := extractorKey{ref: ref, tp: tp}

		if v, ok := x.cache[key]; ok {
			return v.(T), nil
		}

		refs = append(refs, ref)
		count++
		if count > maxRefDepth {
			return zero, &MalformedFileError{
				Err: errors.New("too many levels of indirection"),
				Loc: []string{"object " + ref.String()},
			}
		}

		var err error
		obj, err = x.R.Get(ref, true)
		if err != nil {
			return zero, err
		}
	}

	res, err := extract(x, origObj)
	if err != nil {
		return zero, err
	}

	for _, ref := range refs {
		key := extractorKey{ref: ref, tp: tp}
		x.cache[key] = res
	}

	return res, nil
}

func ExtractorGetOptional[X any, T Embedder[X]](x *Extractor, obj Object, extract func(*Extractor, Object) (T, error)) (T, error) {
	return Optional(ExtractorGet(x, obj, extract))
}
