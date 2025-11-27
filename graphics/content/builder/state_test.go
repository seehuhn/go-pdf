package builder

import (
	"testing"

	"seehuhn.de/go/pdf/graphics"
)

func TestSliceNearlyEqual(t *testing.T) {
	// Two identical slices should be equal.
	a := []float64{1.0, 2.0, 3.0}
	b := []float64{1.0, 2.0, 3.0}

	if !sliceNearlyEqual(a, b) {
		t.Errorf("sliceNearlyEqual(%v, %v) = false, want true", a, b)
	}
}

func TestSetExtGState(t *testing.T) {
	b := New()

	gs := &graphics.ExtGState{
		Set:       graphics.StateLineWidth | graphics.StateFillAlpha,
		LineWidth: 5.0,
		FillAlpha: 0.5,
	}

	b.SetExtGState(gs)

	if b.Err != nil {
		t.Fatalf("SetExtGState failed: %v", b.Err)
	}

	// verify state was applied
	if b.State.Param.LineWidth != 5.0 {
		t.Errorf("LineWidth = %v, want 5.0", b.State.Param.LineWidth)
	}
	if b.State.Param.FillAlpha != 0.5 {
		t.Errorf("FillAlpha = %v, want 0.5", b.State.Param.FillAlpha)
	}
	if b.State.Out&graphics.StateLineWidth == 0 {
		t.Error("StateLineWidth not marked in Out")
	}
	if b.State.Out&graphics.StateFillAlpha == 0 {
		t.Error("StateFillAlpha not marked in Out")
	}
}
