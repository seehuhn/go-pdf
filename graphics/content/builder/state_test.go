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

	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/extgstate"
	"seehuhn.de/go/pdf/graphics/state"
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
	b := New(content.Page, nil)

	gs := &extgstate.ExtGState{
		Set:       state.LineWidth | state.FillAlpha,
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
	if !b.State.IsSet(state.LineWidth) {
		t.Error("StateLineWidth not marked as Known")
	}
	if !b.State.IsSet(state.FillAlpha) {
		t.Error("StateFillAlpha not marked as Known")
	}
}

func TestBuilder_ElisionWithKnown(t *testing.T) {
	// Page: defaults are Known, elision works
	b := New(content.Page, nil)
	b.SetLineWidth(1.0) // default value
	if len(b.Stream) != 0 {
		t.Errorf("Page: setting default should elide, got %d ops", len(b.Stream))
	}

	// Form: defaults are Set-Unknown, no elision
	b2 := New(content.Form, nil)
	b2.SetLineWidth(1.0) // same value but not Known
	if len(b2.Stream) != 1 {
		t.Errorf("Form: should not elide Set-Unknown, got %d ops", len(b2.Stream))
	}
}

func TestBuilder_ElisionAfterSet(t *testing.T) {
	b := New(content.Form, nil)

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
