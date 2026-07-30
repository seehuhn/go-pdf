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
// A decoder that only needs to step back to the reference it was called for
// should use [Cursor.AtRef] rather than take the path apart itself.

// CursorAt builds a [Cursor] from an extractor and a resolution path.
func CursorAt(x *Extractor, path *CycleCheck) Cursor {
	return Cursor{x: x, path: path}
}

// Extractor returns the cursor's underlying extractor, which holds the object
// cache and resolves references.
func (c Cursor) Extractor() *Extractor { return c.x }

// Path returns the cursor's current cycle-detection path.
func (c Cursor) Path() *CycleCheck { return c.path }

// AtRef undoes the reference resolution [Decode] performs before it calls a
// decode function.  If obj was reached through a reference, AtRef returns that
// reference together with a cursor anchored just above it; otherwise c and obj
// come back unchanged.  isDirect is the flag the decode function received.
// A cursor built by hand with no path also comes back unchanged, since there
// is no reference left to step back to.
//
// A decoder that inspects the resolved object and then delegates to another
// decode function needs this.  Passing the resolved object on decodes it a
// second time, under no reference, yielding a Go value distinct from the one
// the extractor caches for that reference; code that compares results by
// pointer would then see two objects where the file has one.  Decoding the
// returned object under the returned cursor steps over the reference again, so
// the cycle check stays intact.
func (c Cursor) AtRef(obj Object, isDirect bool) (Cursor, Object) {
	if isDirect || c.path == nil {
		return c, obj
	}
	return Cursor{x: c.x, path: c.path.Parent}, c.path.Ref
}
