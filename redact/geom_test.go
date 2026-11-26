// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"maps"
	"math"
	"math/rand"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/internal/redactmark"
)

func v(x, y float64) vec.Vec2 { return vec.Vec2{X: x, Y: y} }

func square(x0, y0, x1, y1 float64) poly {
	return convexHull([]vec.Vec2{v(x0, y0), v(x1, y0), v(x1, y1), v(x0, y1)})
}

func TestHull(t *testing.T) {
	proper := []vec.Vec2{v(0, 0), v(2, 0), v(2, 1), v(0, 1)}
	bowtie := []vec.Vec2{v(0, 0), v(2, 1), v(2, 0), v(0, 1)}
	want := convexHull(proper)
	if len(want) != 4 {
		t.Fatalf("hull of rectangle has %d vertices", len(want))
	}
	if diff := cmp.Diff(want, convexHull(bowtie)); diff != "" {
		t.Errorf("bow-tie hull differs (-want +got):\n%s", diff)
	}

	// counterclockwise: positive area
	var area float64
	for i := range want {
		area += want[i].Cross(want[(i+1)%len(want)])
	}
	if area <= 0 {
		t.Errorf("hull is not counterclockwise")
	}

	withExtra := append(proper, v(1, 0), v(1, 0.5), v(0, 0))
	if diff := cmp.Diff(want, convexHull(withExtra)); diff != "" {
		t.Errorf("extra points change hull (-want +got):\n%s", diff)
	}

	if h := convexHull([]vec.Vec2{v(0, 0), v(1, 1), v(2, 2), v(0.5, 0.5)}); len(h) != 2 {
		t.Errorf("collinear points: got %d vertices, want 2", len(h))
	}
	if h := convexHull([]vec.Vec2{v(3, 3), v(3, 3)}); len(h) != 1 {
		t.Errorf("coincident points: got %d vertices, want 1", len(h))
	}
	if h := convexHull(nil); len(h) != 0 {
		t.Errorf("no points: got %d vertices", len(h))
	}

	hs := regionHulls([]redactmark.Quad{redactmark.RectQuad(pdf.Rectangle{LLx: 0, LLy: 0, URx: 2, URy: 1})})
	if diff := cmp.Diff(want, hs[0]); diff != "" {
		t.Errorf("quad hull differs (-want +got):\n%s", diff)
	}
}

func TestIntersects(t *testing.T) {
	sq := square(0, 0, 1, 1)
	cases := []struct {
		name string
		b    poly
		want bool
	}{
		{"overlap", square(0.5, 0.5, 1.5, 1.5), true},
		{"disjoint", square(2, 2, 3, 3), false},
		{"edge touch", square(1, 0, 2, 1), false},
		{"corner touch", square(1, 1, 2, 2), false},
		{"slight overlap", square(0.9, 0, 2, 1), true},
		{"contained", square(0.25, 0.25, 0.75, 0.75), true},
		{"containing", square(-1, -1, 2, 2), true},
		{"segment crossing", poly{v(-1, 0.5), v(2, 0.5)}, true},
		{"segment outside", poly{v(-1, 2), v(2, 2)}, false},
		{"segment along edge", poly{v(0, 0), v(1, 0)}, false},
		{"collinear segment beyond", poly{v(2, 0), v(3, 0)}, false},
		{"point inside", poly{v(0.5, 0.5)}, true},
		{"point outside", poly{v(2, 2)}, false},
		{"point on edge", poly{v(1, 0.5)}, false},
		{"triangle overlap", convexHull([]vec.Vec2{v(0.9, 0.9), v(3, 0.9), v(0.9, 3)}), true},
		{"triangle clear", convexHull([]vec.Vec2{v(1.6, 0), v(3, 0), v(1.6, 3)}), false},
	}
	for _, c := range cases {
		if got := intersects(sq, c.b, touchTol); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
		if got := intersects(c.b, sq, touchTol); got != c.want {
			t.Errorf("%s (swapped): got %v, want %v", c.name, got, c.want)
		}
	}

	segA := poly{v(0, 0), v(2, 2)}
	if !intersects(segA, poly{v(0, 2), v(2, 0)}, touchTol) {
		t.Errorf("crossing segments do not intersect")
	}
	if !intersects(segA, poly{v(1, 1), v(3, 3)}, touchTol) {
		t.Errorf("overlapping collinear segments do not intersect")
	}
	if intersects(segA, poly{v(2, 2), v(3, 3)}, touchTol) {
		t.Errorf("end-to-end segments intersect")
	}
	if !intersects(poly{v(1, 1)}, poly{v(1, 1)}, touchTol) {
		t.Errorf("coincident points do not intersect")
	}
}

func TestInside(t *testing.T) {
	h := square(0, 0, 10, 10)
	pts := []vec.Vec2{v(2, 2), v(8, 3), v(5, 9)}
	if !inside(pts, h, 0) {
		t.Errorf("points not inside with reach 0")
	}
	if !inside(pts, h, 1) {
		t.Errorf("points not inside with reach 1")
	}
	if inside(pts, h, 1.5) {
		t.Errorf("points inside with reach 1.5")
	}
	if inside(append(pts, v(11, 5)), h, 0) {
		t.Errorf("outside point accepted")
	}
	if !inside([]vec.Vec2{v(0, 0), v(10, 10)}, h, 0) {
		t.Errorf("points on the boundary not inside with reach 0")
	}
	rot := h.transform(matrix.RotateDeg(30))
	if !inside(transformPoints(pts, matrix.RotateDeg(30)), rot, 1) {
		t.Errorf("rotated points not inside rotated hull")
	}
}

func TestTransform(t *testing.T) {
	m := matrix.Scale(2, 3).Mul(matrix.Translate(5, 1))
	got := square(0, 0, 1, 1).transform(m)
	want := square(5, 1, 7, 4)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("transform differs (-want +got):\n%s", diff)
	}

	// mirroring keeps the polygon counterclockwise
	got = square(0, 0, 1, 1).transform(matrix.Scale(-1, 1))
	if diff := cmp.Diff(square(-1, 0, 0, 1), got); diff != "" {
		t.Errorf("mirrored differs (-want +got):\n%s", diff)
	}

	// singular matrices flatten the polygon
	flat := square(0, 0, 1, 1).transform(matrix.Matrix{1, 0, 0, 0, 0, 0})
	if len(flat) != 2 {
		t.Errorf("flattened square has %d vertices, want 2", len(flat))
	}
	pt := square(0, 0, 1, 1).transform(matrix.Matrix{0, 0, 0, 0, 3, 4})
	if len(pt) != 1 {
		t.Errorf("collapsed square has %d vertices, want 1", len(pt))
	}
}

func TestSigmaMax(t *testing.T) {
	cases := []struct {
		name string
		m    matrix.Matrix
		want float64
	}{
		{"identity", matrix.Identity, 1},
		{"rotation", matrix.RotateDeg(37), 1},
		{"translation", matrix.Translate(100, -3), 1},
		{"scale", matrix.Scale(2, 5), 5},
		{"shear", matrix.Matrix{1, 0, 1, 1, 0, 0}, (1 + math.Sqrt(5)) / 2},
		{"singular", matrix.Matrix{1, 2, 2, 4, 0, 0}, 5},
	}
	for _, c := range cases {
		if got := sigmaMax(c.m); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: got %g, want %g", c.name, got, c.want)
		}
	}
}

func TestStrokeReach(t *testing.T) {
	m := matrix.Scale(2, 3)
	cases := []struct {
		name  string
		lw    float64
		limit float64
		join  graphics.LineJoinStyle
		cap   graphics.LineCapStyle
		want  float64
	}{
		{"plain", 4, 10, graphics.LineJoinRound, graphics.LineCapButt, 2 * 3},
		{"miter", 4, 10, graphics.LineJoinMiter, graphics.LineCapButt, 2 * 10 * 3},
		{"miter small limit", 4, 0.5, graphics.LineJoinMiter, graphics.LineCapButt, 2 * 3},
		{"square cap", 4, 10, graphics.LineJoinBevel, graphics.LineCapSquare, 2 * math.Sqrt2 * 3},
		{"miter and square", 4, 1.2, graphics.LineJoinMiter, graphics.LineCapSquare, 2 * math.Sqrt2 * 3},
	}
	for _, c := range cases {
		got := strokeReach(c.lw, c.limit, c.join, c.cap, m)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: got %g, want %g", c.name, got, c.want)
		}
	}
	// a line of width zero is still drawn
	if got := strokeReach(0, 10, graphics.LineJoinMiter, graphics.LineCapSquare, m); !(got > 0) {
		t.Errorf("zero width: got reach %g, want a positive value", got)
	}
}

// bruteCells lists the cells which meet h when shrunk by ex and ey, by
// testing each one.
func bruteCells(h poly, w, rows int, ex, ey float64) map[[2]int]bool {
	res := make(map[[2]int]bool)
	for r := range rows {
		for c := range w {
			cell := square(float64(c)/float64(w)+ex, 1-float64(r+1)/float64(rows)+ey,
				float64(c+1)/float64(w)-ex, 1-float64(r)/float64(rows)-ey)
			if intersects(h, cell, -cellEps) {
				res[[2]int{r, c}] = true
			}
		}
	}
	return res
}

func TestCellRanges(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for trial := range 300 {
		w := 1 + rng.Intn(23)
		rows := 1 + rng.Intn(23)
		var pts []vec.Vec2
		for range 1 + rng.Intn(5) {
			pts = append(pts, v(rng.Float64()*1.6-0.3, rng.Float64()*1.6-0.3))
		}
		h := convexHull(pts)
		if trial%3 == 0 && len(h) > 0 {
			// rotated copy
			h = h.transform(matrix.RotateDeg(rng.Float64()*360).Translate(0.5, 0.5))
		}

		var ex, ey float64
		if trial%2 == 1 {
			ex, ey = rng.Float64()*0.4/float64(w), rng.Float64()*0.4/float64(rows)
		}

		want := bruteCells(h, w, rows, ex, ey)
		got := make(map[[2]int]bool)
		last := -1
		for r, cr := range cellRanges(h, w, rows, ex, ey) {
			c0, c1 := cr[0], cr[1]
			if r <= last {
				t.Fatalf("trial %d: rows not increasing", trial)
			}
			last = r
			if c0 < 0 || c1 > w || c0 >= c1 {
				t.Fatalf("trial %d: bad range [%d,%d)", trial, c0, c1)
			}
			for c := c0; c < c1; c++ {
				got[[2]int{r, c}] = true
			}
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("trial %d (%v, %dx%d, margins %g, %g): cells differ (-want +got):\n%s",
				trial, h, w, rows, ex, ey, diff)
		}
	}
}

func TestCellRangesRect(t *testing.T) {
	// an image of 4x4 cells, polygon covering the upper-left interior
	h := square(0.3, 0.55, 0.45, 0.95)
	got := maps.Collect(cellRanges(h, 4, 4, 0, 0))
	want := map[int][2]int{0: {1, 2}, 1: {1, 2}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ranges differ (-want +got):\n%s", diff)
	}

	// a polygon on cell boundaries: without margins the cells which touch
	// it count, with margins only the cells it covers
	h = square(0, 0.5, 0.5, 1)
	got = maps.Collect(cellRanges(h, 4, 4, 0, 0))
	want = map[int][2]int{0: {0, 3}, 1: {0, 3}, 2: {0, 3}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ranges without margins differ (-want +got):\n%s", diff)
	}
	got = maps.Collect(cellRanges(h, 4, 4, 1e-3, 1e-3))
	want = map[int][2]int{0: {0, 2}, 1: {0, 2}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ranges with margins differ (-want +got):\n%s", diff)
	}

	// polygon outside the image yields nothing
	for range cellRanges(square(2, 2, 3, 3), 4, 4, 0, 0) {
		t.Errorf("cell found for polygon outside the image")
	}
}

func TestGrow(t *testing.T) {
	const r = 2.0
	shapes := map[string]poly{
		"square":   square(0, 0, 10, 5),
		"triangle": convexHull([]vec.Vec2{v(0, 0), v(8, 1), v(3, 7)}),
		"segment":  convexHull([]vec.Vec2{v(0, 0), v(6, 3)}),
		"point":    convexHull([]vec.Vec2{v(4, 4)}),
	}
	rng := rand.New(rand.NewSource(1))
	for name, h := range shapes {
		t.Run(name, func(t *testing.T) {
			g := h.grow(r)
			// every point within r of a vertex lies in the grown shape
			for range 200 {
				p := h[rng.Intn(len(h))]
				a, d := rng.Float64()*2*math.Pi, r*math.Sqrt(rng.Float64())
				q := p.Add(v(d*math.Cos(a), d*math.Sin(a)))
				if !inside([]vec.Vec2{q}, g, 0) {
					t.Fatalf("point %v is not in the grown shape", q)
				}
			}
		})
	}

	// a region which is further away than r is missed
	h := square(0, 0, 10, 5)
	if intersects(h.grow(r), square(10+r+0.1, 0, 20, 5), touchTol) {
		t.Error("grown shape meets a distant region")
	}
	if !intersects(h.grow(r), square(10+r-0.1, 0, 20, 5), touchTol) {
		t.Error("grown shape misses a near region")
	}
}

func TestCellRangesExtreme(t *testing.T) {
	const w, rows = 7, 5
	inf, nan := math.Inf(1), math.NaN()
	shapes := []poly{
		{v(-1e308, -1e308), v(1e308, -1e308), v(1e308, 1e308), v(-1e308, 1e308)},
		{v(-1.5e308, -1), v(1.5e308, -1), v(0, 1.7e308)},
		{v(-1e308, -1e308), v(1.7e308, 0.5), v(-1e308, 1e308)},
		{v(-inf, -inf), v(inf, -inf), v(inf, inf), v(-inf, inf)},
		{v(nan, 0.5), v(0.5, nan), v(0.2, 0.2)},
		{v(0.5, 0.5), v(1e300, 0.5000001), v(1e300, 0.5000002)},
		{v(5e307, 0.3), v(-5e307, 0.7)},
	}
	for i, h := range shapes {
		last := -1
		for r, cr := range cellRanges(h, w, rows, 0.01, 0.01) {
			if r <= last || r >= rows {
				t.Fatalf("shape %d: bad row %d", i, r)
			}
			last = r
			if cr[0] < 0 || cr[1] > w || cr[0] >= cr[1] {
				t.Fatalf("shape %d: bad range %v", i, cr)
			}
		}
	}

	// shapes which contain the image meet all cells
	for i, h := range shapes[:3] {
		got := maps.Collect(cellRanges(h, w, rows, 0.01, 0.01))
		for r := range rows {
			if got[r] != [2]int{0, w} {
				t.Errorf("shape %d: row %d has range %v", i, r, got[r])
			}
		}
	}
}

func TestClip(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	randPoly := func() poly {
		var pts []vec.Vec2
		for range 1 + rng.Intn(6) {
			pts = append(pts, v(rng.Float64()*4-2, rng.Float64()*4-2))
		}
		return convexHull(pts)
	}
	box := square(-1, -1, 1.5, 1.5)
	for trial := range 300 {
		h, c := randPoly(), randPoly()
		if trial%2 == 0 {
			c = box
		}
		if len(c) < 3 {
			continue
		}
		var got poly
		if trial%2 == 0 {
			got = convexHull(clipBox(h, -1, 1.5))
		} else {
			got = convexHull(clipConvex(h, c))
		}
		for _, p := range got {
			if !inside([]vec.Vec2{p}, c.grow(1e-9), 0) {
				t.Fatalf("trial %d: vertex %v is outside the clip polygon", trial, p)
			}
		}
		if len(h) < 3 {
			continue
		}
		for range 50 {
			p := v(rng.Float64()*4-2, rng.Float64()*4-2)
			pt := []vec.Vec2{p}
			if inside(pt, h, 1e-9) && inside(pt, c, 1e-9) && !inside(pt, got, -1e-9) {
				t.Fatalf("trial %d: point %v of the intersection is lost", trial, p)
			}
			if !inside(pt, h, -1e-9) && inside(pt, got, 1e-9) {
				t.Fatalf("trial %d: point %v outside the polygon is kept", trial, p)
			}
		}
	}

	if got := clipConvex(square(0, 0, 1, 1), square(2, 2, 3, 3)); len(got) != 0 {
		t.Errorf("disjoint polygons give %v", got)
	}

	// huge coordinates
	const big = 1.7e308
	got := convexHull(clipBox([]vec.Vec2{v(-big, -big), v(big, -big), v(big, big), v(-big, big)}, -1, 2))
	if diff := cmp.Diff(square(-1, -1, 2, 2), got); diff != "" {
		t.Errorf("wrong clip of a huge square (-want +got):\n%s", diff)
	}
	got = convexHull(clipBox([]vec.Vec2{v(-big, 0.5), v(big, 0.5), v(0, big)}, -1, 2))
	if !inside([]vec.Vec2{v(-0.9, 0.6), v(1.9, 1.9)}, got, 0) {
		t.Errorf("clip of a huge triangle is %v", got)
	}
}

func TestScaledHull(t *testing.T) {
	const big = 1.7e308
	pts := []vec.Vec2{v(-big, -big), v(0, 0), v(big, -big), v(big, big), v(-big, big)}
	want := poly{v(-big, -big), v(big, -big), v(big, big), v(-big, big)}
	if diff := cmp.Diff(want, scaledHull(pts)); diff != "" {
		t.Errorf("wrong hull (-want +got):\n%s", diff)
	}
	small := []vec.Vec2{v(0, 0), v(2, 0), v(1, 1), v(1, 3)}
	if diff := cmp.Diff(convexHull(small), scaledHull(small)); diff != "" {
		t.Errorf("wrong hull (-want +got):\n%s", diff)
	}
}

func TestPlaceImageExtremeMatrix(t *testing.T) {
	type testCase struct {
		name   string
		m      matrix.Matrix
		region poly
		// cols are the columns of a 4x4 image which lie under the region
		cols [2]int
	}
	cases := []testCase{
		{
			name:   "thin",
			m:      matrix.Matrix{100, 0, 0, 1e-307, 100, 0},
			region: square(150, -10, 250, 50),
			cols:   [2]int{2, 4},
		},
		{
			// the error bound of the image-space hull is not finite
			name:   "far",
			m:      matrix.Matrix{1, 0, 0, 2e-308, 1e15, 0},
			region: square(1e15+0.5, -1, 1e15+2, 1),
			cols:   [2]int{2, 4},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			how, hulls := placeImage(tc.m, []poly{tc.region})
			if how != placeRedact {
				t.Fatalf("placement is %d", how)
			}
			cells := cellsFor(hulls, 4, 4)
			for r := range 4 {
				covered := slices.ContainsFunc(cells[r], func(rng [2]int) bool {
					return rng[0] <= tc.cols[0] && rng[1] >= tc.cols[1]
				})
				if !covered {
					t.Errorf("row %d: cells %v miss columns %v", r, cells[r], tc.cols)
				}
			}
		})
	}
}

// placedCells returns the cells of a w × h image painted with the matrix m
// which are overwritten for the region, and false if the whole image is
// removed.
func placedCells(m matrix.Matrix, region poly, w, h int) (map[[2]int]bool, bool) {
	how, reg := placeImage(m, []poly{region})
	res := map[[2]int]bool{}
	switch how {
	case placeDrop:
		return nil, false
	case placeRedact:
		for r, rs := range cellsFor(reg, w, h) {
			for _, rng := range rs {
				for c := rng[0]; c < rng[1]; c++ {
					res[[2]int{r, c}] = true
				}
			}
		}
	}
	return res, true
}

// pageCell returns the cell (r, c) of a w × h image painted with the matrix
// m, in page space.
func pageCell(m matrix.Matrix, w, h, r, c int) poly {
	x0, x1 := float64(c)/float64(w), float64(c+1)/float64(w)
	y0, y1 := 1-float64(r+1)/float64(h), 1-float64(r)/float64(h)
	return convexHull(transformPoints([]vec.Vec2{v(x0, y0), v(x1, y0), v(x1, y1), v(x0, y1)}, m))
}

// TestImageCellsConservative checks, for random placements of images and
// random regions, that every cell which overlaps a region by more than
// touchTol in page space is overwritten.
func TestImageCellsConservative(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	gens := map[string]func() matrix.Matrix{
		"general": func() matrix.Matrix {
			var m matrix.Matrix
			for i := range m {
				m[i] = rng.NormFloat64() * 100
			}
			return m
		},
		"axis-parallel": func() matrix.Matrix {
			return matrix.Matrix{float64(1 + rng.Intn(100)), 0, 0, float64(1 + rng.Intn(100)),
				float64(rng.Intn(50)), float64(rng.Intn(50))}
		},
		"near-singular": func() matrix.Matrix {
			a, b, k := rng.NormFloat64()*100, rng.NormFloat64()*100, rng.NormFloat64()
			eps := math.Pow(10, -float64(rng.Intn(12)))
			return matrix.Matrix{a, b, k*a + eps*rng.NormFloat64(), k*b + eps*rng.NormFloat64(),
				rng.NormFloat64() * 50, rng.NormFloat64() * 50}
		},
	}
	for name, gen := range gens {
		t.Run(name, func(t *testing.T) {
			fails := 0
			for trial := 0; trial < 3000 && fails < 10; trial++ {
				w, h := 1+rng.Intn(12), 1+rng.Intn(12)
				m := gen()
				var q []vec.Vec2
				for range 4 {
					q = append(q, v(rng.NormFloat64()*80, rng.NormFloat64()*80))
				}
				region := convexHull(q)
				if len(region) < 3 {
					continue
				}
				cells, ok := placedCells(m, region, w, h)
				if !ok {
					continue
				}
				for r := range h {
					for c := range w {
						if cells[[2]int{r, c}] || !intersects(pageCell(m, w, h, r, c), region, touchTol) {
							continue
						}
						fails++
						t.Errorf("trial %d: m=%v, cell (%d,%d) of %dx%d under %v is kept",
							trial, m, r, c, w, h, region)
					}
				}
			}
		})
	}
}

// TestImageCellsAligned checks that a region on cell boundaries overwrites
// exactly the cells it covers, for images which are mirrored or rotated.
func TestImageCellsAligned(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for trial := range 500 {
		w, h := 1+rng.Intn(8), 1+rng.Intn(8)
		kx, ky := 1+rng.Intn(20), 1+rng.Intn(20)
		sx, sy := float64(kx*w), float64(ky*h)
		tx, ty := float64(rng.Intn(300)), float64(rng.Intn(300))
		var m matrix.Matrix
		switch trial % 4 {
		case 0:
			m = matrix.Matrix{sx, 0, 0, sy, tx, ty}
		case 1:
			m = matrix.Matrix{-sx, 0, 0, sy, tx, ty}
		case 2:
			m = matrix.Matrix{sx, 0, 0, -sy, tx, ty}
		case 3:
			m = matrix.Matrix{0, sy, -sx, 0, tx, ty}
		}
		c0, r0 := rng.Intn(w), rng.Intn(h)
		c1, r1 := c0+1+rng.Intn(w-c0), r0+1+rng.Intn(h-r0)
		if c1-c0 == w && r1-r0 == h {
			continue
		}

		// the region covers columns [c0,c1) and rows [r0,r1)
		var pts []vec.Vec2
		for _, p := range [][2]int{{c0, r0}, {c1, r0}, {c1, r1}, {c0, r1}} {
			pts = append(pts, m.Apply(v(float64(p[0])/float64(w), 1-float64(p[1])/float64(h))))
		}
		region := convexHull(pts)
		want := map[[2]int]bool{}
		for r := r0; r < r1; r++ {
			for c := c0; c < c1; c++ {
				want[[2]int{r, c}] = true
			}
		}
		got, ok := placedCells(m, region, w, h)
		if !ok {
			t.Fatalf("trial %d: image removed", trial)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("trial %d (m=%v, %dx%d): cells differ (-want +got):\n%s", trial, m, w, h, diff)
		}
	}
}
