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

// lruCache is a simple LRU cache for PDF objects.
type lruCache struct {
	capacity    int
	entries     map[Reference]*cacheEntry
	first, last *cacheEntry
}

type cacheEntry struct {
	prev, next *cacheEntry
	key        Reference
	obj        Object
}

// newCache creates a new LRU cache with the given capacity.
func newCache(capacity int) *lruCache {
	return &lruCache{
		capacity: capacity,
		entries:  make(map[Reference]*cacheEntry, capacity),
	}
}

// Put adds an object to the cache.
func (l *lruCache) Put(key Reference, obj Object) {
	if l.capacity <= 0 {
		return
	}

	if ent, ok := l.entries[key]; ok {
		ent.obj = obj
		l.moveToFront(ent)
		return
	}

	ent := &cacheEntry{
		key: key,
		obj: obj,
	}
	l.entries[key] = ent
	l.moveToFront(ent)

	if len(l.entries) > l.capacity {
		l.removeLast()
	}
}

// Get returns an object from the cache and markes it as recently used.
func (l *lruCache) Get(key Reference) (Object, bool) {
	ent, ok := l.entries[key]
	if !ok {
		return nil, false
	}

	l.moveToFront(ent)
	return ent.obj, true
}

// Has returns true if the cache contains the given key.
// The object is not marked as recently used.
func (l *lruCache) Has(key Reference) bool {
	_, ok := l.entries[key]
	return ok
}

func (l *lruCache) moveToFront(ent *cacheEntry) {
	if ent == l.first {
		return
	}

	if ent.prev != nil {
		ent.prev.next = ent.next
	}
	if ent.next != nil {
		ent.next.prev = ent.prev
	}
	if ent == l.last {
		l.last = ent.prev
	}

	ent.prev = nil
	ent.next = l.first
	if l.first != nil {
		l.first.prev = ent
	}
	l.first = ent
	if l.last == nil {
		l.last = ent
	}
}

func (l *lruCache) removeLast() {
	if l.last == nil {
		return
	}

	delete(l.entries, l.last.key)
	if l.last.prev != nil {
		l.last.prev.next = nil
	}
	l.last = l.last.prev
}
