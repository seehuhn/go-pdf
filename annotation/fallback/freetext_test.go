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
	"io"
	"strings"
	"testing"

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
)

// appearanceContent returns the content stream of an annotation's normal
// appearance as a string.
func appearanceContent(t *testing.T, a annotation.Annotation) string {
	t.Helper()

	ap := annotation.Resolve(a.GetCommon(), appearance.Normal)
	if ap == nil || ap.Content == nil {
		t.Fatal("the annotation has no normal appearance")
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
	return string(data)
}

// TestFreeTextTextColor checks that the text of a FreeText annotation is
// drawn in the colour its default appearance string asks for, and that the
// generated default appearance names that colour again.
func TestFreeTextTextColor(t *testing.T) {
	cases := []struct {
		name string
		da   string
		want string
	}{
		{"rgb", "0 0 1 rg /Helv 12 Tf", "0 0 1 rg"},
		{"gray", "/Helv 12 Tf 0.5 g", "0.5 g"},
		{"cmyk", "/Helv 12 Tf 0 1 1 0 k", "0 1 1 0 k"},
		{"none", "/Helv 12 Tf", "0.102 0.094 0.078 rg"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := &annotation.FreeText{
				Common: annotation.Common{
					Rect:     pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 60},
					Contents: "hello",
				},
				DefaultAppearance: c.da,
			}

			g := newGen(t, pdf.V2_0)
			if err := g.AddAppearance(a); err != nil {
				t.Fatal(err)
			}

			if body := appearanceContent(t, a); !strings.Contains(body, c.want) {
				t.Errorf("appearance does not set the text colour %q:\n%s", c.want, body)
			}
			if !strings.Contains(a.DefaultAppearance, c.want) {
				t.Errorf("default appearance %q does not name the colour %q",
					a.DefaultAppearance, c.want)
			}
		})
	}
}

// TestFreeTextCalloutMargin checks that a callout reaching beyond the text
// box grows Rect and records the box as non-negative insets from the new
// Rect, in the order left, bottom, right, top, so that applying them gives
// the text box back.
func TestFreeTextCalloutMargin(t *testing.T) {
	box := pdf.Rectangle{LLx: 50, LLy: 50, URx: 200, URy: 110}
	a := &annotation.FreeText{
		Common: annotation.Common{
			Rect:     box,
			Contents: "hello",
		},
		Markup: annotation.Markup{Intent: annotation.FreeTextIntentCallout},
		// the line starts to the right of and above the box
		CalloutLine: []vec.Vec2{{X: 320, Y: 220}, {X: 200, Y: 110}},
	}

	g := newGen(t, pdf.V2_0)
	if err := g.AddAppearance(a); err != nil {
		t.Fatal(err)
	}

	if len(a.Margin) != 4 {
		t.Fatalf("got Margin %v, want four insets", a.Margin)
	}
	for i, m := range a.Margin {
		if m < 0 {
			t.Errorf("Margin[%d] = %g, want a non-negative inset", i, m)
		}
	}
	if a.Margin[2] <= 0 || a.Margin[3] <= 0 {
		t.Errorf("Margin = %v, want positive right and top insets for a callout beyond those edges", a.Margin)
	}
	inner := applyMargins(a.Rect, a.Margin)
	if !inner.NearlyEqual(&box, 0.01) {
		t.Errorf("applying Margin to Rect gives %v, want the text box %v", inner, box)
	}
}
