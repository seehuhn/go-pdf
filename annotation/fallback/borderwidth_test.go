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
	"bytes"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/graphics/color"
)

// borderCases returns, for each annotation type whose fallback appearance
// draws a border, a pair of otherwise identical annotations: one asking for no
// border and one asking for a border of width 1.
func borderCases() map[string][2]annotation.Annotation {
	rect := pdf.Rectangle{LLx: 10, LLy: 10, URx: 190, URy: 90}
	verts := []float64{10, 10, 190, 10, 100, 90}
	ink := [][]vec.Vec2{{{X: 10, Y: 10}, {X: 100, Y: 50}, {X: 190, Y: 90}}}
	col := color.DeviceRGB{1, 0, 0}

	common := func(b *annotation.Border) annotation.Common {
		return annotation.Common{Rect: rect, Border: b, Color: col}
	}
	// two calls, so that the two annotations never share a Border value
	no := func() *annotation.Border { return nil }
	one := func() *annotation.Border { return &annotation.Border{Width: 1} }

	build := func(f func(*annotation.Border) annotation.Annotation) [2]annotation.Annotation {
		return [2]annotation.Annotation{f(no()), f(one())}
	}

	return map[string][2]annotation.Annotation{
		"Square": build(func(b *annotation.Border) annotation.Annotation {
			return &annotation.Square{Common: common(b)}
		}),
		"Circle": build(func(b *annotation.Border) annotation.Annotation {
			return &annotation.Circle{Common: common(b)}
		}),
		"Caret": build(func(b *annotation.Border) annotation.Annotation {
			return &annotation.Caret{Common: common(b)}
		}),
		"Polygon": build(func(b *annotation.Border) annotation.Annotation {
			return &annotation.Polygon{Common: common(b), Vertices: verts}
		}),
		"PolyLine": build(func(b *annotation.Border) annotation.Annotation {
			return &annotation.PolyLine{Common: common(b), Vertices: verts}
		}),
		"Ink": build(func(b *annotation.Border) annotation.Annotation {
			return &annotation.Ink{Common: common(b), InkList: ink}
		}),
		"Line": build(func(b *annotation.Border) annotation.Annotation {
			return &annotation.Line{Common: common(b), Coords: [4]float64{10, 10, 190, 90}}
		}),
		"FreeText": build(func(b *annotation.Border) annotation.Annotation {
			return &annotation.FreeText{
				Common:            common(b),
				DefaultAppearance: "/Helv 12 Tf 0 g",
			}
		}),
	}
}

// strokeOps are the path-painting operators which stroke (§8.5.3.2).
var strokeOps = []string{"S", "s", "B", "B*", "b", "b*"}

// appearanceStream returns the content of the annotation's normal appearance,
// or nil if it has none.
func appearanceStream(t *testing.T, a annotation.Annotation) []byte {
	t.Helper()
	ap := annotation.Resolve(a.GetCommon(), appearance.Normal)
	if ap == nil || ap.Content == nil {
		return nil
	}
	r, err := ap.Content.RawBytes()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// countStrokes returns how many stroking operators the annotation's normal
// appearance uses.
func countStrokes(t *testing.T, a annotation.Annotation) int {
	t.Helper()
	n := 0
	for tok := range strings.FieldsSeq(string(appearanceStream(t, a))) {
		if slices.Contains(strokeOps, tok) {
			n++
		}
	}
	return n
}

// TestNoBorderDrawsNoStroke checks that an annotation asking for no border
// gets an appearance which strokes nothing, while the same annotation with a
// border of width 1 does stroke.  A nil [annotation.Common.Border] is what an
// explicit zero width in the file decodes to, and a zero width means no border
// is drawn.
func TestNoBorderDrawsNoStroke(t *testing.T) {
	for name, pair := range borderCases() {
		t.Run(name, func(t *testing.T) {
			noBorder, withBorder := pair[0], pair[1]

			if w := annotation.EffectiveBorderWidth(noBorder); w != 0 {
				t.Fatalf("border width %v, want 0: the case is set up wrongly", w)
			}
			if w := annotation.EffectiveBorderWidth(withBorder); w != 1 {
				t.Fatalf("border width %v, want 1: the case is set up wrongly", w)
			}

			s := newGen(t, pdf.V2_0)
			if err := s.AddAppearance(noBorder); err != nil {
				t.Fatal(err)
			}
			s2 := newGen(t, pdf.V2_0)
			if err := s2.AddAppearance(withBorder); err != nil {
				t.Fatal(err)
			}

			if n := countStrokes(t, noBorder); n != 0 {
				t.Errorf("an annotation with no border used %d stroking operators", n)
			}
			if n := countStrokes(t, withBorder); n == 0 {
				t.Error("an annotation with a border of width 1 stroked nothing")
			}
		})
	}
}

// TestCaretThinBox checks that a caret whose box is no taller than its
// border width still gets a well-formed appearance: the stroke width is
// clamped to the box, so no coordinate degenerates.
func TestCaretThinBox(t *testing.T) {
	for _, h := range []float64{0, 0.5, 1} {
		a := &annotation.Caret{Common: annotation.Common{
			Rect:   pdf.Rectangle{LLx: 10, LLy: 10, URx: 20, URy: 10 + h},
			Border: &annotation.Border{Width: 1},
			Color:  color.DeviceRGB{1, 0, 0},
		}}
		s := newGen(t, pdf.V2_0)
		if err := s.AddAppearance(a); err != nil {
			t.Fatal(err)
		}
		ap := annotation.Resolve(a.GetCommon(), appearance.Normal)
		if ap == nil || ap.Content == nil {
			t.Fatalf("height %g: no appearance", h)
		}
		r, err := ap.Content.RawBytes()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		for tok := range strings.FieldsSeq(string(data)) {
			if v, err := strconv.ParseFloat(tok, 64); err == nil && (math.IsNaN(v) || math.IsInf(v, 0)) {
				t.Errorf("height %g: content stream contains %q", h, tok)
			}
		}
	}
}

// TestBorderDashReachesAppearance checks that a border is drawn dashed exactly
// when the entry describing it asks for dashes, whichever of the two entries
// that is.  An annotation carries one or the other: a file may hold both, but
// the border array is dropped on reading whenever a border style is present.
func TestBorderDashReachesAppearance(t *testing.T) {
	rect := pdf.Rectangle{LLx: 10, LLy: 10, URx: 190, URy: 90}
	common := func() annotation.Common {
		return annotation.Common{Rect: rect, Color: color.DeviceRGB{1, 0, 0}}
	}
	withBorder := func(b *annotation.Border) *annotation.Square {
		c := common()
		c.Border = b
		return &annotation.Square{Common: c}
	}
	withStyle := func(s *annotation.BorderStyle) *annotation.Square {
		return &annotation.Square{Common: common(), BorderStyle: s}
	}

	cases := map[string]struct {
		a          annotation.Annotation
		wantDashed bool
	}{
		"border array with dashes": {
			withBorder(&annotation.Border{Width: 1, DashArray: []float64{4, 2}}), true,
		},
		"border array without dashes": {
			withBorder(&annotation.Border{Width: 1}), false,
		},
		"dashed border style": {
			withStyle(&annotation.BorderStyle{Width: 1, Style: "D", DashArray: []float64{4, 2}}), true,
		},
		"beveled border style": {
			withStyle(&annotation.BorderStyle{Width: 1, Style: "B"}), false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := newGen(t, pdf.V2_0)
			if err := s.AddAppearance(tc.a); err != nil {
				t.Fatal(err)
			}
			if got := usesDash(t, tc.a); got != tc.wantDashed {
				t.Errorf("appearance sets a dash pattern = %v, want %v", got, tc.wantDashed)
			}
		})
	}
}

// usesDash reports whether the annotation's normal appearance sets a non-empty
// dash pattern, i.e. whether it contains a "d" operator with a non-empty array.
func usesDash(t *testing.T, a annotation.Annotation) bool {
	t.Helper()
	// "[] 0 d" resets the dash pattern; anything else sets one
	for op := range strings.SplitSeq(string(appearanceStream(t, a)), "\n") {
		f := strings.Fields(op)
		if len(f) >= 3 && f[len(f)-1] == "d" && f[0] != "[]" {
			return true
		}
	}
	return false
}

// paints reports whether a content stream fills or strokes anything.
func paints(data []byte) bool {
	for tok := range strings.FieldsSeq(string(data)) {
		if tok == "f" || slices.Contains(strokeOps, tok) {
			return true
		}
	}
	return false
}

// TestTextMarkupIgnoresBorderWidth checks that the appearance of a text markup
// annotation does not depend on the border asked for.  The annotation
// dictionary has no entry which sets the width of an underline, strike-out or
// squiggly, so each is drawn at a fixed width, and Border, which describes the
// rectangle around an annotation, changes nothing.
func TestTextMarkupIgnoresBorderWidth(t *testing.T) {
	quads := []vec.Vec2{{X: 10, Y: 90}, {X: 190, Y: 90}, {X: 10, Y: 10}, {X: 190, Y: 10}}
	borders := []*annotation.Border{nil, {Width: 1}, {Width: 5}}
	types := []annotation.TextMarkupType{
		annotation.TextMarkupTypeHighlight,
		annotation.TextMarkupTypeUnderline,
		annotation.TextMarkupTypeStrikeOut,
		annotation.TextMarkupTypeSquiggly,
	}

	for _, markupType := range types {
		t.Run(string(markupType), func(t *testing.T) {
			var want []byte
			for _, border := range borders {
				a := &annotation.TextMarkup{
					Common: annotation.Common{
						Rect:   pdf.Rectangle{LLx: 10, LLy: 10, URx: 190, URy: 90},
						Border: border,
						Color:  color.DeviceRGB{1, 1, 0},
					},
					Type:       markupType,
					QuadPoints: quads,
				}
				s := newGen(t, pdf.V2_0)
				if err := s.AddAppearance(a); err != nil {
					t.Fatal(err)
				}

				got := appearanceStream(t, a)
				if !paints(got) {
					t.Fatalf("border %v: the markup drew nothing", border)
				}
				if want == nil {
					want = got
				} else if !bytes.Equal(got, want) {
					t.Errorf("border %v changed the appearance:\n%s\nwant:\n%s",
						border, got, want)
				}
			}
		})
	}
}
