// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package pdf

import (
	"testing"
)

func TestLRUCache(t *testing.T) {
	cache := newCache(12)
	cache.Put(NewReference(100, 0), Integer(100))
	cache.Put(NewReference(101, 0), Integer(101))
	cache.Put(NewReference(102, 0), Integer(102))
	obj, ok := cache.Get(NewReference(100, 0))
	if !ok {
		t.Error("cache miss")
	}
	if obj != Integer(100) {
		t.Error("wrong object")
	}
	// now 101 is the oldest entry and should drop out later

	obj, ok = cache.Get(NewReference(0, 0))
	if ok {
		t.Error("cache hit")
	}
	if obj != nil {
		t.Error("wrong object")
	}

	for i := 0; i < 25; i++ {
		x := i % 10
		key := NewReference(uint32(x), 0)
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

	_, ok = cache.Get(NewReference(100, 0))
	if !ok {
		t.Error("cache miss")
	}
	_, ok = cache.Get(NewReference(101, 0))
	if ok {
		t.Error("cache hit")
	}
	_, ok = cache.Get(NewReference(102, 0))
	if !ok {
		t.Error("cache miss")
	}
}
