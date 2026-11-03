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

package fallback

import (
	"bytes"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/color"
)

// TestCaretPilcrowStable checks that a caret with a pilcrow keeps its
// rectangle, margin and appearance when its appearance is generated
// again, as it is whenever the annotation changes.
func TestCaretPilcrowStable(t *testing.T) {
	tests := []struct {
		rect   pdf.Rectangle
		margin []float64
	}{
		{pdf.Rectangle{LLx: 100, LLy: 200, URx: 104.85, URy: 209}, nil},
		{pdf.Rectangle{LLx: 100, LLy: 200, URx: 104.85, URy: 209}, []float64{1, 2, 1, 0}},
		// a caret box narrower than the rounding grid
		{pdf.Rectangle{LLx: 100, LLy: 200, URx: 100.004, URy: 209}, nil},
	}
	for _, tt := range tests {
		a := &annotation.Caret{
			Rect:   tt.rect,
			Color:  color.DeviceRGB{1, 0, 0},
			Margin: tt.margin,
			Symbol: "P",
		}
		gen := newGen(t, pdf.V2_0)
		if err := gen.AddAppearance(a); err != nil {
			t.Fatal(err)
		}
		// the caret box may only grow, by less than 0.01 on each side
		wantInner := applyMargins(tt.rect, tt.margin)
		got := applyMargins(a.Rect, a.Margin)
		grown := got.LLx <= wantInner.LLx+1e-9 && got.LLy <= wantInner.LLy+1e-9 &&
			got.URx >= wantInner.URx-1e-9 && got.URy >= wantInner.URy-1e-9
		if !grown || !got.NearlyEqual(&wantInner, 0.01) {
			t.Errorf("rect %v, margin %v: caret box moved from %v to %v", tt.rect, tt.margin, wantInner, got)
		}
		rect, m := a.Rect, a.Margin
		content := appearanceStream(t, a)

		for range 3 {
			a.Appearance = nil
			if err := gen.AddAppearance(a); err != nil {
				t.Fatal(err)
			}
		}
		if a.Rect != rect {
			t.Errorf("rect %v, margin %v: rectangle changed from %v to %v", tt.rect, tt.margin, rect, a.Rect)
		}
		if diff := cmp.Diff(m, a.Margin); diff != "" {
			t.Errorf("rect %v, margin %v: margin changed (-want +got):\n%s", tt.rect, tt.margin, diff)
		}
		if got := appearanceStream(t, a); !bytes.Equal(got, content) {
			t.Errorf("rect %v, margin %v: appearance changed from\n%s\nto\n%s", tt.rect, tt.margin, content, got)
		}
	}
}

// TestCaretPilcrowRoomGiven checks that a caret whose rectangle already
// leaves room for the pilcrow below the caret keeps its rectangle and
// margin.
func TestCaretPilcrowRoomGiven(t *testing.T) {
	margin := []float64{20, 30, 20, 0}
	a := &annotation.Caret{
		Rect:   pdf.Rectangle{LLx: 80, LLy: 170, URx: 125, URy: 209},
		Color:  color.DeviceRGB{1, 0, 0},
		Margin: margin,
		Symbol: "P",
	}
	rect := a.Rect
	gen := newGen(t, pdf.V2_0)
	if err := gen.AddAppearance(a); err != nil {
		t.Fatal(err)
	}
	if a.Rect != rect {
		t.Errorf("rectangle changed from %v to %v", rect, a.Rect)
	}
	if diff := cmp.Diff(margin, a.Margin); diff != "" {
		t.Errorf("margin changed (-want +got):\n%s", diff)
	}
}

// TestCaretPilcrowStableRandom checks stability for many caret boxes, so
// that rectangles whose corners fall close to the rounding grid are
// covered.
func TestCaretPilcrowStableRandom(t *testing.T) {
	gen := newGen(t, pdf.V2_0)
	rng := rand.New(rand.NewPCG(1, 2))
	for range 5000 {
		x := rng.Float64() * 500
		y := rng.Float64() * 500
		a := &annotation.Caret{
			Rect: pdf.Rectangle{
				LLx: x, LLy: y,
				URx: x + 0.5 + rng.Float64()*20, URy: y + 0.5 + rng.Float64()*20,
			},
			Color:  color.DeviceRGB{1, 0, 0},
			Symbol: "P",
		}
		if err := gen.AddAppearance(a); err != nil {
			t.Fatal(err)
		}
		rect, m := a.Rect, a.Margin
		content := appearanceStream(t, a)
		a.Appearance = nil
		if err := gen.AddAppearance(a); err != nil {
			t.Fatal(err)
		}
		if a.Rect != rect || !slices.Equal(a.Margin, m) {
			t.Fatalf("unstable: %v %v -> %v %v", rect, m, a.Rect, a.Margin)
		}
		if got := appearanceStream(t, a); !bytes.Equal(got, content) {
			t.Fatalf("unstable appearance for %v %v:\n%s\n->\n%s", rect, m, content, got)
		}
	}
}
