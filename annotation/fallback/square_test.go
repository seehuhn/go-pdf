// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package fallback

import (
	"slices"
	"testing"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
)

// shapeCases returns a square and a circle with the given margins, both
// drawn in the same rectangle.
func shapeCases(rect pdf.Rectangle, margin []float64) map[string]annotation.Annotation {
	common := func() annotation.Common {
		return annotation.Common{Rect: rect, Color: color.DeviceRGB{1, 0, 0}}
	}
	return map[string]annotation.Annotation{
		"Square": &annotation.Square{Common: common(), Margin: margin},
		"Circle": &annotation.Circle{Common: common(), Margin: margin},
	}
}

// TestShapeIsDrawnInsideRect checks where a square or circle lands: the
// stroke reaches the annotation rectangle, less the margins, and stops there
// whatever the line width.
//
// Rect is the outer edge of everything the annotation draws, so a wider pen
// thickens the border inwards.  A pen which instead straddled Rect would put
// half of the border outside the rectangle the appearance is fitted to.
func TestShapeIsDrawnInsideRect(t *testing.T) {
	rect := pdf.Rectangle{LLx: 10, LLy: 10, URx: 110, URy: 60}

	for _, margin := range [][]float64{nil, {4, 4, 4, 4}} {
		outer := applyMargins(rect, margin)
		for name, a := range shapeCases(rect, margin) {
			t.Run(name, func(t *testing.T) {
				for _, lw := range []float64{1, 4, 8, 2} {
					annotation.SetBorderWidth(a, lw, pdf.V2_0)
					g := newGen(t, pdf.V2_0)
					if err := g.AddAppearance(a); err != nil {
						t.Fatal(err)
					}

					want := outer.Grow(-lw / 2)
					if got := inkBounds(t, a); !got.NearlyEqual(&want, 0.01) {
						t.Errorf("at width %v the shape is %v, want %v", lw, got, want)
					}
				}
			})
		}
	}
}

// TestShapeLeavesTheRectangleAlone checks that building an appearance
// changes neither the annotation rectangle nor the margins.  Both describe
// where the shape goes, so a generator which moved them would draw a
// different shape the next time it ran.
func TestShapeLeavesTheRectangleAlone(t *testing.T) {
	rect := pdf.Rectangle{LLx: 10, LLy: 10, URx: 110, URy: 60}

	for _, margin := range [][]float64{nil, {4, 4, 4, 4}} {
		for name, a := range shapeCases(rect, margin) {
			t.Run(name, func(t *testing.T) {
				annotation.SetBorderWidth(a, 6, pdf.V2_0)
				g := newGen(t, pdf.V2_0)
				if err := g.AddAppearance(a); err != nil {
					t.Fatal(err)
				}

				if got := a.GetCommon().Rect; !got.Equal(&rect) {
					t.Errorf("rect = %v, want %v", got, rect)
				}
				if got := marginOf(t, a); !slices.Equal(got, margin) {
					t.Errorf("margin = %v, want %v", got, margin)
				}

				// the appearance is fitted to Rect, so its bounding box has
				// to be Rect and not the ink inside it
				if got := bboxOf(t, a); !got.Equal(&rect) {
					t.Errorf("appearance bbox = %v, want %v", got, rect)
				}

				want := inkBounds(t, a)
				if err := g.AddAppearance(a); err != nil {
					t.Fatal(err)
				}
				if got := inkBounds(t, a); !got.NearlyEqual(&want, 1e-6) {
					t.Errorf("second appearance draws %v, first drew %v", got, want)
				}
			})
		}
	}
}

// inkBounds returns the bounding box of the paths an annotation's normal
// appearance paints, in form space.  Curves count by the points they pass
// through, not by their control points.  It is where the ink goes, before
// the pen the paths are stroked with is taken into account.
func inkBounds(t *testing.T, a annotation.Annotation) pdf.Rectangle {
	t.Helper()

	ap := annotation.Resolve(a.GetCommon(), appearance.Normal)
	if ap == nil || ap.Content == nil {
		t.Fatal("the annotation has no normal appearance")
	}
	st := content.NewState(content.Form, ap.Res)
	var ext rect.Rect
	found := false
	it := ap.Content.NewIter()
	for name, args := range it.All() {
		if err := st.ApplyStateChanges(name, args); err != nil {
			t.Fatal(err)
		}
		pp, ok := content.PathPainting(name)
		if !ok || !(pp.Fill || pp.Stroke) {
			continue
		}
		r, ok := st.PaintedPath().Iter().Transform(st.GState.CTM).TightBBox()
		if !ok {
			continue
		}
		ext.Extend(r)
		found = true
	}
	if err := it.Err(); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("the appearance draws no path")
	}
	return pdf.RectangleFromRect(ext)
}

// bboxOf returns the bounding box of an annotation's normal appearance.
func bboxOf(t *testing.T, a annotation.Annotation) pdf.Rectangle {
	t.Helper()

	ap := annotation.Resolve(a.GetCommon(), appearance.Normal)
	if ap == nil {
		t.Fatal("the annotation has no normal appearance")
	}
	return ap.BBox
}

// TestGivenUpCloudLeavesTheRectangleAlone checks that a cloudy shape whose
// border is too wide for a cloud, so that a plain border is drawn instead,
// is treated as one with a plain border: the ink stays inside the outer
// edge, and neither the rectangle nor the margins change.  A rectangle
// pushed out by the rounding allowance alone would record a margin the
// drawing never needed.
func TestGivenUpCloudLeavesTheRectangleAlone(t *testing.T) {
	rect := pdf.Rectangle{LLx: 10, LLy: 10, URx: 110, URy: 60}

	for _, margin := range [][]float64{nil, {4, 4, 4, 4}} {
		for name, a := range shapeCases(rect, margin) {
			t.Run(name, func(t *testing.T) {
				// a pen as wide as the shape allows, which leaves no room for
				// a cloud inside it
				annotation.SetBorderWidth(a, 25, pdf.V2_0)
				switch s := a.(type) {
				case *annotation.Square:
					s.BorderEffect = &annotation.BorderEffect{Style: "C", Intensity: 1}
				case *annotation.Circle:
					s.BorderEffect = &annotation.BorderEffect{Style: "C", Intensity: 1}
				}
				g := newGen(t, pdf.V2_0)
				if err := g.AddAppearance(a); err != nil {
					t.Fatal(err)
				}

				if got := a.GetCommon().Rect; !got.Equal(&rect) {
					t.Errorf("Rect = %v, want %v", got, rect)
				}
				if got := marginOf(t, a); !slices.Equal(got, margin) {
					t.Errorf("margins = %v, want %v", got, margin)
				}
				if got := bboxOf(t, a); !got.Equal(&rect) {
					t.Errorf("appearance bbox = %v, want %v", got, rect)
				}
			})
		}
	}
}

// TestCloudRecordsNoMarginOnAFlatSide checks that a side the curls do not
// reach past keeps its edge: a cloudy square's flat base lies on the outer
// edge of the border, so the margin on that side is zero.
func TestCloudRecordsNoMarginOnAFlatSide(t *testing.T) {
	a := &annotation.Square{
		Rect: pdf.Rectangle{LLx: 100, LLy: 100, URx: 200, URy: 200}, Color: color.DeviceRGB{1, 0, 0},
		BorderStyle:  &annotation.BorderStyle{Width: 2},
		BorderEffect: &annotation.BorderEffect{Style: "C", Intensity: 1},
	}
	g := newGen(t, pdf.V2_0)
	if err := g.AddAppearance(a); err != nil {
		t.Fatal(err)
	}
	m := marginOf(t, a)
	if len(m) != 4 {
		t.Fatalf("margins = %v, want four", m)
	}
	if m[1] != 0 {
		t.Errorf("margin below the flat base = %v, want 0", m[1])
	}
	if a.Rect.LLy != 100 {
		t.Errorf("Rect.LLy = %v, want 100", a.Rect.LLy)
	}
}
