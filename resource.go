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
)

// Embedder represents a PDF resource (a font, image, pattern, etc.) which has
// not yet been associated with a specific PDF file.
type Embedder[T any] interface {
	// Embed converts the Go representation of the object into a PDF object,
	// corresponding to the PDF version of the output file.
	//
	// The first return value is the PDF representation of the object.
	// If the object is embedded in the PDF file, this may be a reference.
	//
	// The second return value is a Go representation of the embedded object.
	// In most cases, this value is not used and T can be set to [Unused].
	Embed(rm *ResourceManager) (Native, T, error)
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
	Out       *Writer
	embedded  map[any]embRes
	finishers []Finisher
	isClosed  bool
}

// NewResourceManager creates a new ResourceManager.
func NewResourceManager(w *Writer) *ResourceManager {
	return &ResourceManager{
		Out:      w,
		embedded: make(map[any]embRes),
	}
}

type embRes struct {
	Val Object
	Emb any
}

// Finisher is implemented by embedded objects that need to perform
// finalization work when the ResourceManager is closed.
// The Finish method is called automatically for any embedded object
// whose type implements this interface.
type Finisher interface {
	Finish(*ResourceManager) error
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
func ResourceManagerEmbed[T any](rm *ResourceManager, r Embedder[T]) (Object, T, error) {
	var zero T

	if existing, ok := rm.embedded[r]; ok {
		return existing.Val, existing.Emb.(T), nil
	}
	if rm.isClosed {
		return nil, zero, errors.New("resource manager is already closed")
	}

	val, emb, err := r.Embed(rm)
	if err != nil {
		return nil, zero, fmt.Errorf("failed to embed resource: %w", err)
	}

	rm.embedded[r] = embRes{Val: val, Emb: emb}

	if finisher, ok := any(emb).(Finisher); ok {
		rm.finishers = append(rm.finishers, finisher)
	}

	return val, emb, nil
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
func ResourceManagerEmbedFunc[T any](rm *ResourceManager, f func(*ResourceManager, T) (Object, error), obj T) (Object, error) {
	if existing, ok := rm.embedded[obj]; ok {
		return existing.Val, nil
	}
	if rm.isClosed {
		return nil, errors.New("resource manager is already closed")
	}

	val, err := f(rm, obj)
	if err != nil {
		return nil, fmt.Errorf("failed to embed resource: %w", err)
	}

	rm.embedded[obj] = embRes{Val: val}

	return val, nil
}

// Close runs the Finish methods of all embedded resources where the Go
// representation implemented the [Finisher] interface.
//
// After Close has been called, the resource manager can no longer be used.
func (rm *ResourceManager) Close() error {
	if rm.isClosed {
		return nil
	}

	for len(rm.finishers) > 0 {
		r := rm.finishers[0]
		k := copy(rm.finishers, rm.finishers[1:])
		rm.finishers = rm.finishers[:k]

		if err := r.Finish(rm); err != nil {
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
