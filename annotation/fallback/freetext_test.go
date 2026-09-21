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
	"slices"
	"strings"
	"testing"

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/internal/debug/memfile"
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

// TestFreeTextHonoursDAFontSize checks that the generator draws text at the
// size the default appearance string asks for, keeps that size in the
// rewritten DA, and leaves the annotation's alignment as set.
func TestFreeTextHonoursDAFontSize(t *testing.T) {
	a := &annotation.FreeText{
		Common: annotation.Common{
			Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 400, URy: 200},
			Contents: "one two three four five six seven eight nine ten " +
				"eleven twelve thirteen fourteen",
		},
		DefaultAppearance: "/Helv 18 Tf 0 g",
		Align:             pdf.TextAlignCenter,
	}

	g := newGen(t, pdf.V2_0)
	if err := g.AddAppearance(a); err != nil {
		t.Fatal(err)
	}

	body := appearanceContent(t, a)
	if !strings.Contains(body, "18 Tf") {
		t.Errorf("content stream does not set the font size to 18:\n%s", body)
	}
	if strings.Contains(body, "12 Tf") {
		t.Errorf("content stream still sets the fixed font size 12:\n%s", body)
	}
	if !strings.Contains(a.DefaultAppearance, "18 Tf") {
		t.Errorf("default appearance %q does not keep the font size 18", a.DefaultAppearance)
	}
	if a.Align != pdf.TextAlignCenter {
		t.Errorf("Align = %v, want it kept as TextAlignCenter", a.Align)
	}
}

// TestFreeTextDefaultFontSize checks that a DA which names no usable font
// size falls back to the default size 12: no size at all, a size of 0, and a
// size a PDF file cannot hold, which would leave the appearance unwritable.
func TestFreeTextDefaultFontSize(t *testing.T) {
	cases := []struct {
		name string
		da   string
	}{
		{"no size", "0 g"},
		{"zero", "/Helv 0 Tf 0 g"},
		{"infinite", "/Helv Inf Tf 0 g"},
		{"not a number", "/Helv NaN Tf 0 g"},
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

			body := appearanceContent(t, a)
			if !strings.Contains(body, "12 Tf") {
				t.Errorf("content stream does not fall back to font size 12:\n%s", body)
			}
			if !strings.Contains(a.DefaultAppearance, "12 Tf") {
				t.Errorf("default appearance %q does not fall back to font size 12", a.DefaultAppearance)
			}
		})
	}
}

// TestFreeTextTypewriterEncodesAtV2 checks that a Typewriter FreeText's
// appearance can be written to a PDF 2.0 file: the typewriter font must not
// carry a fixed /Name, since PDF 2.0 forbids that font dictionary entry (see
// annotation/fallback/style.go's Generator.typewriter), and a font instance
// asking for one fails the writer close instead of saving the document.
func TestFreeTextTypewriterEncodesAtV2(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	a := &annotation.FreeText{
		Common: annotation.Common{
			Rect:     pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 60},
			Contents: "hello",
		},
		Markup:            annotation.Markup{Intent: annotation.FreeTextIntentTypeWriter},
		DefaultAppearance: "/Cour 12 Tf 0 g",
	}

	g := newGen(t, pdf.V2_0)
	if err := g.AddAppearance(a); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Encode(rm); err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestFreeTextTypewriterFont checks that a Typewriter FreeText is drawn in
// its own font, distinct from the shared content font every other FreeText
// uses, and that a plain FreeText still uses the shared one.
func TestFreeTextTypewriterFont(t *testing.T) {
	g := newGen(t, pdf.V2_0)
	plain := &annotation.FreeText{Markup: annotation.Markup{Intent: annotation.FreeTextIntentPlain}}
	typewriter := &annotation.FreeText{Markup: annotation.Markup{Intent: annotation.FreeTextIntentTypeWriter}}

	if g.freeTextFont(plain) != g.ContentFont() {
		t.Error("a plain FreeText should be drawn in the shared content font")
	}
	if g.freeTextFont(typewriter) != g.typewriter() {
		t.Error("a Typewriter FreeText should be drawn in the typewriter font")
	}
	if g.freeTextFont(typewriter) == g.ContentFont() {
		t.Error("the typewriter font must not be the same instance as the content font")
	}
	if name := g.typewriter().PostScriptName(); name != "Courier" {
		t.Errorf("typewriter font PostScript name = %q, want %q", name, "Courier")
	}
}

// TestFreeTextKeepsItsBorderStyle checks that generating an appearance
// leaves the border the document gave.  The generator records the effective
// width where a later regeneration can read it, and must not turn a dashed
// or bevelled border into a plain one on the way: an edited box is written
// back to the file, so a style lost here is lost for good.
func TestFreeTextKeepsItsBorderStyle(t *testing.T) {
	a := &annotation.FreeText{
		Common:            annotation.Common{Rect: pdf.Rectangle{URx: 200, URy: 60}},
		DefaultAppearance: "/Helv 12 Tf 0 g",
		BorderStyle: &annotation.BorderStyle{
			Width: 2, Style: "D", DashArray: []float64{3, 2},
		},
	}

	g := newGen(t, pdf.V2_0)
	if err := g.AddAppearance(a); err != nil {
		t.Fatal(err)
	}

	if got := annotation.EffectiveBorderWidth(a); got != 2 {
		t.Errorf("border width = %v, want 2", got)
	}
	if got := annotation.EffectiveBorderStyle(a); got != "D" {
		t.Errorf("border style = %q, want D", got)
	}
	if got := annotation.EffectiveBorderDash(a); !slices.Equal(got, []float64{3, 2}) {
		t.Errorf("dash array = %v, want [3 2]", got)
	}
	// the two places a border can live are mutually exclusive: an annotation
	// carrying both cannot be written at all
	if a.BorderStyle != nil && a.Common.Border != nil {
		t.Error("the annotation carries both a border style and a border array")
	}
}

// TestFreeTextBorderlessStaysBorderless checks that a box the document drew
// without a border still has none after its appearance is generated, in
// either of the two places one can live.
func TestFreeTextBorderlessStaysBorderless(t *testing.T) {
	a := &annotation.FreeText{
		Common:            annotation.Common{Rect: pdf.Rectangle{URx: 200, URy: 60}},
		DefaultAppearance: "/Helv 12 Tf 0 g",
	}

	g := newGen(t, pdf.V2_0)
	if err := g.AddAppearance(a); err != nil {
		t.Fatal(err)
	}

	if got := annotation.EffectiveBorderWidth(a); got != 0 {
		t.Errorf("border width = %v, want 0", got)
	}
	if a.BorderStyle != nil || a.Common.Border != nil {
		t.Errorf("border = %v, style = %v, want neither", a.Common.Border, a.BorderStyle)
	}
}
