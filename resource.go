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
	"fmt"
	"io"
)

// Resource is a PDF resource inside a content stream.
type Resource interface {
	PDFObject() Object // value to use in the resource dictionary
}

// Res can be embedded in a struct to implement the [Resource] interface.
type Res struct {
	Data Object
}

// PDFObject implements the [Resource] interface.
func (r Res) PDFObject() Object {
	return r.Data
}

// Embedder represents a PDF resource which has not yet been associated
// with a PDF file.
type Embedder[T Resource] interface {
	// Embed embeds the resource into the PDF file
	Embed(rm *ResourceManager) (T, error)
}

// ResourceManager keeps track of which resources have been embedded in the PDF
// file.
//
// Use the [ResourceManagerEmbed] function to embed resources.
//
// The ResourceManager must be closed with the [Close] method before the PDF
// file is closed.
type ResourceManager struct {
	Out        Putter
	embedded   map[any]Resource
	needsClose []io.Closer
	isClosed   bool
}

// NewResourceManager creates a new ResourceManager.
func NewResourceManager(w Putter) *ResourceManager {
	return &ResourceManager{
		Out:      w,
		embedded: make(map[any]Resource),
	}
}

// ResourceManagerEmbed embeds a resource in the PDF file.
//
// If the embedded type, T, is an io.Closer, the Close() method will be called
// when the ResourceManager is closed.
//
// Once Go supports methods with type parameters, this function can be turned
// into a method on ResourceManager.
func ResourceManagerEmbed[T Resource](rm *ResourceManager, r Embedder[T]) (T, error) {
	var zero T

	if er, ok := rm.embedded[r]; ok {
		return er.(T), nil
	}
	if rm.isClosed {
		return zero, fmt.Errorf("resource manager is already closed")
	}

	er, err := r.Embed(rm)
	if err != nil {
		return zero, fmt.Errorf("failed to embed resource: %w", err)
	}

	rm.embedded[r] = er

	if closer, ok := any(er).(io.Closer); ok {
		rm.needsClose = append(rm.needsClose, closer)
	}

	return er, nil
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
