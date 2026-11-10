// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package graphics

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCheckDashPatternValid(t *testing.T) {
	for _, tc := range []struct {
		pattern []float64
		phase   float64
	}{
		{nil, 0},
		{[]float64{}, 0},
		{[]float64{3}, 0},
		{[]float64{3, 2}, 1},
		{[]float64{3, 2}, 6},
		{[]float64{3, 0}, 0}, // only "all zero" is forbidden
		{[]float64{2}, -5},   // a negative phase has defined semantics
	} {
		if err := CheckDashPattern(tc.pattern, tc.phase); err != nil {
			t.Errorf("CheckDashPattern(%v, %v): %v", tc.pattern, tc.phase, err)
		}
	}
}

func TestCheckDashPatternInvalid(t *testing.T) {
	for _, tc := range []struct {
		pattern []float64
		phase   float64
	}{
		{[]float64{-1}, 0},          // negative dash length
		{[]float64{3, -2}, 0},       //
		{[]float64{0}, 0},           // all dash lengths zero
		{[]float64{0, 0}, 0},        //
		{nil, 1},                    // solid line with a non-zero phase
		{[]float64{}, -1},           //
		{[]float64{math.NaN()}, 0},  // no PDF representation
		{[]float64{math.Inf(1)}, 0}, //
		{[]float64{3}, math.NaN()},  //
		{[]float64{3}, math.Inf(1)}, //
		{[]float64{3}, math.Inf(-1)},
	} {
		if err := CheckDashPattern(tc.pattern, tc.phase); err == nil {
			t.Errorf("CheckDashPattern(%v, %v): expected an error, got none",
				tc.pattern, tc.phase)
		}
	}
}

// TestRepairDashPattern checks that dash patterns a PDF file can contain but
// the specification does not allow are turned into equivalent valid ones.
func TestRepairDashPattern(t *testing.T) {
	for _, tc := range []struct {
		inPattern   []float64
		inPhase     float64
		wantPattern []float64
		wantPhase   float64
	}{
		{[]float64{3, 2}, 1, []float64{3, 2}, 1},
		{[]float64{3, 2}, -1, []float64{3, 2}, -1},
		{[]float64{-1, 2}, 3, []float64{0, 2}, 3},
		{[]float64{3, -2}, 0, []float64{3, 0}, 0},
		{[]float64{-1, -2}, 3, []float64{}, 0}, // degenerates to a solid line
		{[]float64{0, 0}, 5, []float64{}, 0},
		{[]float64{}, 7, []float64{}, 0}, // a solid line needs a zero phase
	} {
		gotPattern, gotPhase := RepairDashPattern(tc.inPattern, tc.inPhase)

		if diff := cmp.Diff(tc.wantPattern, gotPattern); diff != "" {
			t.Errorf("pattern (-want +got):\n%s", diff)
		}
		if gotPhase != tc.wantPhase {
			t.Errorf("phase = %v, want %v", gotPhase, tc.wantPhase)
		}

		// the repaired pattern must be one we can write back out
		if err := CheckDashPattern(gotPattern, gotPhase); err != nil {
			t.Errorf("repaired pattern is still invalid: %v", err)
		}
	}
}
