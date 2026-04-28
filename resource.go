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
	"sync"
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
	// The return value is the PDF representation of the object.
	// If the object is embedded in the PDF file, this may be a [Reference].
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

// EmbedderInfo describes runtime properties of the file being written,
// visible to Embedders.
type EmbedderInfo struct {
	// DocumentEncrypted reports whether the PDF being written has
	// document-level encryption configured.
	DocumentEncrypted bool

	// MetadataEncrypted reports whether the document-level XMP metadata
	// stream would be encrypted by the document encryption.  This is true
	// only when DocumentEncrypted is true and /EncryptMetadata is true (the
	// PDF default).
	MetadataEncrypted bool
}

// GetInfo returns information about the file being written.
func (e *EmbedHelper) GetInfo() EmbedderInfo {
	enc := e.rm.Out.w.enc
	if enc == nil {
		return EmbedderInfo{}
	}
	return EmbedderInfo{
		DocumentEncrypted: true,
		MetadataEncrypted: !enc.sec.unencryptedMetadata,
	}
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

// Store encodes an Encoder object and stores it in the PDF file.
//
// If the encoder was previously stored (via Store or StoreAt), the cached
// reference is returned without encoding again.
func (rm *ResourceManager) Store(enc Encoder) (Reference, error) {
	if ref, ok := rm.embedded[enc].(Reference); ok {
		return ref, nil
	}
	ref := rm.Out.Alloc()
	if err := rm.StoreAt(ref, enc); err != nil {
		return 0, err
	}
	return ref, nil
}

// StoreAt encodes an Encoder object and stores it at a specific reference.
//
// This is useful when references must be allocated before encoding, such as
// when two objects need to reference each other (e.g., popup and text annotations).
//
// Returns an error if enc.Encode returns a Reference, since that would conflict
// with the explicitly provided reference.
func (rm *ResourceManager) StoreAt(ref Reference, enc Encoder) error {
	native, err := enc.Encode(rm)
	if err != nil {
		return err
	}
	if _, isRef := native.(Reference); isRef {
		return errors.New("cannot use StoreAt: Encode returned a Reference")
	}
	if err := rm.Out.Put(ref, native); err != nil {
		return err
	}
	rm.embedded[enc] = ref
	return nil
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

var ErrCycle = errors.New("cycle in recursive structure")

// CycleCheck tracks which references are currently being resolved on the call
// stack, forming an immutable linked list. Each level of recursion extends
// the list by prepending a node. This enables cycle detection without shared
// mutable state, making it safe for concurrent use.
type CycleCheck struct {
	Ref    Reference
	Parent *CycleCheck
}

// Seen reports whether ref is already on the path.
func (p *CycleCheck) Seen(ref Reference) bool {
	for n := p; n != nil; n = n.Parent {
		if n.Ref == ref {
			return true
		}
	}
	return false
}

// Extractor caches extracted PDF objects to ensure that extracting the same
// reference multiple times returns the same Go object.
//
// The Extractor is safe for concurrent use from multiple goroutines.
type Extractor struct {
	R     Getter
	mu    sync.Mutex
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

func (x *Extractor) cacheGet(key extractorKey) (any, bool) {
	x.mu.Lock()
	v, ok := x.cache[key]
	x.mu.Unlock()
	return v, ok
}

func (x *Extractor) cachePut(key extractorKey, val any) {
	x.mu.Lock()
	x.cache[key] = val
	x.mu.Unlock()
}

// ExtractorGet resolves indirect references and extracts a typed object.
// The path parameter tracks which references are being resolved on the current
// call stack to detect cycles.
func ExtractorGet[T any](x *Extractor, path *CycleCheck, obj Object, extract func(*Extractor, *CycleCheck, Object, bool) (T, error)) (T, error) {
	var zero T
	tp := reflect.TypeFor[T]()

	var refs []Reference
	for {
		ref, ok := obj.(Reference)
		if !ok {
			break
		}
		key := extractorKey{ref: ref, tp: tp}

		if v, ok := x.cacheGet(key); ok {
			return v.(T), nil
		}

		// check for cycle
		if path.Seen(ref) {
			return zero, &MalformedFileError{
				Err: ErrCycle,
				Loc: []string{"object " + ref.String()},
			}
		}

		refs = append(refs, ref)
		path = &CycleCheck{Ref: ref, Parent: path}

		var err error
		obj, err = x.R.Get(ref, true)
		if err != nil {
			return zero, err
		}
	}

	isDirect := len(refs) == 0
	res, err := extract(x, path, obj, isDirect)
	if err != nil {
		return zero, err
	}

	// cache under all refs
	for _, ref := range refs {
		key := extractorKey{ref: ref, tp: tp}
		x.cachePut(key, res)
	}

	return res, nil
}

// ExtractorGetOptional is like [ExtractorGet] but treats a nil result as
// acceptable rather than as an error.
func ExtractorGetOptional[T any](x *Extractor, path *CycleCheck, obj Object, extract func(*Extractor, *CycleCheck, Object, bool) (T, error)) (T, error) {
	return Optional(ExtractorGet(x, path, obj, extract))
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
func (x *Extractor) Resolve(path *CycleCheck, obj Object) (Native, error) {
	if obj == nil {
		return nil, nil
	}

	ref, isReference := obj.(Reference)
	if !isReference {
		return obj.AsPDF(0), nil
	}

	for {
		if path.Seen(ref) {
			return nil, &MalformedFileError{
				Err: ErrCycle,
				Loc: []string{"object " + ref.String()},
			}
		}

		path = &CycleCheck{Ref: ref, Parent: path}

		next, err := x.R.Get(ref, true)
		if err != nil {
			return nil, err
		}

		if ref, isReference = next.(Reference); !isReference {
			return next, nil
		}
	}
}

// DeepResolve recursively resolves all references in a PDF object tree.
// References are resolved through the extractor, and the values within
// Dicts and Arrays are resolved recursively. The returned object contains
// no References.
func (x *Extractor) DeepResolve(obj Object) (Native, error) {
	if obj == nil {
		return nil, nil
	}
	native := obj.AsPDF(0)
	if ref, ok := native.(Reference); ok {
		resolved, err := x.Resolve(nil, ref)
		if err != nil {
			return nil, err
		}
		native = resolved
	}
	switch v := native.(type) {
	case Dict:
		res := make(Dict, len(v))
		for k, val := range v {
			r, err := x.DeepResolve(val)
			if err != nil {
				return nil, err
			}
			res[k] = r
		}
		return res, nil
	case Array:
		res := make(Array, len(v))
		for i, val := range v {
			r, err := x.DeepResolve(val)
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

func extractorResolveAndCast[T Native](x *Extractor, path *CycleCheck, obj Object) (T, error) {
	var zero T
	resolved, err := x.Resolve(path, obj)
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
func (x *Extractor) GetArray(path *CycleCheck, obj Object) (Array, error) {
	return extractorResolveAndCast[Array](x, path, obj)
}

// GetBoolean resolves any indirect reference and returns the object as a Boolean.
// If obj is nil, the function returns false, nil.
func (x *Extractor) GetBoolean(path *CycleCheck, obj Object) (Boolean, error) {
	return extractorResolveAndCast[Boolean](x, path, obj)
}

// GetDict resolves any indirect reference and returns the object as a Dict.
// If obj is nil, the function returns nil, nil.
func (x *Extractor) GetDict(path *CycleCheck, obj Object) (Dict, error) {
	return extractorResolveAndCast[Dict](x, path, obj)
}

// GetDictTyped resolves any indirect reference and checks that the resulting
// object is a dictionary. The function also checks that the "Type" entry of
// the dictionary, if set, is equal to the given type.
//
// If the object is nil, the function returns nil, nil.
func (x *Extractor) GetDictTyped(path *CycleCheck, obj Object, tp Name) (Dict, error) {
	dict, err := x.GetDict(path, obj)
	if dict == nil || err != nil {
		return nil, err
	}

	haveType, err := x.GetName(path, dict["Type"])
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
func (x *Extractor) GetName(path *CycleCheck, obj Object) (Name, error) {
	return extractorResolveAndCast[Name](x, path, obj)
}

// GetReal resolves any indirect reference and returns the object as a Real.
// If obj is nil, the function returns 0, nil.
func (x *Extractor) GetReal(path *CycleCheck, obj Object) (Real, error) {
	return extractorResolveAndCast[Real](x, path, obj)
}

// GetStream resolves any indirect reference and returns the object as a Stream.
// If obj is nil, the function returns nil, nil.
func (x *Extractor) GetStream(path *CycleCheck, obj Object) (*Stream, error) {
	return extractorResolveAndCast[*Stream](x, path, obj)
}

// GetString resolves any indirect reference and returns the object as a String.
// If obj is nil, the function returns "", nil.
func (x *Extractor) GetString(path *CycleCheck, obj Object) (String, error) {
	return extractorResolveAndCast[String](x, path, obj)
}

// GetInteger resolves any indirect reference and returns the object as an Integer.
// If obj is nil, the function returns 0, nil.
// Integers are returned as is.
// Floating point values are silently rounded to the nearest integer.
// All other object types result in an error.
func (x *Extractor) GetInteger(path *CycleCheck, obj Object) (Integer, error) {
	resolved, err := x.Resolve(path, obj)
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
func (x *Extractor) GetNumber(path *CycleCheck, obj Object) (float64, error) {
	resolved, err := x.Resolve(path, obj)
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
