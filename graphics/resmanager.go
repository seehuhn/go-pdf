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

package graphics

import (
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
)

// Embedder represents a PDF resource which has not yet been associated
// with a PDF file.
type Embedder[T pdf.Resource] interface {
	// Embed embeds the resource into the PDF file
	Embed(w pdf.Putter) (T, error)
}

// ResourceManager keeps track of which resources have been embedded in the PDF
// file.
type ResourceManager struct {
	w          pdf.Putter
	embedded   map[any]pdf.Resource
	needsClose []io.Closer
	isClosed   bool
}

// NewResourceManager creates a new ResourceManager.
func NewResourceManager(w pdf.Putter) *ResourceManager {
	return &ResourceManager{
		w:        w,
		embedded: make(map[any]pdf.Resource),
	}
}

// ResourceManagerEmbed embeds a resource in the PDF file.
//
// If the embedded type, T, is an io.Closer, the Close() method will be
// called when the ResourceManager is closed.
//
// If/when Go supports methods with type parameters, this function will
// be turned into a method on ResourceManager.
func ResourceManagerEmbed[T pdf.Resource](rm *ResourceManager, r Embedder[T]) (T, error) {
	var zero T

	if rm == nil {
		return zero, fmt.Errorf("no resource manager provided")
	}
	if er, ok := rm.embedded[r]; ok {
		return er.(T), nil
	}
	if rm.isClosed {
		return zero, fmt.Errorf("resource manager is already closed")
	}

	er, err := r.Embed(rm.w)
	if err != nil {
		return zero, fmt.Errorf("failed to embed resource: %w", err)
	}

	rm.embedded[r] = er

	if closer, ok := any(er).(io.Closer); ok {
		rm.needsClose = append(rm.needsClose, closer)
	}

	return er, nil
}

// Close closes all embedded resources which are also io.Closers.
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
