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

package appearance

import (
	"testing"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/form"
)

func TestResolve(t *testing.T) {
	cases := []struct {
		name  string
		dict  *Dict
		state pdf.Name
		kind  Kind
		want  *form.Form
	}{
		{
			name: "nilDict",
			dict: nil,
			kind: Normal,
			want: nil,
		},
		{
			name: "normal",
			dict: &Dict{Normal: appA},
			kind: Normal,
			want: appA,
		},
		{
			name: "ownRollOver",
			dict: &Dict{Normal: appA, RollOver: appB},
			kind: RollOver,
			want: appB,
		},
		{
			// an appearance the caller left out shows the normal one, the way
			// it does for a reader
			name: "rollOverFallsBack",
			dict: &Dict{Normal: appA},
			kind: RollOver,
			want: appA,
		},
		{
			name: "downFallsBack",
			dict: &Dict{Normal: appA, RollOver: appB},
			kind: Down,
			want: appA,
		},
		{
			// an empty map names no appearance, so it is left out as well
			name: "emptyMapFallsBack",
			dict: &Dict{Normal: appA, DownMap: map[pdf.Name]*form.Form{}},
			kind: Down,
			want: appA,
		},
		{
			name:  "stateSelects",
			dict:  &Dict{NormalMap: normalStates},
			state: "Off",
			kind:  Normal,
			want:  appB,
		},
		{
			// the fallback keeps the state: a down appearance left out shows
			// the normal appearance of the current state
			name:  "stateKeptOnFallback",
			dict:  &Dict{NormalMap: normalStates},
			state: "On",
			kind:  Down,
			want:  appA,
		},
		{
			// §12.5.5: nothing is shown for a state the appearance dictionary
			// does not define
			name:  "unknownState",
			dict:  &Dict{NormalMap: normalStates},
			state: "Middle",
			kind:  Normal,
			want:  nil,
		},
		{
			// ... and likewise where the annotation names no state at all
			name: "noState",
			dict: &Dict{NormalMap: normalStates},
			kind: Normal,
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.dict.Resolve(tc.state, tc.kind)
			if got != tc.want {
				t.Errorf("Resolve(%q, %v) = %v, want %v",
					tc.state, tc.kind, got, tc.want)
			}
		})
	}
}

// TestSetNormal checks that an appearance which repeated the previous normal
// appearance repeats the new one afterwards, while one of its own is left
// alone.  The result is read back through Resolve, which is what the appearance
// shown to the user follows.
func TestSetNormal(t *testing.T) {
	cases := []struct {
		name         string
		dict         *Dict
		wantRollOver *form.Form
		wantDown     *form.Form
	}{
		{
			// entries the caller left out follow the new normal appearance
			name:         "shorthand",
			dict:         &Dict{Normal: appA},
			wantRollOver: appC,
			wantDown:     appC,
		},
		{
			name:         "repeatsNormal",
			dict:         &Dict{Normal: appA, RollOver: appA, Down: appA},
			wantRollOver: appC,
			wantDown:     appC,
		},
		{
			name:         "ownAppearance",
			dict:         &Dict{Normal: appA, RollOver: appB, Down: appB},
			wantRollOver: appB,
			wantDown:     appB,
		},
		{
			name:         "mixed",
			dict:         &Dict{Normal: appA, RollOver: appA, Down: appB},
			wantRollOver: appC,
			wantDown:     appB,
		},
		{
			// a per-state appearance repeating the normal one follows the new
			// normal appearance, even where the new one has no states
			name:         "statesRepeatNormal",
			dict:         &Dict{NormalMap: normalStates, RollOverMap: normalStates},
			wantRollOver: appC,
			wantDown:     appC,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := tc.dict
			d.SetNormal(appC, nil)

			if got := d.Resolve("", Normal); got != appC {
				t.Errorf("normal appearance = %v, want %v", got, appC)
			}
			if got := d.Resolve("", RollOver); got != tc.wantRollOver {
				t.Errorf("rollover appearance = %v, want %v", got, tc.wantRollOver)
			}
			if got := d.Resolve("", Down); got != tc.wantDown {
				t.Errorf("down appearance = %v, want %v", got, tc.wantDown)
			}
		})
	}
}

// TestAppearanceToRect checks the mapping of §12.5.5: the matrix returned maps
// the appearance's bounding box onto the annotation rectangle, so that the
// smallest upright rectangle around the mapped bounding box is the annotation
// rectangle itself.  The property is checked rather than the matrix entries,
// since any matrix with this property draws the appearance where it belongs.
func TestAppearanceToRect(t *testing.T) {
	rect := pdf.Rectangle{LLx: 100, LLy: 200, URx: 220, URy: 260}

	cases := []struct {
		name   string
		bbox   pdf.Rectangle
		matrix matrix.Matrix
	}{
		{
			name:   "identity",
			bbox:   pdf.Rectangle{URx: 24, URy: 24},
			matrix: matrix.Identity,
		},
		{
			// a form matrix left at zero stands for the identity matrix
			name:   "zeroMatrix",
			bbox:   pdf.Rectangle{URx: 24, URy: 24},
			matrix: matrix.Zero,
		},
		{
			name:   "offsetBBox",
			bbox:   pdf.Rectangle{LLx: -10, LLy: 5, URx: 30, URy: 45},
			matrix: matrix.Identity,
		},
		{
			name:   "scaled",
			bbox:   pdf.Rectangle{URx: 24, URy: 24},
			matrix: matrix.Scale(2, 3),
		},
		{
			name:   "translated",
			bbox:   pdf.Rectangle{URx: 24, URy: 24},
			matrix: matrix.Translate(-17, 42),
		},
		{
			// the appearance box is the upright rectangle around the rotated
			// quadrilateral
			name:   "rotated",
			bbox:   pdf.Rectangle{URx: 40, URy: 10},
			matrix: matrix.RotateDeg(30),
		},
		{
			name:   "flipped",
			bbox:   pdf.Rectangle{URx: 24, URy: 24},
			matrix: matrix.Scale(-1, 1),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ap := testForm(tc.bbox, tc.matrix)

			m, ok := ToRect(ap, rect)
			if !ok {
				t.Fatal("no appearance to draw")
			}

			got := mappedBounds(tc.bbox, m)
			if !got.NearlyEqual(&rect, 1e-9) {
				t.Errorf("mapped bounding box = %s, want %s", &got, &rect)
			}
		})
	}
}

// TestAppearanceToRectNothingToDraw checks the cases where there is nothing to
// draw and no matrix to draw it with.
func TestAppearanceToRectNothingToDraw(t *testing.T) {
	rect := pdf.Rectangle{LLx: 100, LLy: 200, URx: 220, URy: 260}

	noContent := testForm(pdf.Rectangle{URx: 24, URy: 24}, matrix.Identity)
	noContent.Content = nil

	cases := []struct {
		name string
		ap   *form.Form
	}{
		{
			name: "nilForm",
			ap:   nil,
		},
		{
			name: "noContent",
			ap:   noContent,
		},
		{
			name: "emptyBBox",
			ap:   testForm(pdf.Rectangle{URx: 24}, matrix.Identity),
		},
		{
			// a bounding box which is flattened by the form matrix leaves
			// nothing to draw, either
			name: "collapsedByMatrix",
			ap:   testForm(pdf.Rectangle{URx: 24, URy: 24}, matrix.Scale(1, 0)),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := ToRect(tc.ap, rect); ok {
				t.Error("got a matrix for an appearance with nothing to draw")
			}
		})
	}
}

// testForm returns an appearance with the given bounding box and form matrix.
func testForm(bbox pdf.Rectangle, m matrix.Matrix) *form.Form {
	f := makeTestAppearance(0.5)
	f.BBox = bbox
	f.Matrix = m
	return f
}

// mappedBounds returns the smallest upright rectangle containing the image of
// bbox under m.
func mappedBounds(bbox pdf.Rectangle, m matrix.Matrix) pdf.Rectangle {
	ll := m.Apply(vec.Vec2{X: bbox.LLx, Y: bbox.LLy})
	lr := m.Apply(vec.Vec2{X: bbox.URx, Y: bbox.LLy})
	ul := m.Apply(vec.Vec2{X: bbox.LLx, Y: bbox.URy})
	ur := m.Apply(vec.Vec2{X: bbox.URx, Y: bbox.URy})
	return pdf.Rectangle{
		LLx: min(ll.X, lr.X, ul.X, ur.X),
		LLy: min(ll.Y, lr.Y, ul.Y, ur.Y),
		URx: max(ll.X, lr.X, ul.X, ur.X),
		URy: max(ll.Y, lr.Y, ul.Y, ur.Y),
	}
}
