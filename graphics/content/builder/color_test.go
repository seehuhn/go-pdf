// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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
	"math"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
)

// TestSetColorRejectsOutOfRange checks that the builder refuses a colour with
// components outside the range its colour space allows.
//
// The reader clips such components into range (§8.4.1), so emitting them would
// write a file describing a different colour.  It would also leave the builder
// inconsistent with itself: the operator arguments would hold the value the
// caller passed, while the graphics state the builder tracks would hold the
// clipped one.
func TestSetColorRejectsOutOfRange(t *testing.T) {
	for _, tc := range []struct {
		name string
		col  color.Color
	}{
		{"gray above", color.DeviceGray(1.5)},
		{"gray below", color.DeviceGray(-0.5)},
		{"gray NaN", color.DeviceGray(math.NaN())},
		{"rgb above", color.DeviceRGB{0.5, 2, 0.5}},
		{"rgb below", color.DeviceRGB{0.5, -1, 0.5}},
		{"cmyk above", color.DeviceCMYK{0, 0, 0, 4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := New(content.Page, nil, pdf.V2_0)
			b.SetFillColor(tc.col)
			if b.Err == nil {
				t.Error("out-of-range fill colour accepted")
			}

			b = New(content.Page, nil, pdf.V2_0)
			b.SetStrokeColor(tc.col)
			if b.Err == nil {
				t.Error("out-of-range stroke colour accepted")
			}
		})
	}
}

// TestSetColorAcceptsRangeEdges checks that the bounds are inclusive, so that
// black and white are not rejected.
func TestSetColorAcceptsRangeEdges(t *testing.T) {
	for _, col := range []color.Color{
		color.DeviceGray(0), color.DeviceGray(1),
		color.DeviceRGB{0, 0, 0}, color.DeviceRGB{1, 1, 1},
		color.DeviceCMYK{0, 0, 0, 0}, color.DeviceCMYK{1, 1, 1, 1},
	} {
		b := New(content.Page, nil, pdf.V2_0)
		b.SetFillColor(col)
		if b.Err != nil {
			t.Errorf("%v rejected: %v", col, b.Err)
		}
	}
}

// TestSetColorStateMatchesStream checks that the graphics state the builder
// tracks agrees with the operands it emits.  The builder tracks its state by
// replaying each operator through the same state machine the reader uses, so
// any clipping there would make the two disagree.
func TestSetColorStateMatchesStream(t *testing.T) {
	b := New(content.Page, nil, pdf.V2_0)
	want := color.DeviceRGB{0.25, 0.5, 0.75}
	b.SetFillColor(want)
	if b.Err != nil {
		t.Fatal(b.Err)
	}

	if got := b.State.GState.FillColor; got != color.Color(want) {
		t.Errorf("tracked state = %v, want %v", got, want)
	}

	var found bool
	for _, op := range b.Stream {
		if op.Name != content.OpSetFillRGB {
			continue
		}
		found = true
		if len(op.Args) != 3 {
			t.Fatalf("got %d operands, want 3", len(op.Args))
		}
		for i, arg := range op.Args {
			num, ok := arg.(pdf.Number)
			if !ok || float64(num) != want[i] {
				t.Errorf("operand %d = %v, want %g", i, arg, want[i])
			}
		}
	}
	if !found {
		t.Error("no rg operator emitted")
	}
}
