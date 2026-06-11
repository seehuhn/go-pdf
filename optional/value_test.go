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

func TestValueZeroValue(t *testing.T) {
	var v Value[int]
	_, ok := v.Get()
	if ok {
		t.Error("zero value should be absent")
	}
}

func TestValueSet(t *testing.T) {
	v := New(42)
	got, ok := v.Get()
	if !ok {
		t.Error("should be present")
	}
	if got != 42 {
		t.Errorf("got %d, want 42", got)
	}
}

func TestValuePresentZero(t *testing.T) {
	v := New(0)
	got, ok := v.Get()
	if !ok {
		t.Error("a present zero must be distinct from absent")
	}
	if got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

func TestValueClear(t *testing.T) {
	v := New(7)
	v.Clear()
	if got, ok := v.Get(); ok || got != 0 {
		t.Error("should be absent and zeroed after clear")
	}
}

func TestValueSetOverwrite(t *testing.T) {
	v := New(1)
	v.Set(2)
	if got, ok := v.Get(); !ok || got != 2 {
		t.Errorf("got %d (set %v), want 2 present", got, ok)
	}
}

func TestValueEqual(t *testing.T) {
	var unset1, unset2 Value[int]
	one1 := New(1)
	one2 := New(1)
	two := New(2)
	zero := New(0)

	if !unset1.Equal(unset2) {
		t.Error("two absent values should be equal")
	}
	if !one1.Equal(one2) {
		t.Error("equal present values should be equal")
	}
	if one1.Equal(two) {
		t.Error("different present values should not be equal")
	}
	if unset1.Equal(zero) {
		t.Error("absent and present-zero must not be equal")
	}
}
