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

import "testing"

func TestFloat64ZeroValue(t *testing.T) {
	var f Float64
	_, ok := f.Get()
	if ok {
		t.Error("zero value should not be set")
	}
}

func TestFloat64SetNonZero(t *testing.T) {
	f := NewFloat64(0.5)
	v, ok := f.Get()
	if !ok {
		t.Error("should be set")
	}
	if v != 0.5 {
		t.Errorf("got %g, want 0.5", v)
	}
}

func TestFloat64SetZero(t *testing.T) {
	f := NewFloat64(0)
	v, ok := f.Get()
	if !ok {
		t.Error("should be set even when value is 0")
	}
	if v != 0 {
		t.Errorf("got %g, want 0", v)
	}
}

func TestFloat64Clear(t *testing.T) {
	f := NewFloat64(0.5)
	f.Clear()
	_, ok := f.Get()
	if ok {
		t.Error("should not be set after clear")
	}
}

func TestFloat64Equal(t *testing.T) {
	var unset1, unset2 Float64
	a1 := NewFloat64(0.5)
	a2 := NewFloat64(0.5)
	b := NewFloat64(0.25)
	zero1 := NewFloat64(0)
	zero2 := NewFloat64(0)

	if !unset1.Equal(unset2) {
		t.Error("two unset values should be equal")
	}
	if !a1.Equal(a2) {
		t.Error("two equal values should be equal")
	}
	if !zero1.Equal(zero2) {
		t.Error("two set-zero values should be equal")
	}
	if a1.Equal(b) {
		t.Error("different values should not be equal")
	}
	if unset1.Equal(zero1) {
		t.Error("unset and set-zero should not be equal")
	}
}
