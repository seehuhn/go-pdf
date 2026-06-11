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

package optional

// Value represents an optional value of type T.
//
// The zero Value is the absent state. Because absence is distinct from any
// particular value, Value is used for PDF entries whose omission carries
// meaning different from any value they could hold — for example an inheritable
// field flag set, where a present zero blocks inheritance that an absent entry
// would allow.
//
// T must be comparable so that two Values can be compared with [Value.Equal].
type Value[T comparable] struct {
	isSet bool
	val   T
}

// New returns a Value holding v (the present state).
func New[T comparable](v T) Value[T] {
	return Value[T]{isSet: true, val: v}
}

// Get returns the contained value and whether it is present.
func (o Value[T]) Get() (T, bool) {
	return o.val, o.isSet
}

// Set stores v, making the Value present.
func (o *Value[T]) Set(v T) {
	o.isSet = true
	o.val = v
}

// Clear makes the Value absent.
func (o *Value[T]) Clear() {
	var zero T
	o.isSet = false
	o.val = zero
}

// Equal reports whether o and other are both absent, or both present and equal.
func (o Value[T]) Equal(other Value[T]) bool {
	return o.isSet == other.isSet && o.val == other.val
}
