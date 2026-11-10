// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
