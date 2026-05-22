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

// Float64 represents an optional 64-bit floating-point value.
//
// This is used for PDF fields whose absence carries meaning distinct
// from any in-range numeric value (e.g. movie-action override fields
// where "no override" must be distinguishable from "override with the
// PDF default").
type Float64 struct {
	isSet bool
	val   float64
}

// NewFloat64 creates a new Float64 with the given value.
func NewFloat64(v float64) Float64 {
	var f Float64
	f.Set(v)
	return f
}

// Get returns the value and whether it is set.
func (f Float64) Get() (float64, bool) {
	return f.val, f.isSet
}

// Set sets the value.
func (f *Float64) Set(v float64) {
	f.isSet = true
	f.val = v
}

// Clear clears the value.
func (f *Float64) Clear() {
	f.isSet = false
	f.val = 0
}

// Equal compares two Float64s for equality.
func (f Float64) Equal(other Float64) bool {
	return f.isSet == other.isSet && f.val == other.val
}
