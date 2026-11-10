// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package optional

// UInt represents an optional integer.
type UInt struct {
	isSet bool
	val   uint
}

// NewUInt creates a new UInt with the given value.
func NewUInt(v uint) UInt {
	var k UInt
	k.Set(v)
	return k
}

// Get returns the value and whether it is set.
func (k UInt) Get() (uint, bool) {
	return k.val, k.isSet
}

// Set sets the value.
func (k *UInt) Set(v uint) {
	k.isSet = true
	k.val = v
}

// Clear clears the value.
func (k *UInt) Clear() {
	k.isSet = false
	k.val = 0
}

// Equal compares two UInts for equality.
func (k UInt) Equal(other UInt) bool {
	return k.isSet == other.isSet && k.val == other.val
}
