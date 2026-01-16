// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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
