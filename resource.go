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
	"io"
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
	Embed(rm *ResourceManager) (Object, T, error)
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
	Out        Putter
	embedded   map[any]embRes
	needsClose []io.Closer
	isClosed   bool
}

// NewResourceManager creates a new ResourceManager.
func NewResourceManager(w Putter) *ResourceManager {
	return &ResourceManager{
		Out:      w,
		embedded: make(map[any]embRes),
	}
}

type embRes struct {
	Val Object
	Emb any
}

// ResourceManagerEmbed embeds a resource in the PDF file.
//
// If the resource is already present in the file, the existing resource is
// returned.
//
// If the embedded type, T, is an io.Closer, the Close() method will be called
// when the ResourceManager is closed.
//
// Once Go supports methods with type parameters, this function can be turned
// into a method on ResourceManager.
func ResourceManagerEmbed[T any](rm *ResourceManager, r Embedder[T]) (Object, T, error) {
	var zero T

	if er, ok := rm.embedded[r]; ok {
		return er.Val, er.Emb.(T), nil
	}
	if rm.isClosed {
		return nil, zero, fmt.Errorf("resource manager is already closed")
	}

	val, emb, err := r.Embed(rm)
	if err != nil {
		return nil, zero, fmt.Errorf("failed to embed resource: %w", err)
	}

	rm.embedded[r] = embRes{Val: val, Emb: emb}

	if closer, ok := any(emb).(io.Closer); ok {
		rm.needsClose = append(rm.needsClose, closer)
	}

	return val, emb, nil
}

// Close closes all embedded resources which implement [io.Closer].
//
// After Close has been called, no more resources can be embedded.
func (rm *ResourceManager) Close() error {
	if rm.isClosed {
		return nil
	}
	for _, r := range rm.needsClose {
		if err := r.Close(); err != nil {
			return err
		}
	}
	return nil
}

// A CycleChecker checks for infinite recursion in PDF objects.
type CycleChecker struct {
	seen map[Reference]bool
}

// NewCycleChecker creates a new CycleChecker.
func NewCycleChecker() *CycleChecker {
	return &CycleChecker{seen: make(map[Reference]bool)}
}

// Check checks whether the given object is part of a recursive structure. If
// the object is not a reference, nil is returned.  If the object is a reference
// which has been seen before, ErrRecursiveStructure is returned.  Otherwise,
// nil is returned and the reference is marked as seen.
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
