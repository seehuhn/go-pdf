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
	"sort"
	"strings"
	"sync"
)

// Encoder represents a PDF object which is tied to a specific PDF file.
type Encoder interface {
	// Encode converts the Go representation of the object into its inline PDF
	// representation (a Dict, Array, stream, etc.).  It must not return a
	// Reference; allocating and writing the indirect object is the job of
	// [ResourceManager.Store].  An object that needs its own reference while
	// encoding (for example to let its children point back at it) obtains it
	// via [ResourceManager.GetReference].
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
	reserved map[any]bool
	deferred []func(*EmbedHelper) error
	isClosed bool
}

// NewResourceManager creates a new ResourceManager.
func NewResourceManager(w *Writer) *ResourceManager {
	return &ResourceManager{
		Out:      w,
		embedded: make(map[any]Native),
		reserved: make(map[any]bool),
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

// GetReference returns the reference associated with enc, assigning one if
// necessary.  If no reference has been assigned yet, it allocates and reserves
// one, so that a later [ResourceManager.Store] still encodes enc at that
// reference.
//
// This is useful when a reference must be obtained before encoding, such as
// when two objects need to reference each other (e.g. a widget annotation and
// its parent field).
func (rm *ResourceManager) GetReference(enc Encoder) Reference {
	if ref, ok := rm.embedded[enc].(Reference); ok {
		return ref
	}
	ref := rm.Out.Alloc()
	rm.embedded[enc] = ref
	rm.reserved[enc] = true
	return ref
}

// StoreDeferred reserves a reference for enc and arranges for enc to be encoded
// at that reference during [ResourceManager.Close], after all eagerly written
// objects. It returns the reserved reference immediately.
//
// Use it for an object that must be written only after others exist — for
// example an interactive form, which is written after the pages whose widget
// annotations it owns. Any error from encoding enc is reported by Close.
func (rm *ResourceManager) StoreDeferred(enc Encoder) Reference {
	ref := rm.GetReference(enc)
	rm.deferred = append(rm.deferred, func(*EmbedHelper) error {
		_, err := rm.Store(enc)
		return err
	})
	return ref
}

// Store encodes enc and stores it in the PDF file, returning its reference.
//
// If enc was previously stored, the cached reference is returned without
// encoding again.  If a reference was reserved for enc via
// [ResourceManager.GetReference], enc is encoded at that reference.
//
// [Encoder.Encode] must return the object's inline representation, never a
// reference; allocating and writing the object is the job of Store.  An encoder
// that returns a nil object writes nothing; Store then returns the zero
// reference, which callers treat as "unset".
func (rm *ResourceManager) Store(enc Encoder) (Reference, error) {
	ref, isSet := rm.embedded[enc].(Reference)
	if isSet && !rm.reserved[enc] {
		return ref, nil
	}

	native, err := enc.Encode(rm)
	if err != nil {
		return 0, err
	}
	if native == nil {
		// The encoder wrote nothing inline. If it reserved a reference for
		// itself via GetReference, this signals a deferred write: the object
		// will be filled in later via [ResourceManager.StoreEncoded]. Keep the
		// reservation pending and return the reserved reference so callers can
		// already refer to the object. Otherwise the encoder chose to write
		// nothing, reported as the zero reference, which callers treat as
		// "unset".
		if ref, ok := rm.embedded[enc].(Reference); ok {
			return ref, nil
		}
		return 0, nil
	}
	if _, isRef := native.(Reference); isRef {
		return 0, errors.New("encode must not return a reference")
	}

	return rm.putEncoded(enc, native)
}

// putEncoded writes native as the object for enc, at enc's reserved reference
// (from [ResourceManager.GetReference]) or a freshly allocated one, fulfilling
// any reservation.
func (rm *ResourceManager) putEncoded(enc Encoder, native Native) (Reference, error) {
	ref, isSet := rm.embedded[enc].(Reference)
	if !isSet {
		ref = rm.Out.Alloc()
		rm.embedded[enc] = ref
	}
	if err := rm.Out.Put(ref, native); err != nil {
		return 0, err
	}
	delete(rm.reserved, enc)
	return ref, nil
}

// StoreEncoded stores an already-encoded value as the object for enc, at the
// reference reserved for enc via [ResourceManager.GetReference] (or a freshly
// allocated one), and fulfils the reservation. Unlike [ResourceManager.Store]
// it does not call enc.Encode; the caller supplies the value.
//
// Use it when one object computes the contents of another that reserved its
// reference earlier — for example an interactive form writing the merged
// field/widget dictionaries whose references its widgets reserved while the
// pages were being written.
func (rm *ResourceManager) StoreEncoded(enc Encoder, obj Native) (Reference, error) {
	if obj == nil {
		return 0, errors.New("StoreEncoded requires a non-nil object")
	}
	if _, isRef := obj.(Reference); isRef {
		return 0, errors.New("StoreEncoded must not store a reference")
	}
	return rm.putEncoded(enc, obj)
}

// Close runs all deferred calls registered with [EmbedHelper.Defer] or
// [ResourceManager.StoreDeferred], then verifies that every reserved reference
// has been written.
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

	// every reference handed out by GetReference must eventually be written by
	// Store; a leftover reservation is a reference to an object that was never
	// written, i.e. a dangling reference in the output file
	if len(rm.reserved) > 0 {
		return fmt.Errorf("reference reserved but never written: %s", reservedTypeList(rm.reserved))
	}

	rm.isClosed = true
	return nil
}

// reservedTypeList returns the distinct Go types of the encoders still holding
// a reserved-but-unwritten reference, sorted for a deterministic message.
func reservedTypeList(reserved map[any]bool) string {
	seen := make(map[string]bool)
	var names []string
	for enc := range reserved {
		name := fmt.Sprintf("%T", enc)
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// ErrCycle reports that a chain of indirect references loops back on itself.
var ErrCycle = errors.New("cycle in recursive structure")

// ErrDepth reports that indirect-reference nesting exceeded
// [limits.MaxExtractDepth].
var ErrDepth = errors.New("reference nesting too deep")

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

// depth reports the number of references on the path.
func (p *CycleCheck) depth() int {
	n := 0
	for ; p != nil; p = p.Parent {
		n++
	}
	return n
}

// Extractor caches extracted PDF objects to ensure that extracting the same
// reference multiple times returns the same Go object.
//
// The Extractor is safe for concurrent use from multiple goroutines.
type Extractor struct {
	R     Getter
	mu    sync.Mutex
	cache map[extractorKey]any
	wip   map[extractorKey]*pending // decodes in progress (DecodeExclusive)
}

type extractorKey struct {
	ref Reference
	tp  reflect.Type
}

// pending is a decode that one goroutine is running while others wait on it.
type pending struct {
	done chan struct{}
	val  any
	err  error
}

// NewExtractor creates a new Extractor using the given Getter to read PDF
// objects.
func NewExtractor(r Getter) *Extractor {
	return &Extractor{
		R:     r,
		cache: make(map[extractorKey]any),
		wip:   make(map[extractorKey]*pending),
	}
}

func (x *Extractor) cacheGet(key extractorKey) (any, bool) {
	x.mu.Lock()
	v, ok := x.cache[key]
	x.mu.Unlock()
	return v, ok
}

// cacheStoreOrLoad publishes res under every reference in refs and returns res.
// If the first reference is already cached — another goroutine decoded the same
// object concurrently — it stores nothing and returns the existing value, so
// every caller ends up with one shared object. The first writer for a reference
// wins; later racers adopt its result and discard their own.
//
// Publishing this way (rather than waiting on an in-flight marker) keeps decode
// deadlock-free: two goroutines decoding mutually-referential objects never wait
// on each other, so a malformed file cannot hang the reader.
func (x *Extractor) cacheStoreOrLoad(refs []Reference, tp reflect.Type, res any) any {
	x.mu.Lock()
	defer x.mu.Unlock()
	if v, ok := x.cache[extractorKey{ref: refs[0], tp: tp}]; ok {
		return v
	}
	for _, ref := range refs {
		x.cache[extractorKey{ref: ref, tp: tp}] = res
	}
	return res
}

// StoreOrLoadPair publishes two typed views of a single PDF object — for
// example the field half and the widget half of a merged field/widget
// dictionary — under one object reference, atomically. If the reference is
// already populated (another goroutine decoded the same object), it returns the
// existing pair and stores nothing, so every caller shares one consistent pair.
//
// The two views are cached under their respective Go types, exactly as
// [Decode] would cache them, so a later Decode for either type finds
// the published value.
func StoreOrLoadPair[A, B any](x *Extractor, ref Reference, a A, b B) (A, B) {
	ka := extractorKey{ref: ref, tp: reflect.TypeFor[A]()}
	kb := extractorKey{ref: ref, tp: reflect.TypeFor[B]()}
	x.mu.Lock()
	defer x.mu.Unlock()
	if v, ok := x.cache[ka]; ok {
		a = v.(A)
	} else {
		x.cache[ka] = a
	}
	if v, ok := x.cache[kb]; ok {
		b = v.(B)
	} else {
		x.cache[kb] = b
	}
	return a, b
}
