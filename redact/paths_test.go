// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
)

// redactPath redacts the region (100,100)-(200,200) from a page with the
// given content and returns the operators of the result.
func redactPath(t testing.TB, data string) []opRecord {
	t.Helper()
	in := textSource(t, data, pdf.Dict{"Rect": rect(100, 100, 200, 200)})
	out := writeAndRead(t, in, nil, "")
	_, dicts := pageDicts(t, out)
	ops := pageOps(t, out, dicts[0])

	// skip the clip which excludes the regions
	for i, op := range ops {
		if op.name == content.OpClipEvenOdd {
			return ops[i+2:]
		}
	}
	t.Fatal("exclusion clip missing")
	return nil
}

// pathOnly returns the path-construction, clipping and painting operators
// of ops.
func pathOnly(ops []opRecord) []string {
	var res []string
	for _, op := range ops {
		switch op.name {
		case content.OpMoveTo, content.OpLineTo, content.OpCurveTo, content.OpCurveToV,
			content.OpCurveToY, content.OpClosePath, content.OpRectangle,
			content.OpClipNonZero, content.OpClipEvenOdd,
			content.OpStroke, content.OpCloseAndStroke, content.OpFill, content.OpFillCompat,
			content.OpFillEvenOdd, content.OpFillAndStroke, content.OpFillAndStrokeEvenOdd,
			content.OpCloseFillAndStroke, content.OpCloseFillAndStrokeEvenOdd, content.OpEndPath:
			res = append(res, fmtOps([]opRecord{op})[0])
		}
	}
	return res
}

// pathPoints returns the operands of the construction operators of ops as
// a list of points, with "m" operators marking subpath starts only if they
// are explicit.  Rectangles are expanded to four points and closed.
func pathPoints(t testing.TB, ops []opRecord) []vec.Vec2 {
	t.Helper()
	var res []vec.Vec2
	for _, op := range ops {
		var v []float64
		for _, a := range op.args {
			x, _ := getNumber(a)
			v = append(v, x)
		}
		switch op.name {
		case content.OpMoveTo, content.OpLineTo:
			res = append(res, vec.Vec2{X: v[0], Y: v[1]})
		case content.OpCurveTo:
			res = append(res, vec.Vec2{X: v[0], Y: v[1]}, vec.Vec2{X: v[2], Y: v[3]},
				vec.Vec2{X: v[4], Y: v[5]})
		case content.OpRectangle:
			res = append(res, vec.Vec2{X: v[0], Y: v[1]}, vec.Vec2{X: v[0] + v[2], Y: v[1]},
				vec.Vec2{X: v[0] + v[2], Y: v[1] + v[3]}, vec.Vec2{X: v[0], Y: v[1] + v[3]})
		}
	}
	return res
}

func TestPathInsideRemoved(t *testing.T) {
	got := pathOnly(redactPath(t, "120 120 40 40 re f"))
	if len(got) != 0 {
		t.Errorf("path survives: %v", got)
	}
}

func TestPathCrossingKept(t *testing.T) {
	got := pathOnly(redactPath(t, "150 150 100 40 re f"))
	want := []string{"150 150 100 40 re\n", "f\n"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("path changed (-want +got):\n%s", diff)
	}
}

func TestPathSubpathsSelective(t *testing.T) {
	got := pathOnly(redactPath(t, "10 10 m 50 10 l 50 50 l h 120 120 m 150 120 l 150 150 l h 220 220 m 260 220 l 260 260 l h f"))
	want := []string{
		"10 10 m\n", "50 10 l\n", "50 50 l\n", "h\n",
		"220 220 m\n", "260 220 l\n", "260 260 l\n", "h\n", "f\n",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("path changed (-want +got):\n%s", diff)
	}
}

func TestPathImplicitSubpath(t *testing.T) {
	cases := []struct {
		name, data string
		want       []vec.Vec2 // geometry of the kept subpaths
	}{
		{
			// after h the next segment starts at the subpath start
			"after h",
			"120 120 m 150 120 l 150 150 l h 10 10 l 30 10 l 30 30 l f",
			[]vec.Vec2{{X: 120, Y: 120}, {X: 10, Y: 10}, {X: 30, Y: 10}, {X: 30, Y: 30}},
		},
		{
			"after re",
			"120 120 20 20 re 10 10 l 30 10 l 30 30 l f",
			[]vec.Vec2{{X: 120, Y: 120}, {X: 10, Y: 10}, {X: 30, Y: 10}, {X: 30, Y: 30}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ops := redactPath(t, c.data)
			// the kept segments start at the start point of the deleted
			// subpath, which is far inside the region
			pts := pathPoints(t, ops)
			kept := pts
			if len(kept) == 0 || kept[0] != c.want[0] {
				t.Fatalf("implicit start lost: %v", pathOnly(ops))
			}
			// the first operator of the kept part is an explicit m
			var first content.OpName
			for _, op := range ops {
				if op.name == content.OpMoveTo || op.name == content.OpLineTo {
					first = op.name
					break
				}
			}
			if first != content.OpMoveTo {
				t.Errorf("kept path starts with %q, want m", first)
			}
			// and the geometry after the m is that of the input
			rest := kept[1:]
			if !slices.Equal(rest, c.want[1:]) {
				t.Errorf("got points %v, want %v", rest, c.want[1:])
			}
		})
	}
}

func TestPathStrokeReach(t *testing.T) {
	// a horizontal line at y = 105 lies inside the region; its stroke,
	// 20 wide, extends below the region
	cases := []struct {
		name, data string
		removed    bool
	}{
		{"thin", "1 w 130 150 m 170 150 l S", true},
		{"wide", "130 w 130 150 m 170 150 l S", false},
		{"fill only", "130 w 130 150 m 170 150 l 170 160 l f", true},
		{"square cap", "50 w 1 j 2 J 130 150 m 170 150 l S", false},
		{"butt cap", "50 w 1 j 0 J 130 150 m 170 150 l S", true},
		{"miter", "30 w 0 j 10 M 130 150 m 170 150 l 170 160 l S", false},
		{"scaled", "q 10 0 0 10 0 0 cm 5 w 13 15 m 17 15 l S Q", false},
		{"scaled thin", "q 10 0 0 10 0 0 cm 0.5 w 13 15 m 17 15 l S Q", true},
		{"hairline", "0 w 130 100.1 m 170 100.1 l S", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := pathOnly(redactPath(t, c.data))
			if (len(got) == 0) != c.removed {
				t.Errorf("removed=%v, want %v (%v)", len(got) == 0, c.removed, got)
			}
		})
	}
}

func TestPathClip(t *testing.T) {
	cases := []struct {
		name, data string
		want       []string
	}{
		{"inside", "120 120 40 40 re W n 0 0 300 300 re f",
			[]string{"0 0 0 0 re\n", "W\n", "n\n", "0 0 300 300 re\n", "f\n"}},
		{"inside even-odd fill", "120 120 40 40 re W* f",
			[]string{"0 0 0 0 re\n", "W*\n", "n\n"}},
		{"crossing", "150 150 100 40 re W n",
			[]string{"150 150 100 40 re\n", "W\n", "n\n"}},
		{"partly", "120 120 40 40 re 10 10 20 20 re W n",
			[]string{"10 10 20 20 re\n", "W\n", "n\n"}},
		{"no clip", "120 120 40 40 re n", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := pathOnly(redactPath(t, c.data))
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Errorf("path changed (-want +got):\n%s", diff)
			}
		})
	}
}

// TestPathStateUnaffected checks that a stroked path outside the region
// survives with its graphics state, while the filled path inside is removed.
func TestPathStateUnaffected(t *testing.T) {
	ops := redactPath(t, "q 2 w 120 120 40 40 re f 10 10 20 20 re S Q")
	want := []string{"10 10 20 20 re\n", "S\n"}
	if diff := cmp.Diff(want, pathOnly(ops)); diff != "" {
		t.Errorf("paths changed (-want +got):\n%s", diff)
	}
	var width pdf.Object
	for _, op := range ops {
		if op.name == content.OpSetLineWidth {
			width = op.args[0]
		}
	}
	if w, ok := width.(pdf.Integer); !ok || w != 2 {
		t.Errorf("line width changed to %v", width)
	}
}

func TestPathLosesMarkedContent(t *testing.T) {
	data := "/Span <</ActualText (secret)>> BDC 120 120 40 40 re f EMC"
	ops := redactPath(t, data)
	for _, op := range ops {
		if op.name == content.OpBeginMarkedContentWithProperties {
			if d, ok := op.args[1].(pdf.Dict); ok {
				if _, has := d["ActualText"]; has {
					t.Errorf("/ActualText survives")
				}
			}
		}
	}
}

// TestPathBudgetPoints checks that the work for testing a path against the
// regions grows with the number of points of the path, not only with the
// number of subpaths.
func TestPathBudgetPoints(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("0 0 m\n")
	for i := range 40000 {
		fmt.Fprintf(&sb, "%d %d l\n", 150+i%100, 150+i%7)
	}
	sb.WriteString("S\n")
	for _, tc := range []struct {
		regions int
		fails   bool
	}{{2, false}, {2000, true}} {
		in := textSource(t, sb.String(), manyRegions(tc.regions)...)
		err := Write(io.Discard, in, nil)
		if got := errors.Is(err, ErrTooComplex); got != tc.fails || !tc.fails && err != nil {
			t.Errorf("%d regions: got error %v, want ErrTooComplex: %t", tc.regions, err, tc.fails)
		}
	}
}
