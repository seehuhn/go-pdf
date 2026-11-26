// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redactmark

import (
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
)

func TestDrawText(t *testing.T) {
	F := font.Must(standard.Helvetica.New())
	rect := pdf.Rectangle{LLx: 10, LLy: 20, URx: 210, URy: 60}
	// a quadrilateral rotated by 30 degrees
	rotated := Quad{{X: 15, Y: 176}, {X: 84.28, Y: 216}, {X: 30, Y: 150}, {X: 99.28, Y: 190}}

	for _, quad := range []Quad{RectQuad(rect), rotated} {
		for _, repeat := range []bool{false, true} {
			for _, size := range []float64{0, 10} {
				b := builder.New(content.Form, nil, pdf.V1_7)
				Draw(b, &Mark{
					Quads: []Quad{quad},
					Rect:  rect,
					Fill:  color.DeviceGray(0),
					Text: &Text{
						Font:   F,
						Size:   size,
						Color:  color.DeviceGray(1),
						String: "REDACTED",
						Align:  pdf.TextAlignCenter,
						Repeat: repeat,
					},
				})
				if _, err := b.Harvest(); err != nil {
					t.Errorf("repeat=%t size=%g: %v", repeat, size, err)
				}
				if len(b.Resources.Font) == 0 {
					t.Errorf("repeat=%t size=%g: no text drawn", repeat, size)
				}
			}
		}
	}
}

func TestDrawOverlay(t *testing.T) {
	overlay := &form.Form{Res: &content.Resources{}, BBox: pdf.Rectangle{URx: 10, URy: 10}}
	b := builder.New(content.Form, nil, pdf.V1_7)
	Draw(b, &Mark{
		Quads:   []Quad{RectQuad(pdf.Rectangle{URx: 10, URy: 10})},
		Overlay: overlay,
		Fill:    color.DeviceGray(0),
	})
	if _, err := b.Harvest(); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, x := range b.Resources.XObject {
		found = found || x == overlay
	}
	if !found {
		t.Error("overlay not drawn")
	}
}

func TestQuadsFromPoints(t *testing.T) {
	pts := make([]vec.Vec2, 9)
	if n := len(QuadsFromPoints(pts)); n != 2 {
		t.Errorf("got %d quadrilaterals, want 2", n)
	}
}

// TestDrawTextBounded checks that the overlay text drawn for a long string
// over many regions stays small.
func TestDrawTextBounded(t *testing.T) {
	F := font.Must(standard.Helvetica.New())
	quads := make([]Quad, 1000)
	for i := range quads {
		quads[i] = RectQuad(pdf.Rectangle{URx: 500, URy: 200})
	}
	text := strings.Repeat("REDACTED ", 50000)

	for _, repeat := range []bool{false, true} {
		b := builder.New(content.Form, nil, pdf.V1_7)
		Draw(b, &Mark{
			Quads: quads,
			Text: &Text{
				Font:   F,
				Size:   1,
				Color:  color.DeviceGray(0),
				String: text,
				Repeat: repeat,
			},
		})
		ops, err := b.Harvest()
		if err != nil {
			t.Fatal(err)
		}
		r, err := ops.RawBytes()
		if err != nil {
			t.Fatal(err)
		}
		n, err := io.Copy(io.Discard, r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		// drawing every glyph would take several hundred megabytes
		if n > 4<<20 {
			t.Errorf("repeat=%t: content stream has %d bytes", repeat, n)
		}
	}
}

// TestLineWindow compares the glyphs kept by line with those whose ink
// intersects the window.
func TestLineWindow(t *testing.T) {
	F := font.Must(standard.Helvetica.New())
	geom := F.GetGeometry()
	l := newTextLayout(F, "Wave AVATAR, fjord ")
	unit := F.Layout(nil, 1, "Wave AVATAR, fjord ")

	for _, size := range []float64{1, 7.5} {
		for _, reps := range []float64{1, 4} {
			for _, x0 := range []float64{-30, 0, 12} {
				for _, win := range [][2]float64{{0, 40}, {-5, 3}, {25, 26}, {-1000, 1000}} {
					lo, hi := win[0], win[1]

					var want []float64
					var wantText string
					for k := range int(reps) {
						x := x0 + float64(k)*unit.TotalWidth()*size + unit.Skip*size
						for _, g := range unit.Seq {
							ext := geom.GlyphExtents[g.GID]
							if x+ext.URx*size >= lo && x+ext.LLx*size <= hi {
								want = append(want, x)
								wantText += g.Text
							}
							x += g.Advance * size
						}
					}

					seq, _ := l.line(x0, size, reps, lo, hi, maxGlyphs)
					if seq.Text() != wantText {
						t.Errorf("size=%g reps=%g x0=%g window=%v: got %q, want %q",
							size, reps, x0, win, seq.Text(), wantText)
						continue
					}
					x := seq.Skip
					for i, g := range seq.Seq {
						if math.Abs(x-want[i]) > 1e-9 {
							t.Errorf("size=%g reps=%g x0=%g window=%v: glyph %d at %g, want %g",
								size, reps, x0, win, i, x, want[i])
							break
						}
						x += g.Advance
					}
				}
			}
		}
	}
}

func TestFromAnnotation(t *testing.T) {
	rect := pdf.Rectangle{LLx: 10, LLy: 20, URx: 110, URy: 60}

	a := &annotation.Redact{Rect: rect}
	m := FromAnnotation(a, func() font.Layouter {
		t.Error("font requested for an annotation without overlay text")
		return nil
	})
	if len(m.Quads) != 1 || m.Quads[0] != RectQuad(rect) {
		t.Errorf("quads = %v, want the rectangle", m.Quads)
	}
	if m.Text != nil {
		t.Error("mark has text but the annotation has none")
	}

	pts := []vec.Vec2{{X: 0, Y: 1}, {X: 1, Y: 1}, {X: 0, Y: 0}, {X: 1, Y: 0}}
	a = &annotation.Redact{
		Rect:              rect,
		QuadPoints:        pts,
		FillColor:         color.DeviceRGB{0, 0, 0},
		OverlayText:       pdf.Text("X"),
		DefaultAppearance: "/Helv 9 Tf 1 g",
		Repeat:            true,
	}
	m = FromAnnotation(a, func() font.Layouter { return nil })
	if len(m.Quads) != 1 || m.Quads[0] != Quad(pts) {
		t.Errorf("quads = %v, want the quad points", m.Quads)
	}
	if m.Fill != a.FillColor {
		t.Errorf("fill = %v, want %v", m.Fill, a.FillColor)
	}
	if m.Text == nil {
		t.Fatal("mark has no text")
	}
	if m.Text.Size != 9 || m.Text.Color != color.Color(color.DeviceGray(1)) ||
		m.Text.String != "X" || !m.Text.Repeat {
		t.Errorf("text = %+v, want size 9, white, \"X\", repeated", m.Text)
	}
}

// Fewer than four points give no quadrilateral, so the region is the
// rectangle; trailing points which do not fill a quadrilateral are ignored.
func TestFromAnnotationPartialQuadPoints(t *testing.T) {
	rect := pdf.Rectangle{LLx: 10, LLy: 20, URx: 110, URy: 60}
	pts := make([]vec.Vec2, 9)
	for i := range pts {
		pts[i] = vec.Vec2{X: float64(i), Y: float64(i * i)}
	}
	for n := 1; n <= 3; n++ {
		a := &annotation.Redact{Rect: rect, QuadPoints: pts[:n]}
		m := FromAnnotation(a, func() font.Layouter { return nil })
		if len(m.Quads) != 1 || m.Quads[0] != RectQuad(rect) {
			t.Errorf("%d points: quads = %v, want the rectangle", n, m.Quads)
		}
	}
	for _, n := range []int{5, 7, 9} {
		a := &annotation.Redact{Rect: rect, QuadPoints: pts[:n]}
		m := FromAnnotation(a, func() font.Layouter { return nil })
		if len(m.Quads) != n/4 || m.Quads[0] != Quad(pts[:4]) {
			t.Errorf("%d points: quads = %v, want %d from the points", n, m.Quads, n/4)
		}
	}
}

// The font getter is called when there is overlay text, and its result is
// used for the text.
func TestFromAnnotationFont(t *testing.T) {
	var f font.Layouter = font.Must(standard.Helvetica.New())
	calls := 0
	a := &annotation.Redact{
		Rect:        pdf.Rectangle{URx: 10, URy: 10},
		OverlayText: pdf.Text("X"),
	}
	m := FromAnnotation(a, func() font.Layouter { calls++; return f })
	if calls != 1 || m.Text == nil || m.Text.Font != f {
		t.Errorf("calls = %d, text = %+v, want one call and the font set", calls, m.Text)
	}
}

// countGlyphs returns the number of glyphs which the text-showing operators
// of a content stream show.
func countGlyphs(t *testing.T, ops []content.Operator) int {
	t.Helper()
	n := 0
	for _, op := range ops {
		switch op.Name {
		case content.OpTextShow:
			s, _ := op.Args[0].(pdf.String)
			n += len(s)
		case content.OpTextShowArray:
			a, _ := op.Args[0].(pdf.Array)
			for _, x := range a {
				if s, ok := x.(pdf.String); ok {
					n += len(s)
				}
			}
		}
	}
	return n
}

func TestDrawCharged(t *testing.T) {
	F := font.Must(standard.Helvetica.New())
	mark := &Mark{
		Quads: []Quad{RectQuad(pdf.Rectangle{URx: 100, URy: 100}), RectQuad(pdf.Rectangle{URx: 50, URy: 50})},
		Fill:  color.DeviceGray(0),
		Text: &Text{
			Font: F, Size: 4, Color: color.DeviceGray(1),
			String: "AB", Repeat: true,
		},
	}

	// every glyph shown is charged
	var total int64
	b := builder.New(content.Form, nil, pdf.V1_7)
	err := DrawCharged(b, mark, func(n int64) error {
		total += n
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	ops, err := b.Harvest()
	if err != nil {
		t.Fatal(err)
	}
	if glyphs := countGlyphs(t, ops.Ops); glyphs == 0 || total < int64(glyphs) {
		t.Errorf("charged %d for %d glyphs", total, glyphs)
	}

	// an error of the charge function ends the drawing
	errBudget := errors.New("budget")
	left := total / 2
	b = builder.New(content.Form, nil, pdf.V1_7)
	err = DrawCharged(b, mark, func(n int64) error {
		left -= n
		if left < 0 {
			return errBudget
		}
		return nil
	})
	if err != errBudget {
		t.Errorf("got error %v, want the error of the charge function", err)
	}
}
