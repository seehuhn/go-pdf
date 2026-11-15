// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package dict

import (
	"maps"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/postscript/cid"
)

func testRuns() CIDMetrics[float64] {
	return CIDMetrics[float64]{
		{First: 1, Last: 3, Value: 500},
		{First: 5, Last: 7, Values: []float64{10, 20, 30}},
	}
}

func TestCIDMetricsGet(t *testing.T) {
	m := testRuns()
	want := map[cid.CID]float64{1: 500, 2: 500, 3: 500, 5: 10, 6: 20, 7: 30}
	for c := cid.CID(0); c <= 65535; c++ {
		got, ok := m.Get(c)
		w, wantOK := want[c]
		if ok != wantOK || got != w {
			t.Errorf("Get(%d) = %v, %v; want %v, %v", c, got, ok, w, wantOK)
		}
	}

	var empty CIDMetrics[float64]
	if _, ok := empty.Get(1); ok {
		t.Error("Get on nil metrics succeeded")
	}
}

func TestCIDMetricsAll(t *testing.T) {
	m := testRuns()
	want := map[cid.CID]float64{1: 500, 2: 500, 3: 500, 5: 10, 6: 20, 7: 30}
	if diff := cmp.Diff(want, maps.Collect(m.All())); diff != "" {
		t.Errorf("All mismatch (-want +got):\n%s", diff)
	}

	var keys []cid.CID
	for c := range m.All() {
		keys = append(keys, c)
	}
	if !slices.IsSorted(keys) {
		t.Errorf("keys not increasing: %v", keys)
	}

	n := 0
	for range m.All() {
		n++
		if n == 2 {
			break
		}
	}
	if n != 2 {
		t.Errorf("early stop: got %d iterations", n)
	}
}

func TestCIDMetricsFromMap(t *testing.T) {
	cases := []struct {
		name string
		in   map[cid.CID]float64
		want CIDMetrics[float64]
	}{
		{"empty", map[cid.CID]float64{}, nil},
		{"single", map[cid.CID]float64{4: 7}, CIDMetrics[float64]{
			{First: 4, Last: 4, Values: []float64{7}},
		}},
		{"range", map[cid.CID]float64{1: 5, 2: 5, 3: 5}, CIDMetrics[float64]{
			{First: 1, Last: 3, Value: 5},
		}},
		{"pair", map[cid.CID]float64{1: 5, 2: 5}, CIDMetrics[float64]{
			{First: 1, Last: 2, Values: []float64{5, 5}},
		}},
		{"mixed", map[cid.CID]float64{1: 1, 2: 2, 3: 3, 4: 3, 5: 3, 6: 3, 7: 9}, CIDMetrics[float64]{
			{First: 1, Last: 2, Values: []float64{1, 2}},
			{First: 3, Last: 6, Value: 3},
			{First: 7, Last: 7, Values: []float64{9}},
		}},
		{"ends", map[cid.CID]float64{0: 1, 65535: 1}, CIDMetrics[float64]{
			{First: 0, Last: 0, Values: []float64{1}},
			{First: 65535, Last: 65535, Values: []float64{1}},
		}},
		{"gap", map[cid.CID]float64{1: 1, 3: 1}, CIDMetrics[float64]{
			{First: 1, Last: 1, Values: []float64{1}},
			{First: 3, Last: 3, Values: []float64{1}},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CIDMetricsFromMap(c.in)
			if err := got.validate(); err != nil {
				t.Errorf("invalid result: %v", err)
			}
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}

	if CIDMetricsFromMap[float64](nil) != nil {
		t.Error("nil map did not give nil")
	}

	t.Run("VMetrics", func(t *testing.T) {
		v := VMetrics{OffsX: 1, OffsY: 2, DeltaY: -3}
		in := map[cid.CID]VMetrics{2: v, 3: v, 4: v, 9: {DeltaY: -1}}
		got := CIDMetricsFromMap(in)
		want := CIDMetrics[VMetrics]{
			{First: 2, Last: 4, Value: v},
			{First: 9, Last: 9, Values: []VMetrics{{DeltaY: -1}}},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("random", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		for range 200 {
			in := make(map[cid.CID]float64)
			for range rng.IntN(60) {
				in[cid.CID(rng.IntN(100))] = float64(rng.IntN(3))
			}
			m := CIDMetricsFromMap(in)
			if err := m.validate(); err != nil {
				t.Fatalf("invalid result for %v: %v", in, err)
			}
			if diff := cmp.Diff(in, maps.Collect(m.All())); diff != "" {
				t.Fatalf("round trip failed (-want +got):\n%s", diff)
			}
		}
	})
}

func TestCIDMetricsValidate(t *testing.T) {
	cases := []struct {
		name string
		m    CIDMetrics[float64]
		ok   bool
	}{
		{"nil", nil, true},
		{"valid", testRuns(), true},
		{"top", CIDMetrics[float64]{{First: 65535, Last: 65535, Value: 1}}, true},
		{"reversed", CIDMetrics[float64]{{First: 5, Last: 4, Value: 1}}, false},
		{"too large", CIDMetrics[float64]{{First: 1, Last: 65536, Value: 1}}, false},
		{"bad length", CIDMetrics[float64]{{First: 1, Last: 3, Values: []float64{1, 2}}}, false},
		{"empty values", CIDMetrics[float64]{{First: 1, Last: 1, Values: []float64{}}}, false},
		{"overlap", CIDMetrics[float64]{
			{First: 1, Last: 5, Value: 1},
			{First: 5, Last: 7, Value: 2},
		}, false},
		{"order", CIDMetrics[float64]{
			{First: 10, Last: 12, Value: 1},
			{First: 1, Last: 2, Value: 2},
		}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.m.validate()
			if (err == nil) != c.ok {
				t.Errorf("validate() = %v, want ok=%v", err, c.ok)
			}
		})
	}
}
