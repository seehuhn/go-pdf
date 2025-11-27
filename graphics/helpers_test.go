package graphics

import "testing"

func TestSliceNearlyEqual(t *testing.T) {
	// Two identical slices should be equal.
	a := []float64{1.0, 2.0, 3.0}
	b := []float64{1.0, 2.0, 3.0}

	if !sliceNearlyEqual(a, b) {
		t.Errorf("sliceNearlyEqual(%v, %v) = false, want true", a, b)
	}
}
