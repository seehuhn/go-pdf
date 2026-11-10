// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
