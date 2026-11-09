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

package font

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
)

func TestBoundingBox(t *testing.T) {
	geom := &Geometry{
		GlyphExtents: []rect.Rect{
			{}, // .notdef
			{}, // space
			{LLx: 0.125, LLy: -0.125, URx: 0.5, URy: 0.75}, // A
		},
	}
	const size = 8 // exact binary values, for exact comparisons

	cases := []struct {
		name string
		seq  *GlyphSeq
		want *pdf.Rectangle
	}{
		{"nil", nil, &pdf.Rectangle{}},
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
