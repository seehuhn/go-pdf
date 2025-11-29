// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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
