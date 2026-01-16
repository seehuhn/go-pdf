package optional

import "testing"

func TestBoolZeroValue(t *testing.T) {
	var b Bool
	_, ok := b.Get()
	if ok {
		t.Error("zero value should not be set")
	}
}

func TestBoolSetTrue(t *testing.T) {
	b := NewBool(true)
	v, ok := b.Get()
	if !ok {
		t.Error("should be set")
	}
	if !v {
		t.Error("should be true")
	}
}

func TestBoolSetFalse(t *testing.T) {
	b := NewBool(false)
	v, ok := b.Get()
	if !ok {
		t.Error("should be set")
	}
	if v {
		t.Error("should be false")
	}
}

func TestBoolClear(t *testing.T) {
	b := NewBool(true)
	b.Clear()
	_, ok := b.Get()
	if ok {
		t.Error("should not be set after clear")
	}
}

func TestBoolEqual(t *testing.T) {
	var unset1, unset2 Bool
	true1 := NewBool(true)
	true2 := NewBool(true)
	false1 := NewBool(false)
	false2 := NewBool(false)

	if !unset1.Equal(unset2) {
		t.Error("two unset values should be equal")
	}
	if !true1.Equal(true2) {
		t.Error("two true values should be equal")
	}
	if !false1.Equal(false2) {
		t.Error("two false values should be equal")
	}
	if unset1.Equal(true1) {
		t.Error("unset and true should not be equal")
	}
	if unset1.Equal(false1) {
		t.Error("unset and false should not be equal")
	}
	if true1.Equal(false1) {
		t.Error("true and false should not be equal")
	}
}

func TestBoolSetOverwrite(t *testing.T) {
	b := NewBool(true)
	b.Set(false)
	v, ok := b.Get()
	if !ok || v {
		t.Error("should be set to false")
	}

	b.Set(true)
	v, ok = b.Get()
	if !ok || !v {
		t.Error("should be set to true")
	}
}
