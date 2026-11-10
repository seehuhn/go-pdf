// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package fallback

import (
	"math"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/color"
)

func TestFlattenEllipseTooSmall(t *testing.T) {
	// an ellipse needs half an axis in each direction to be worth drawing
	cases := map[string]pdf.Rectangle{
		"empty":     {},
		"thin in x": {URx: 0.9, URy: 100},
		"thin in y": {URx: 100, URy: 0.9},
		"tiny":      {URx: 0.9, URy: 0.9},
	}
	for name, r := range cases {
		t.Run(name, func(t *testing.T) {
			if got := flattenEllipse(r); got != nil {
				t.Errorf("got %d vertices, want none", len(got))
			}
		})
	}
}

// TestFlattenEllipseOnCurve checks that the vertices lie on the ellipse
// inscribed in the rectangle.
func TestFlattenEllipseOnCurve(t *testing.T) {
	r := pdf.Rectangle{LLx: 10, LLy: 20, URx: 110, URy: 80}
	rx, ry := r.Dx()/2, r.Dy()/2
	xMid, yMid := (r.LLx+r.URx)/2, (r.LLy+r.URy)/2

	verts := flattenEllipse(r)
	if len(verts) < 12 {
		t.Fatalf("got %d vertices", len(verts))
	}
	for i, v := range verts {
		dx := (v.X - xMid) / rx
		dy := (v.Y - yMid) / ry
		if math.Abs(dx*dx+dy*dy-1) > 1e-12 {
			t.Errorf("vertex %d (%v) is not on the ellipse", i, v)
		}
	}
}

// TestFlattenEllipseSampleLimit checks that a rectangle with coordinates far
// beyond any page does not make the generator ask for an unbounded number of
// vertices.  The spacing is fixed, so without the limit the count would grow
// with the coordinates while the file stayed the same size, and the count
// would eventually leave the range of int.
func TestFlattenEllipseSampleLimit(t *testing.T) {
	for _, size := range []float64{1e5, 1e10, 1e30, 1e100} {
		verts := flattenEllipse(pdf.Rectangle{URx: size, URy: size})
		if len(verts) > maxSamplePoints {
			t.Errorf("size %g: got %d vertices, limit is %d",
				size, len(verts), maxSamplePoints)
		}
		if len(verts) < 12 {
			t.Errorf("size %g: got %d vertices", size, len(verts))
		}
		for i, v := range verts {
			if math.IsNaN(v.X) || math.IsNaN(v.Y) ||
				math.IsInf(v.X, 0) || math.IsInf(v.Y, 0) {
				t.Fatalf("size %g: vertex %d is %v", size, i, v)
			}
		}
	}
}

// TestCircleCloudyLargeRect drives the whole generator with a cloudy circle
// whose rectangle is far larger than any page, the shape which used to abort
// the appearance with a slice length out of range.
func TestCircleCloudyLargeRect(t *testing.T) {
	for _, size := range []float64{1e5, 1e10, 1e30, 1e100} {
		a := &annotation.Circle{
			Rect:         pdf.Rectangle{URx: size, URy: size},
			Color:        color.DeviceRGB{1, 0, 0},
			Border:       &annotation.Border{Width: 1},
			BorderEffect: &annotation.BorderEffect{Style: "C", Intensity: 1},
		}

		g := newGen(t, pdf.V2_0)
		if err := g.AddAppearance(a); err != nil {
			t.Errorf("size %g: %v", size, err)
		}
	}
}
