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

package fallback

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/color"
)

// TestLeaderLineGeometry checks the layout of a horizontal line with leader
// lines.  The leaders run perpendicular to the line, to its left looking from
// the start towards the end, so for a line running to the right they go up.
func TestLeaderLineGeometry(t *testing.T) {
	a := &annotation.Line{
		Coords: [4]float64{0, 0, 100, 0},
		LL:     20, // leader length, up to the line proper
		LLO:    5,  // gap between the coords and where the leaders start
		LLE:    3,  // leader extension past the line proper
	}

	want := lineGeometry{
		main: []vec.Vec2{{X: 0, Y: 25}, {X: 100, Y: 25}},
		leaders: [][]vec.Vec2{
			{{X: 0, Y: 5}, {X: 0, Y: 28}},
			{{X: 100, Y: 5}, {X: 100, Y: 28}},
		},
	}
	if diff := cmp.Diff(want, newLineGeometry(a), cmp.AllowUnexported(lineGeometry{})); diff != "" {
		t.Errorf("geometry mismatch (-want +got):\n%s", diff)
	}
}

// TestLeaderLineDirection checks that the sign of LL and the direction of the
// line together choose the side the leaders stand on.
func TestLeaderLineDirection(t *testing.T) {
	cases := []struct {
		coords [4]float64
		ll     float64
		wantY  float64
	}{
		{[4]float64{0, 0, 100, 0}, 20, 20},
		{[4]float64{100, 0, 0, 0}, 20, -20},
		{[4]float64{0, 0, 100, 0}, -20, -20},
		{[4]float64{100, 0, 0, 0}, -20, 20},
	}
	for _, c := range cases {
		a := &annotation.Line{Coords: c.coords, LL: c.ll, LLE: 2}
		geom := newLineGeometry(a)
		if y := geom.main[0].Y; y != c.wantY {
			t.Errorf("L=%v LL=%g: line proper at y=%g, want %g", c.coords, c.ll, y, c.wantY)
		}
		// the extension continues past the line proper, away from the coords
		if y := geom.leaders[0][1].Y; math.Abs(y) != math.Abs(c.wantY)+2 || y*c.wantY < 0 {
			t.Errorf("L=%v LL=%g: leader ends at y=%g", c.coords, c.ll, y)
		}
	}
}

// TestLeaderLineShortLine checks that a line too short to have a direction
// is laid out without leader lines.
func TestLeaderLineShortLine(t *testing.T) {
	a := &annotation.Line{Coords: [4]float64{10, 10, 10.05, 10}, LL: 20}

	geom := newLineGeometry(a)
	if geom.leaders != nil {
		t.Errorf("got leaders %v", geom.leaders)
	}
	want := []vec.Vec2{{X: 10, Y: 10}, {X: 10.05, Y: 10}}
	if diff := cmp.Diff(want, geom.main); diff != "" {
		t.Errorf("line mismatch (-want +got):\n%s", diff)
	}
}

// TestLeaderLineEndingsInRect checks that line endings on a line with leader
// lines are measured where they are drawn, on the line proper.
func TestLeaderLineEndingsInRect(t *testing.T) {
	a := &annotation.Line{
		Common: annotation.Common{
			Color:  color.DeviceRGB{1, 0, 0},
			Border: &annotation.Border{Width: 1},
		},
		Coords: [4]float64{0, 0, 100, 0},
		LL:     20,
		LineEndingStyle: [2]annotation.LineEndingStyle{
			annotation.LineEndingStyleClosedArrow,
			annotation.LineEndingStyleClosedArrow,
		},
	}

	g := newGen(t, pdf.V2_0)
	if err := g.AddAppearance(a); err != nil {
		t.Fatal(err)
	}

	var want pdf.Rectangle
	geom := newLineGeometry(a)
	p1, p2 := geom.main[0], geom.main[1]
	lineEndingBBox(&want, annotation.LineEndingStyleClosedArrow, lineEndingInfo{At: p1, Dir: p1.Sub(p2)}, 1)
	lineEndingBBox(&want, annotation.LineEndingStyleClosedArrow, lineEndingInfo{At: p2, Dir: p2.Sub(p1)}, 1)
	if want.URy <= 20.5 {
		t.Fatalf("test case gives no arrow wider than the line: %v", want)
	}
	r := a.Rect
	if r.LLx > want.LLx || r.LLy > want.LLy || r.URx < want.URx || r.URy < want.URy {
		t.Errorf("rectangle %v does not hold the arrows %v", a.Rect, want)
	}
}

// TestLineWithLeaders drives the generator with a line which has leaders, and
// checks that the appearance draws them and stays inside the rectangle.
func TestLineWithLeaders(t *testing.T) {
	a := &annotation.Line{
		Common: annotation.Common{
			Color:  color.DeviceRGB{0, 0, 1},
			Border: &annotation.Border{Width: 2},
		},
		Coords: [4]float64{20, 20, 120, 60},
		LL:     15,
		LLO:    4,
		LLE:    3,
	}

	g := newGen(t, pdf.V2_0)
	if err := g.AddAppearance(a); err != nil {
		t.Fatal(err)
	}

	// the two leaders and the line between them
	if n := countStrokes(t, a); n < 3 {
		t.Errorf("the line drew %d strokes, want at least 3", n)
	}

	// the leaders and the line proper have to fit inside the rectangle the
	// appearance is placed in
	geom := newLineGeometry(a)
	for _, seg := range append([][]vec.Vec2{geom.main}, geom.leaders...) {
		for _, p := range seg {
			if !a.Rect.Contains(p) {
				t.Errorf("point %v lies outside rect %v", p, a.Rect)
			}
		}
	}
}

// TestLineWithShortLeaders checks that a line too short to have a direction
// falls back to the plain line rather than dividing by its length.
func TestLineWithShortLeaders(t *testing.T) {
	a := &annotation.Line{
		Common: annotation.Common{
			Color:  color.DeviceRGB{0, 0, 1},
			Border: &annotation.Border{Width: 1},
		},
		Coords: [4]float64{30, 30, 30.01, 30},
		LL:     15,
	}

	g := newGen(t, pdf.V2_0)
	if err := g.AddAppearance(a); err != nil {
		t.Fatal(err)
	}
	if n := countStrokes(t, a); n == 0 {
		t.Error("the line stroked nothing")
	}
}

// captionLine returns a horizontal line annotation from (100, 100) to
// (300, 100), with a caption.
func captionLine(above bool, offset []float64) *annotation.Line {
	return &annotation.Line{
		Common: annotation.Common{
			Color:    color.DeviceRGB{0, 0, 1},
			Border:   &annotation.Border{Width: 1},
			Contents: "caption",
		},
		Coords:        [4]float64{100, 100, 300, 100},
		Caption:       true,
		CaptionAbove:  above,
		CaptionOffset: offset,
	}
}

// TestLineCaptionInline checks that an inline caption is shown centred on
// the line, which is broken round it, and that the rectangle holds it.
func TestLineCaptionInline(t *testing.T) {
	a := captionLine(false, nil)
	g := newGen(t, pdf.V2_0)
	capt := g.layoutCaption(a, newLineGeometry(a).main, 1)
	if err := g.AddAppearance(a); err != nil {
		t.Fatal(err)
	}

	stream := string(appearanceStream(t, a))
	if !strings.Contains(stream, "Tj") && !strings.Contains(stream, "TJ") {
		t.Error("the caption is not shown")
	}
	if n := countStrokes(t, a); n != 2 {
		t.Errorf("the line was drawn in %d strokes, want 2", n)
	}

	// the gap is centred on the line and wider than the text
	width := capt.lines[0].TotalWidth()
	if got := capt.gap[0] + capt.gap[1]; math.Abs(got-200) > 1e-9 {
		t.Errorf("gap %v is not centred on the line", capt.gap)
	}
	if got := capt.gap[1] - capt.gap[0]; got <= width {
		t.Errorf("gap %v is no wider than the text, %g", capt.gap, width)
	}
	// the capitals are centred on the line
	if y := capt.tm[0][5]; y >= 100 || y < 100-captionFontSize {
		t.Errorf("baseline at y=%g, want a little below the line", y)
	}

	r, cb := a.Rect, capt.bbox
	if r.LLx > cb.LLx || r.LLy > cb.LLy || r.URx < cb.URx || r.URy < cb.URy {
		t.Errorf("rectangle %v does not hold the caption %v", r, cb)
	}
}

// TestLineCaptionAbove checks that a caption above the line stands on it,
// and that the line is not broken.
func TestLineCaptionAbove(t *testing.T) {
	a := captionLine(true, nil)
	g := newGen(t, pdf.V2_0)
	capt := g.layoutCaption(a, newLineGeometry(a).main, 1)
	if err := g.AddAppearance(a); err != nil {
		t.Fatal(err)
	}

	if capt.gapped {
		t.Error("a caption above the line breaks it")
	}
	if n := countStrokes(t, a); n != 1 {
		t.Errorf("the line was drawn in %d strokes, want 1", n)
	}
	// the descenders reach down to the edge of the line, up to rounding
	descent := g.ContentFont().GetGeometry().Descent * captionFontSize
	if bottom := capt.tm[0][5] + descent; bottom < 100.5-0.01 {
		t.Errorf("descenders reach down to y=%g, into the line", bottom)
	}
	if a.Rect.URy < capt.bbox.URy {
		t.Errorf("rectangle %v does not hold the caption %v", a.Rect, capt.bbox)
	}
}

// TestLineCaptionOffset checks that the caption offset moves the text, and
// the gap with it, along the line and across it.
func TestLineCaptionOffset(t *testing.T) {
	g := newGen(t, pdf.V2_0)
	plain := captionLine(false, nil)
	moved := captionLine(false, []float64{20, 3})
	c0 := g.layoutCaption(plain, newLineGeometry(plain).main, 1)
	c1 := g.layoutCaption(moved, newLineGeometry(moved).main, 1)

	if dx := c1.tm[0][4] - c0.tm[0][4]; math.Abs(dx-20) > 0.011 {
		t.Errorf("text moved by %g along the line, want 20", dx)
	}
	if dy := c1.tm[0][5] - c0.tm[0][5]; math.Abs(dy-3) > 0.011 {
		t.Errorf("text moved by %g across the line, want 3", dy)
	}
	if dx := c1.gap[0] - c0.gap[0]; math.Abs(dx-20) > 1e-9 {
		t.Errorf("gap moved by %g, want 20", dx)
	}
}

// TestLineCaptionFollowsDirection checks that the caption of a line running
// from right to left runs with it, upside down, and that a caption above
// the line stands on the left of the line's direction, which is below it.
func TestLineCaptionFollowsDirection(t *testing.T) {
	a := captionLine(true, nil)
	a.Coords = [4]float64{300, 100, 100, 100}
	g := newGen(t, pdf.V2_0)
	capt := g.layoutCaption(a, newLineGeometry(a).main, 1)

	M := capt.tm[0]
	if M[0] != -1 || M[3] != -1 {
		t.Errorf("text matrix %v, want upside-down text", M)
	}
	if y := capt.tm[0][5]; y > 100 {
		t.Errorf("baseline at y=%g, above the line", y)
	}
}

// TestLineCaptionMultiLine checks that each line of a caption is shown on
// its own, one below the other.
func TestLineCaptionMultiLine(t *testing.T) {
	a := captionLine(false, nil)
	a.Contents = "one\r\ntwo\rthree"
	g := newGen(t, pdf.V2_0)
	capt := g.layoutCaption(a, newLineGeometry(a).main, 1)

	if len(capt.lines) != 3 {
		t.Fatalf("got %d caption lines, want 3", len(capt.lines))
	}
	for i := 1; i < 3; i++ {
		if capt.tm[i][5] >= capt.tm[i-1][5] {
			t.Errorf("line %d is not below line %d", i, i-1)
		}
	}
}

// TestLineCaptionWiderThanLine checks that a caption wider than its line
// leaves the line endings in place.
func TestLineCaptionWiderThanLine(t *testing.T) {
	a := captionLine(false, nil)
	a.Coords = [4]float64{100, 100, 110, 100}
	a.Contents = "a caption much wider than the line"
	a.LineEndingStyle = [2]annotation.LineEndingStyle{
		annotation.LineEndingStyleClosedArrow,
		annotation.LineEndingStyleClosedArrow,
	}
	g := newGen(t, pdf.V2_0)
	capt := g.layoutCaption(a, newLineGeometry(a).main, 1)
	if capt.gap != [2]float64{0, 10} {
		t.Errorf("gap %v, want the whole line", capt.gap)
	}
	if err := g.AddAppearance(a); err != nil {
		t.Fatal(err)
	}
	stream := string(appearanceStream(t, a))
	if n := strings.Count(stream, "\ns\n") + strings.Count(stream, "\nb\n"); n != 2 {
		t.Errorf("found %d closed arrows, want 2:\n%s", n, stream)
	}
	// the caption covers the whole line, so no half of it is left to draw
	if n := strings.Count(stream, "\nS\n"); n != 0 {
		t.Errorf("found %d line strokes, want none:\n%s", n, stream)
	}
}

// TestLineCaptionDashPhase checks that the dashes of a line broken by an
// inline caption run on across the gap, as though the line continued.
func TestLineCaptionDashPhase(t *testing.T) {
	a := captionLine(false, nil)
	a.BorderStyle = &annotation.BorderStyle{Width: 1, Style: "D", DashArray: []float64{3, 2}}
	g := newGen(t, pdf.V2_0)
	capt := g.layoutCaption(a, newLineGeometry(a).main, 1)
	if err := g.AddAppearance(a); err != nil {
		t.Fatal(err)
	}

	var phases []float64
	tokens := strings.Fields(string(appearanceStream(t, a)))
	for i, tok := range tokens {
		// the solid dash the stream starts with is not part of the line
		if tok == "d" && i > 1 && tokens[i-2] != "[]" {
			x, err := strconv.ParseFloat(tokens[i-1], 64)
			if err != nil {
				t.Fatalf("bad dash phase %q", tokens[i-1])
			}
			phases = append(phases, x)
		}
	}
	want := []float64{0, pdf.Round(capt.gap[1], 2), 0}
	if len(phases) != len(want) {
		t.Fatalf("dash phases %v, want %v", phases, want)
	}
	for i := range want {
		if math.Abs(phases[i]-want[i]) > 0.011 {
			t.Errorf("dash phases %v, want %v", phases, want)
			break
		}
	}
}
