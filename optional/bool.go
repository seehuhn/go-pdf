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

// Bool represents an optional boolean value.
//
// This is used for PDF fields that have three states: not set, true, or false.
// An example is the Trapped field in the document information dictionary,
// where "not set" means "Unknown".
type Bool struct {
	isSet bool
	val   bool
}

// NewBool creates a new Bool with the given value.
func NewBool(v bool) Bool {
	var b Bool
	b.Set(v)
	return b
}

// Get returns the value and whether it is set.
func (b Bool) Get() (bool, bool) {
	return b.val, b.isSet
}

// Set sets the value.
func (b *Bool) Set(v bool) {
	b.isSet = true
	b.val = v
}

// Clear clears the value.
func (b *Bool) Clear() {
	b.isSet = false
	b.val = false
}

// Equal compares two Bools for equality.
func (b Bool) Equal(other Bool) bool {
	return b.isSet == other.isSet && b.val == other.val
}
