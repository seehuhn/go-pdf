// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package fallback

import (
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/color"
)

// TestShortLineInsetEndings checks that a line shorter than the setbacks of
// its two endings draws only the endings, and no line running backwards
// between their connection points.
func TestShortLineInsetEndings(t *testing.T) {
	a := &annotation.Line{
		Color:  color.DeviceRGB{1, 0, 0},
		Border: &annotation.Border{Width: 1},
		Coords: [4]float64{100, 100, 105, 100},
		LineEndingStyle: [2]annotation.LineEndingStyle{
			annotation.LineEndingStyleDiamond,
			annotation.LineEndingStyleDiamond,
		},
	}
	g := newGen(t, pdf.V2_0)
	if err := g.AddAppearance(a); err != nil {
		t.Fatal(err)
	}

	stream := string(appearanceStream(t, a))
	if n := strings.Count(stream, "\ns\n"); n != 2 {
		t.Errorf("found %d diamonds, want 2:\n%s", n, stream)
	}
	if n := strings.Count(stream, "\nS\n"); n != 0 {
		t.Errorf("found %d line strokes, want none:\n%s", n, stream)
	}
}

// TestPolyLineShortEndSegment checks that a polyline whose first segment is
// shorter than the setback of its line ending starts at the inner end of
// that segment.
func TestPolyLineShortEndSegment(t *testing.T) {
	a := &annotation.PolyLine{
		Color:    color.DeviceRGB{1, 0, 0},
		Border:   &annotation.Border{Width: 1},
		Vertices: []float64{100, 100, 102, 100, 102, 150},
		LineEndingStyle: [2]annotation.LineEndingStyle{
			annotation.LineEndingStyleDiamond,
			annotation.LineEndingStyleNone,
		},
	}
	g := newGen(t, pdf.V2_0)
	if err := g.AddAppearance(a); err != nil {
		t.Fatal(err)
	}

	stream := string(appearanceStream(t, a))
	if !strings.Contains(stream, "102 100 m\n102 150 l\nS\n") {
		t.Errorf("the path does not start at the second vertex:\n%s", stream)
	}
}
