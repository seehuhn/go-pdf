// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package content

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPathPainting(t *testing.T) {
	want := map[OpName]PathPaint{
		OpStroke:                    {Stroke: true},
		OpCloseAndStroke:            {Stroke: true, Close: true},
		OpFill:                      {Fill: true},
		OpFillCompat:                {Fill: true},
		OpFillEvenOdd:               {Fill: true, EvenOdd: true},
		OpFillAndStroke:             {Fill: true, Stroke: true},
		OpFillAndStrokeEvenOdd:      {Fill: true, Stroke: true, EvenOdd: true},
		OpCloseFillAndStroke:        {Fill: true, Stroke: true, Close: true},
		OpCloseFillAndStrokeEvenOdd: {Fill: true, Stroke: true, EvenOdd: true, Close: true},
		OpEndPath:                   {},
	}
	for name := range operators {
		got, ok := PathPainting(name)
		exp, isPath := want[name]
		if ok != isPath {
			t.Errorf("%s: ok = %t, want %t", name, ok, isPath)
			continue
		}
		if diff := cmp.Diff(exp, got); diff != "" {
			t.Errorf("%s: (-want +got):\n%s", name, diff)
		}
	}
	for name := range want {
		if !Known(name) {
			t.Errorf("%s is not in the operator table", name)
		}
	}
}

func TestPaintKind(t *testing.T) {
	want := map[OpName]PaintKind{
		OpTextShow:                       PaintText,
		OpTextShowArray:                  PaintText,
		OpTextShowMoveNextLine:           PaintText,
		OpTextShowMoveNextLineSetSpacing: PaintText,
		OpShading:                        PaintShading,
		OpXObject:                        PaintXObject,
		OpInlineImage:                    PaintInlineImage,
	}
	for name := range operators {
		exp, ok := want[name]
		if !ok {
			if _, isPath := PathPainting(name); isPath {
				exp = PaintPath
			} else {
				exp = PaintNone
			}
		}
		if got := name.PaintKind(); got != exp {
			t.Errorf("%s: got %d, want %d", name, got, exp)
		}
	}
	for name := range want {
		if !Known(name) {
			t.Errorf("%s is not in the operator table", name)
		}
	}
}
