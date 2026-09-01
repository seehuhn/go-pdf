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
	"fmt"
	"math"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/extgstate"
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
	b := New(content.Page, nil, pdf.V2_0)

	gs := &extgstate.ExtGState{
		Set:       graphics.StateLineWidth | graphics.StateFillAlpha,
		LineWidth: 5.0,
		FillAlpha: 0.5,
	}

	b.SetExtGState(gs)

	if b.Err != nil {
		t.Fatalf("SetExtGState failed: %v", b.Err)
	}

	// verify state was applied
	if b.State.GState.LineWidth != 5.0 {
		t.Errorf("LineWidth = %v, want 5.0", b.State.GState.LineWidth)
	}
	if b.State.GState.FillAlpha != 0.5 {
		t.Errorf("FillAlpha = %v, want 0.5", b.State.GState.FillAlpha)
	}
	// with new State model, params set by gs are Known
	if !b.State.IsSet(graphics.StateLineWidth) {
		t.Error("StateLineWidth not marked as Known")
	}
	if !b.State.IsSet(graphics.StateFillAlpha) {
		t.Error("StateFillAlpha not marked as Known")
	}
}

func TestBuilder_ElisionWithKnown(t *testing.T) {
	// Page: defaults are Known, elision works
	b := New(content.Page, nil, pdf.V2_0)
	b.SetLineWidth(1.0) // default value
	if len(b.Stream) != 0 {
		t.Errorf("Page: setting default should elide, got %d ops", len(b.Stream))
	}

	// Form: defaults are Set-Unknown, no elision
	b2 := New(content.Form, nil, pdf.V2_0)
	b2.SetLineWidth(1.0) // same value but not Known
	if len(b2.Stream) != 1 {
		t.Errorf("Form: should not elide Set-Unknown, got %d ops", len(b2.Stream))
	}
}

func TestBuilder_ElisionAfterSet(t *testing.T) {
	b := New(content.Form, nil, pdf.V2_0)

	// First set: should emit (not Known)
	b.SetLineWidth(5.0)
	if len(b.Stream) != 1 {
		t.Errorf("First set should emit, got %d ops", len(b.Stream))
	}

	// Second set with same value: should elide (now Known)
	b.SetLineWidth(5.0)
	if len(b.Stream) != 1 {
		t.Errorf("Second set should elide, got %d ops", len(b.Stream))
	}
}

// TestNonFiniteStateValuesRejected checks that the graphics state setters
// which validate a range reject non-finite values too.  A NaN fails every
// ordered comparison, so a check written as a rejection of the bad values
// would let one through and emit a token no PDF processor can read back.
func TestNonFiniteStateValuesRejected(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  func(*Builder, float64)
	}{
		{"SetLineWidth", (*Builder).SetLineWidth},
		{"SetMiterLimit", (*Builder).SetMiterLimit},
		{"SetFlatnessTolerance", (*Builder).SetFlatnessTolerance},
	} {
		for _, v := range []float64{
			math.NaN(),
			math.Inf(+1),
			math.Inf(-1),
		} {
			t.Run(fmt.Sprintf("%s/%v", tc.name, v), func(t *testing.T) {
				b := New(content.Page, nil, pdf.V2_0)
				tc.set(b, v)
				if b.Err == nil {
					t.Error("expected an error, got none")
				}
				if len(b.Stream) != 0 {
					t.Errorf("emitted %d operators, want 0", len(b.Stream))
				}
			})
		}
	}
}

// TestSetLineDashRejectsInvalid checks that dash patterns the specification
// does not allow are refused, rather than written into the content stream.
func TestSetLineDashRejectsInvalid(t *testing.T) {
	for _, tc := range []struct {
		pattern []float64
		phase   float64
	}{
		{[]float64{-1}, 0},          // negative dash length
		{[]float64{0}, 0},           // all dash lengths zero
		{[]float64{0, 0}, 0},        //
		{nil, 1},                    // solid line with a non-zero phase
		{[]float64{math.NaN()}, 0},  // no PDF representation
		{[]float64{math.Inf(1)}, 0}, //
		{[]float64{3}, math.NaN()},  //
	} {
		t.Run(fmt.Sprintf("%v/%v", tc.pattern, tc.phase), func(t *testing.T) {
			b := New(content.Page, nil, pdf.V2_0)
			b.SetLineDash(tc.pattern, tc.phase)
			if b.Err == nil {
				t.Error("expected an error, got none")
			}
			if len(b.Stream) != 0 {
				t.Errorf("emitted %d operators, want 0", len(b.Stream))
			}
		})
	}
}

// TestSetLineDashAccepts checks that valid dash patterns are still written.
func TestSetLineDashAccepts(t *testing.T) {
	for _, tc := range []struct {
		pattern []float64
		phase   float64
	}{
		{nil, 0},          // solid line
		{[]float64{3}, 0}, //
		{[]float64{3, 2}, 1},
		{[]float64{3, 0}, 0}, // only "all zero" is forbidden
		{[]float64{2}, -5},   // a negative phase has defined semantics
	} {
		t.Run(fmt.Sprintf("%v/%v", tc.pattern, tc.phase), func(t *testing.T) {
			b := New(content.Page, nil, pdf.V2_0)
			b.SetLineDash(tc.pattern, tc.phase)
			if b.Err != nil {
				t.Error(b.Err)
			}
		})
	}
}
