package pdf

import (
	"testing"
)

func TestLRUCache(t *testing.T) {
	cache := newCache(12)
	cache.Put(&Reference{Number: 100}, Integer(100))
	cache.Put(&Reference{Number: 101}, Integer(101))
	cache.Put(&Reference{Number: 102}, Integer(102))
	obj, ok := cache.Get(&Reference{Number: 100})
	if !ok {
		t.Error("cache miss")
	}
	if obj != Integer(100) {
		t.Error("wrong object")
	}
	// now 101 is the oldest entry and should drop out later

	obj, ok = cache.Get(&Reference{Number: 0})
	if ok {
		t.Error("cache hit")
	}
	if obj != nil {
		t.Error("wrong object")
	}

	for i := 0; i < 25; i++ {
		x := i % 10
		key := &Reference{Number: uint32(x)}
		val := Integer(x)

		obj, ok := cache.Get(key)
		if ok != (i >= 10) {
			t.Error("cache hit/miss mismatch")
		}
		if ok {
			if obj != val {
				t.Error("wrong object")
			}
		} else {
			cache.Put(key, val)
		}
	}

	_, ok = cache.Get(&Reference{Number: 100})
	if !ok {
		t.Error("cache miss")
	}
	_, ok = cache.Get(&Reference{Number: 101})
	if ok {
		t.Error("cache hit")
	}
	_, ok = cache.Get(&Reference{Number: 102})
	if !ok {
		t.Error("cache miss")
	}
}
