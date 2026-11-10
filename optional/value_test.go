// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
