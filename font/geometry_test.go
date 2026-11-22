// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package font

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/sfnt/glyph"
)

func TestBoundingBox(t *testing.T) {
	geom := &Geometry{
		GlyphExtents: []rect.Rect{
			{}, // .notdef
			{}, // space
			{LLx: 0.125, LLy: -0.125, URx: 0.5, URy: 0.75},                             // A
			{LLx: math.Inf(-1), LLy: math.Inf(-1), URx: math.Inf(1), URy: math.Inf(1)}, // unbounded
		},
	}
	infinite := &pdf.Rectangle{LLx: math.Inf(-1), LLy: math.Inf(-1), URx: math.Inf(1), URy: math.Inf(1)}
	const size = 8 // exact binary values, for exact comparisons

	cases := []struct {
		name string
		seq  *GlyphSeq
		want *pdf.Rectangle
	}{
		{"blank only", &GlyphSeq{Seq: []Glyph{{GID: 1, Advance: 3}}}, &pdf.Rectangle{}},
		{
			"one glyph",
			&GlyphSeq{Seq: []Glyph{{GID: 2, Advance: 5}}},
			&pdf.Rectangle{LLx: 1, LLy: -1, URx: 4, URy: 6},
		},
		{
			"blank between",
			&GlyphSeq{Seq: []Glyph{{GID: 2, Advance: 5}, {GID: 1, Advance: 3}, {GID: 2, Advance: 5}}},
			&pdf.Rectangle{LLx: 1, LLy: -1, URx: 12, URy: 6},
		},
		{
			"leading blanks",
			&GlyphSeq{Seq: []Glyph{{GID: 1, Advance: 3}, {GID: 1, Advance: 3}, {GID: 2, Advance: 5}}},
			&pdf.Rectangle{LLx: 7, LLy: -1, URx: 10, URy: 6},
		},
		{
			"skip and rise",
			&GlyphSeq{Skip: 2, Seq: []Glyph{{GID: 1, Advance: 3}, {GID: 2, Advance: 5, Rise: 1}}},
			&pdf.Rectangle{LLx: 6, LLy: 0, URx: 9, URy: 7},
		},
		{
			"unbounded glyph",
			&GlyphSeq{Seq: []Glyph{{GID: 2, Advance: 5}, {GID: 3, Advance: 5}}},
			infinite,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := geom.BoundingBox(size, c.seq)
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Errorf("wrong bounding box (-want +got):\n%s", diff)
			}
		})
	}
}

func TestIsUnbounded(t *testing.T) {
	geom := &Geometry{
		GlyphExtents: []rect.Rect{
			{}, // blank
			{LLx: 0.125, LLy: -0.125, URx: 0.5, URy: 0.75},
			{LLx: math.Inf(-1), LLy: math.Inf(-1), URx: math.Inf(1), URy: math.Inf(1)},
		},
	}
	for gid, want := range []bool{false, false, true} {
		if got := geom.IsUnbounded(glyph.ID(gid)); got != want {
			t.Errorf("glyph %d: IsUnbounded() = %t, want %t", gid, got, want)
		}
	}
}
