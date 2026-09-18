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
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/geom/path"
	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
)

func TestDrawPath(t *testing.T) {
	var d path.Data
	d.MoveTo(vec.Vec2{X: 1.005, Y: 2})
	d.LineTo(vec.Vec2{X: 3, Y: 4.567})
	d.CubeTo(
		vec.Vec2{X: 5, Y: 6},
		vec.Vec2{X: 7, Y: 8},
		vec.Vec2{X: 9, Y: 10},
	)
	d.Close()

	b := New(content.Page, nil, pdf.V2_0)
	b.DrawPath(d.Iter(), 2)
	b.Stroke()

	stream, err := b.Harvest()
	if err != nil {
		t.Fatal(err)
	}

	want := []content.Operator{
		{Name: content.OpMoveTo, Args: []pdf.Object{pdf.Number(1), pdf.Number(2)}},
		{Name: content.OpLineTo, Args: []pdf.Object{pdf.Number(3), pdf.Number(4.57)}},
		{Name: content.OpCurveTo, Args: []pdf.Object{
			pdf.Number(5), pdf.Number(6),
			pdf.Number(7), pdf.Number(8),
			pdf.Number(9), pdf.Number(10),
		}},
		{Name: content.OpClosePath},
		{Name: content.OpStroke},
	}
	if diff := cmp.Diff(want, stream.Ops); diff != "" {
		t.Errorf("unexpected operators (-want +got):\n%s", diff)
	}
}

// TestDrawPathQuadratic checks that quadratic segments reach the content
// stream as the cubic segments which describe the same curve.
func TestDrawPathQuadratic(t *testing.T) {
	var d path.Data
	d.MoveTo(vec.Vec2{X: 0, Y: 0})
	d.QuadTo(vec.Vec2{X: 3, Y: 3}, vec.Vec2{X: 6, Y: 0})

	b := New(content.Page, nil, pdf.V2_0)
	b.DrawPath(d.Iter(), 2)
	b.Fill()

	stream, err := b.Harvest()
	if err != nil {
		t.Fatal(err)
	}

	want := []content.Operator{
		{Name: content.OpMoveTo, Args: []pdf.Object{pdf.Number(0), pdf.Number(0)}},
		{Name: content.OpCurveTo, Args: []pdf.Object{
			pdf.Number(2), pdf.Number(2),
			pdf.Number(4), pdf.Number(2),
			pdf.Number(6), pdf.Number(0),
		}},
		{Name: content.OpFill},
	}
	if diff := cmp.Diff(want, stream.Ops); diff != "" {
		t.Errorf("unexpected operators (-want +got):\n%s", diff)
	}
}

func TestDrawPathEmpty(t *testing.T) {
	b := New(content.Page, nil, pdf.V2_0)
	b.DrawPath(path.Empty, 2)

	stream, err := b.Harvest()
	if err != nil {
		t.Fatal(err)
	}
	if len(stream.Ops) != 0 {
		t.Errorf("got %d operators, want none", len(stream.Ops))
	}
}
