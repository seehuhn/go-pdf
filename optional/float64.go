// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
