// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package pdf

import (
	"reflect"
	"sync"
	"testing"
)

// mapGetter is a Getter which serves the objects in the map.
type mapGetter map[Reference]Native

func (g mapGetter) GetMeta() *MetaInfo {
	return &MetaInfo{Version: V2_0}
}

func (g mapGetter) Get(ref Reference, _ bool) (Native, error) {
	return g[ref], nil
}

// TestDecodeExclusiveChain checks that DecodeExclusive single-flights on the
// object, not on the reference it was given, so that callers which reach one
// object through different references run the decode function once.
func TestDecodeExclusiveChain(t *testing.T) {
	obj, a, b := NewReference(1, 0), NewReference(2, 0), NewReference(3, 0)
	g := mapGetter{
		obj: Dict{"X": Integer(1)},
		a:   obj,
		b:   a,
	}
	starts := []Reference{obj, a, b}
	key := extractorKey{ref: obj, tp: reflect.TypeFor[*int]()}

	for _, start := range starts {
		x := NewExtractor(g)
		_, err := CursorAt(x, nil).DecodeExclusive(start, func(Cursor, Object, bool) (*int, error) {
			x.mu.Lock()
			_, inFlight := x.wip[key]
			x.mu.Unlock()
			if !inFlight {
				t.Errorf("decode via %s is not single-flighted on the object", start)
			}
			return new(int), nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	x := NewExtractor(g)
	var mu sync.Mutex
	calls := 0
	decode := func(Cursor, Object, bool) (*int, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		return new(int), nil
	}
	var wg sync.WaitGroup
	results := make([]*int, 6)
	for i := range results {
		wg.Go(func() {
			v, err := CursorAt(x, nil).DecodeExclusive(starts[i%3], decode)
			if err != nil {
				t.Error(err)
			}
			results[i] = v
		})
	}
	wg.Wait()
	if calls != 1 {
		t.Errorf("decode function ran %d times, want 1", calls)
	}
	for i, v := range results {
		if v != results[0] {
			t.Errorf("caller %d got a different value", i)
		}
	}
}
