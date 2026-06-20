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

// This file holds low-level accessors that bundle and unbundle the two values
// a [Cursor] carries.  Most code should use the typed [Cursor] methods and
// [Decode] instead.  These accessors are for the few places that need to step
// the resolution path by hand (the linked-structure decoders in "outline",
// "navnode", "opaque" and "oc"), that stash an extractor for deferred
// re-extraction, or that reach the underlying [Getter] for document metadata.

// CursorAt builds a [Cursor] from an extractor and a resolution path.
func CursorAt(x *Extractor, path *CycleCheck) Cursor {
	return Cursor{x: x, path: path}
}

// Extractor returns the cursor's underlying extractor, which holds the object
// cache and resolves references.
func (c Cursor) Extractor() *Extractor { return c.x }

// Path returns the cursor's current cycle-detection path.
func (c Cursor) Path() *CycleCheck { return c.path }
